package disbursement

import (
	"context"
	"errors"
	"log/slog"
)

func (p *Processor) processPayment(
	ctx context.Context,
	batchID BatchID,
	request PaymentRequest,
	cancelPending <-chan struct{},
) {
	select {
	case p.providerSlots <- struct{}{}:
		defer func() { <-p.providerSlots }()
	case <-cancelPending:
		return
	}

	if !p.startProviderCall(batchID, request.WorkerID) {
		return
	}

	paymentContext, cancel := context.WithTimeout(ctx, p.providerTimeout)
	defer cancel()

	paymentResult, err := p.provider.Pay(paymentContext, request)
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
		"provider_transaction_id", result.ProviderTransactionID,
		"error_code", result.ErrorCode,
	)
	if batchCompleted {
		p.logger.InfoContext(ctx, "disbursement batch completed", "batch_id", batchID)
	}
}

func (p *Processor) startProviderCall(batchID BatchID, workerID WorkerID) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	storedBatch, exists := p.batches[batchID]
	if !exists {
		return false
	}
	result := storedBatch.results[workerID]
	if result.Status != StatusPending {
		return false
	}
	result.Status = StatusInFlight
	return true
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
	if err == nil && paymentResult.ProviderTransactionID != "" {
		result.Status = StatusSuccess
		result.ProviderTransactionID = paymentResult.ProviderTransactionID
		currentObligation.status = obligationPaid
	} else {
		result.Status = StatusFailed
		result.ErrorCode, result.ErrorMessage = classifyProviderError(err)
		currentObligation.status = obligationAvailable
	}

	storedBatch.activeCount--
	return *result, storedBatch.activeCount == 0
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
