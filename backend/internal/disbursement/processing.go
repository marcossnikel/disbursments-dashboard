package disbursement

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

func (p *Processor) processPayment(
	ctx context.Context,
	batchID BatchID,
	request PaymentRequest,
) {
	var paymentResult PaymentResult
	var err error
	for attempt := 1; attempt <= p.providerMaxAttempts; attempt++ {
		if attempt > 1 {
			p.recordPaymentAttempt(batchID, request.WorkerID, attempt)
		}

		paymentContext, cancel := context.WithTimeout(ctx, p.providerTimeout)
		paymentResult, err = p.provider.Pay(paymentContext, request)
		cancel()

		if paymentSucceeded(paymentResult, err) ||
			!p.shouldRetryPayment(ctx, err, attempt) {
			break
		}

		p.logger.WarnContext(
			ctx,
			"disbursement automatic retry scheduled",
			"batch_id", batchID,
			"disbursement_id", request.DisbursementID,
			"worker_id", request.WorkerID,
			"attempt", attempt,
			"max_attempts", p.providerMaxAttempts,
		)
		if waitErr := waitForRetry(ctx, p.providerRetryDelay); waitErr != nil {
			err = waitErr
			break
		}
	}

	result, batchCompleted := p.recordPaymentResult(batchID, request, paymentResult, err)

	logLevel := slog.LevelInfo
	if result.Status == StatusFailed {
		logLevel = slog.LevelWarn
	}
	p.logger.Log(
		ctx,
		logLevel,
		"disbursement result recorded",
		"batch_id", batchID,
		"disbursement_id", result.DisbursementID,
		"worker_id", result.Worker.ID(),
		"status", result.Status,
		"attempts", result.Attempts,
		"provider_transaction_id", result.ProviderTransactionID,
		"error_code", result.ErrorCode,
	)
	if batchCompleted {
		p.logger.InfoContext(ctx, "disbursement batch completed", "batch_id", batchID)
	}
}

func (p *Processor) shouldRetryPayment(
	ctx context.Context,
	err error,
	attempt int,
) bool {
	if ctx.Err() != nil || attempt >= p.providerMaxAttempts {
		return false
	}
	errorCode, _ := classifyProviderError(err)
	return errorCode != ProviderDeclined
}

func paymentSucceeded(result PaymentResult, err error) bool {
	return err == nil && result.ProviderTransactionID != ""
}

func waitForRetry(ctx context.Context, delay time.Duration) error {
	if delay == 0 {
		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *Processor) recordPaymentAttempt(
	batchID BatchID,
	workerID WorkerID,
	attempt int,
) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.batches[batchID].results[workerID].Attempts = attempt
}

func (p *Processor) recordPaymentResult(
	batchID BatchID,
	request PaymentRequest,
	paymentResult PaymentResult,
	err error,
) (DisbursementResult, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	storedBatch := p.batches[batchID]
	result := storedBatch.results[request.WorkerID]
	currentObligation := p.obligations[request.WorkerID]
	if paymentSucceeded(paymentResult, err) {
		result.Status = StatusSuccess
		result.ProviderTransactionID = paymentResult.ProviderTransactionID
		currentObligation.status = obligationPaid
	} else {
		result.Status = StatusFailed
		result.ErrorCode, result.ErrorMessage = classifyProviderError(err)
		currentObligation.status = obligationAvailable
	}

	storedBatch.pendingCount--
	return *result, storedBatch.pendingCount == 0
}

func classifyProviderError(err error) (ProviderErrorCode, string) {
	var providerFailure *ProviderFailure
	switch {
	case errors.As(err, &providerFailure):
		return providerFailure.Code, providerFailure.Message
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return ProviderTimeout, "the provider request timed out"
	default:
		return ProviderError, "the provider could not process the payment"
	}
}
