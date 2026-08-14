package disbursement

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"
)

// Processor owns worker availability and batch processing state.
type Processor struct {
	applicationContext context.Context
	provider           PaymentProvider
	providerTimeout    time.Duration
	logger             *slog.Logger

	mu          sync.RWMutex
	workerOrder []WorkerID
	obligations map[WorkerID]*paymentObligation
	batches     map[BatchID]*batch
	jobs        sync.WaitGroup
}

var (
	ErrInvalidProcessor    = errors.New("invalid processor")
	ErrInvalidSubmission   = errors.New("invalid submission")
	ErrDemoResetInProgress = errors.New("cannot reset demo while a batch is processing")
)

type ProcessorConfig struct {
	Provider        PaymentProvider
	ProviderTimeout time.Duration
	Logger          *slog.Logger
}

// paymentObligationStatus prevents the same seeded obligation from being paid twice.
type paymentObligationStatus uint8

const (
	obligationAvailable paymentObligationStatus = iota
	obligationReserved
	obligationPaid
	obligationOutcomeUnknown
)

type paymentObligation struct {
	worker         Worker
	status         paymentObligationStatus
	batchID        BatchID
	disbursementID DisbursementID
}

type batch struct {
	id                 BatchID
	canonicalWorkerIDs []WorkerID
	resultOrder        []WorkerID
	results            map[WorkerID]*DisbursementResult
	pendingCount       int
}

func NewProcessor(
	applicationContext context.Context,
	workers []Worker,
	config ProcessorConfig,
) (*Processor, error) {
	if err := validateProcessorConfiguration(applicationContext, workers, config); err != nil {
		return nil, err
	}

	logger := config.Logger
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}

	workerOrder, obligations, err := indexPaymentObligations(workers)
	if err != nil {
		return nil, err
	}

	return &Processor{
		applicationContext: applicationContext,
		provider:           config.Provider,
		providerTimeout:    config.ProviderTimeout,
		logger:             logger,
		workerOrder:        workerOrder,
		obligations:        obligations,
		batches:            make(map[BatchID]*batch),
	}, nil
}

func (p *Processor) AvailableWorkers() []Worker {
	p.mu.RLock()
	defer p.mu.RUnlock()

	workers := make([]Worker, 0, len(p.obligations))
	for _, workerID := range p.workerOrder {
		obligation := p.obligations[workerID]
		if obligation.status == obligationAvailable {
			workers = append(workers, obligation.worker)
		}
	}

	return workers
}

func (p *Processor) Submit(batchID BatchID, workerIDs []WorkerID) (Submission, error) {
	canonicalWorkerIDs, err := canonicalWorkerSet(batchID, workerIDs)
	if err != nil {
		return Submission{}, err
	}

	p.mu.Lock()
	if existingBatch, exists := p.batches[batchID]; exists {
		isExactReplay := slices.Equal(existingBatch.canonicalWorkerIDs, canonicalWorkerIDs)
		p.mu.Unlock()
		if isExactReplay {
			p.logger.Info("disbursement batch replayed", "batch_id", batchID)
			return Submission{BatchID: batchID, Created: false}, nil
		}
		p.logger.Warn("disbursement batch ID conflict", "batch_id", batchID)
		return Submission{}, &IdempotencyConflictError{BatchID: batchID}
	}

	for _, workerID := range workerIDs {
		if _, exists := p.obligations[workerID]; !exists {
			p.mu.Unlock()
			return Submission{}, fmt.Errorf("%w: unknown worker ID %q", ErrInvalidSubmission, workerID)
		}
	}

	unavailableWorkers := p.unavailableWorkersLocked(workerIDs)
	if len(unavailableWorkers) > 0 {
		p.mu.Unlock()
		p.logger.Warn(
			"disbursement batch rejected",
			"batch_id", batchID,
			"unavailable_worker_count", len(unavailableWorkers),
		)
		return Submission{}, &WorkersUnavailableError{Workers: unavailableWorkers}
	}

	results := make(map[WorkerID]*DisbursementResult, len(workerIDs))
	requests := make([]PaymentRequest, 0, len(workerIDs))
	for _, workerID := range workerIDs {
		currentObligation := p.obligations[workerID]
		disbursementID := DisbursementID(randomID("disb"))
		result := &DisbursementResult{
			DisbursementID: disbursementID,
			Worker:         currentObligation.worker,
			Status:         StatusPending,
		}
		results[workerID] = result
		requests = append(requests, PaymentRequest{
			DisbursementID: disbursementID,
			WorkerID:       workerID,
			Amount:         currentObligation.worker.Amount(),
		})

		currentObligation.status = obligationReserved
		currentObligation.batchID = batchID
		currentObligation.disbursementID = disbursementID
	}

	p.batches[batchID] = &batch{
		id:                 batchID,
		canonicalWorkerIDs: canonicalWorkerIDs,
		resultOrder:        slices.Clone(workerIDs),
		results:            results,
		pendingCount:       len(workerIDs),
	}
	p.jobs.Add(len(requests))
	p.mu.Unlock()
	p.logger.Info(
		"disbursement batch accepted",
		"batch_id", batchID,
		"worker_count", len(workerIDs),
	)

	for _, request := range requests {
		go p.processPayment(batchID, request)
	}

	return Submission{BatchID: batchID, Created: true}, nil
}

func (p *Processor) Batch(batchID BatchID) (BatchSnapshot, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	storedBatch, exists := p.batches[batchID]
	if !exists {
		return BatchSnapshot{}, false
	}

	status := BatchProcessing
	if storedBatch.pendingCount == 0 {
		status = BatchCompleted
	}
	results := make([]DisbursementResult, 0, len(storedBatch.resultOrder))
	for _, workerID := range storedBatch.resultOrder {
		results = append(results, *storedBatch.results[workerID])
	}

	return BatchSnapshot{
		BatchID: storedBatch.id,
		Status:  status,
		Results: results,
	}, true
}

func (p *Processor) Wait() {
	p.jobs.Wait()
}

// ResetDemo restores every seeded obligation and removes process-local batch history.
// It is intentionally unavailable while provider calls are still in progress.
func (p *Processor) ResetDemo() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, storedBatch := range p.batches {
		if storedBatch.pendingCount > 0 {
			return ErrDemoResetInProgress
		}
	}

	for _, currentObligation := range p.obligations {
		currentObligation.status = obligationAvailable
		currentObligation.batchID = ""
		currentObligation.disbursementID = ""
	}
	p.batches = make(map[BatchID]*batch)
	p.logger.Info("demo state reset", "worker_count", len(p.obligations))

	return nil
}

func (p *Processor) unavailableWorkersLocked(workerIDs []WorkerID) []UnavailableWorker {
	var unavailableWorkers []UnavailableWorker
	for _, workerID := range workerIDs {
		currentObligation := p.obligations[workerID]

		var reason UnavailableReason
		switch currentObligation.status {
		case obligationReserved:
			reason = AlreadyPending
		case obligationPaid:
			reason = AlreadyPaid
		case obligationOutcomeUnknown:
			reason = OutcomeUnknown
		default:
			continue
		}

		unavailableWorkers = append(unavailableWorkers, UnavailableWorker{
			Worker:         currentObligation.worker,
			Reason:         reason,
			BatchID:        currentObligation.batchID,
			DisbursementID: currentObligation.disbursementID,
		})
	}

	return unavailableWorkers
}

func (p *Processor) processPayment(batchID BatchID, request PaymentRequest) {
	defer p.jobs.Done()

	ctx, cancel := context.WithTimeout(p.applicationContext, p.providerTimeout)
	defer cancel()

	paymentResult, err := p.provider.Pay(ctx, request)

	p.mu.Lock()

	storedBatch := p.batches[batchID]
	result := storedBatch.results[request.WorkerID]
	currentObligation := p.obligations[request.WorkerID]
	if err == nil && paymentResult.ProviderTransactionID != "" {
		result.Status = StatusSuccess
		result.ProviderTransactionID = paymentResult.ProviderTransactionID
		currentObligation.status = obligationPaid
	} else {
		p.recordFailureLocked(result, currentObligation, err)
	}
	storedBatch.pendingCount--
	completed := storedBatch.pendingCount == 0
	resultSnapshot := *result
	p.mu.Unlock()

	logLevel := slog.LevelInfo
	if resultSnapshot.Status == StatusFailed || resultSnapshot.Status == StatusOutcomeUnknown {
		logLevel = slog.LevelWarn
	}
	p.logger.Log(
		p.applicationContext,
		logLevel,
		"disbursement result recorded",
		"batch_id", batchID,
		"disbursement_id", resultSnapshot.DisbursementID,
		"worker_id", resultSnapshot.Worker.ID(),
		"status", resultSnapshot.Status,
		"provider_txn_id", resultSnapshot.ProviderTransactionID,
		"error_code", resultSnapshot.ErrorCode,
	)
	if completed {
		p.logger.Info("disbursement batch completed", "batch_id", batchID)
	}
}

func (p *Processor) recordFailureLocked(
	result *DisbursementResult,
	currentObligation *paymentObligation,
	err error,
) {
	var providerFailure *ProviderFailure
	switch {
	case errors.As(err, &providerFailure):
		result.ErrorCode = providerFailure.Code
		result.ErrorMessage = providerFailure.Message
		if providerFailure.OutcomeUnknown || providerFailure.Code == ProviderTimeout {
			result.Status = StatusOutcomeUnknown
			currentObligation.status = obligationOutcomeUnknown
			return
		}
		result.Status = StatusFailed
		currentObligation.status = obligationAvailable
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		result.Status = StatusOutcomeUnknown
		result.ErrorCode = ProviderTimeout
		result.ErrorMessage = "the provider did not confirm whether the payment completed"
		currentObligation.status = obligationOutcomeUnknown
	default:
		result.Status = StatusFailed
		result.ErrorCode = ProviderError
		result.ErrorMessage = "the provider could not process the payment"
		currentObligation.status = obligationAvailable
	}
}

func validateProcessorConfiguration(
	applicationContext context.Context,
	workers []Worker,
	config ProcessorConfig,
) error {
	if applicationContext == nil {
		return fmt.Errorf("%w: application context is required", ErrInvalidProcessor)
	}
	if len(workers) == 0 {
		return fmt.Errorf("%w: at least one worker is required", ErrInvalidProcessor)
	}
	if config.Provider == nil {
		return fmt.Errorf("%w: payment provider is required", ErrInvalidProcessor)
	}
	if config.ProviderTimeout <= 0 {
		return fmt.Errorf("%w: provider timeout must be positive", ErrInvalidProcessor)
	}
	return nil
}

func indexPaymentObligations(workers []Worker) ([]WorkerID, map[WorkerID]*paymentObligation, error) {
	workerOrder := make([]WorkerID, 0, len(workers))
	obligations := make(map[WorkerID]*paymentObligation, len(workers))
	for _, worker := range workers {
		if _, exists := obligations[worker.ID()]; exists {
			return nil, nil, fmt.Errorf("%w: duplicate worker ID %q", ErrInvalidProcessor, worker.ID())
		}
		workerOrder = append(workerOrder, worker.ID())
		obligations[worker.ID()] = &paymentObligation{
			worker: worker,
			status: obligationAvailable,
		}
	}
	return workerOrder, obligations, nil
}

func canonicalWorkerSet(batchID BatchID, workerIDs []WorkerID) ([]WorkerID, error) {
	if strings.TrimSpace(string(batchID)) == "" {
		return nil, fmt.Errorf("%w: batch ID is required", ErrInvalidSubmission)
	}
	if len(workerIDs) == 0 {
		return nil, fmt.Errorf("%w: at least one worker ID is required", ErrInvalidSubmission)
	}

	canonicalWorkerIDs := slices.Clone(workerIDs)
	slices.Sort(canonicalWorkerIDs)
	for index, workerID := range canonicalWorkerIDs {
		if strings.TrimSpace(string(workerID)) == "" {
			return nil, fmt.Errorf("%w: worker ID is required", ErrInvalidSubmission)
		}
		if index > 0 && canonicalWorkerIDs[index-1] == workerID {
			return nil, fmt.Errorf("%w: duplicate worker ID %q", ErrInvalidSubmission, workerID)
		}
	}

	return canonicalWorkerIDs, nil
}

func randomID(prefix string) string {
	return prefix + "-" + strings.ToLower(rand.Text())
}
