package disbursement

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
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

	mu          sync.RWMutex
	workerOrder []WorkerID
	obligations map[WorkerID]*obligation
	batches     map[BatchID]*batch
	jobs        sync.WaitGroup
}

var ErrInvalidProcessor = errors.New("invalid processor")
var ErrInvalidSubmission = errors.New("invalid submission")

type ProcessorConfig struct {
	Provider        PaymentProvider
	ProviderTimeout time.Duration
}

type obligationStatus uint8

const (
	obligationAvailable obligationStatus = iota
	obligationReserved
	obligationPaid
	obligationBlocked
)

type obligation struct {
	worker         Worker
	status         obligationStatus
	batchID        BatchID
	disbursementID DisbursementID
}

type batch struct {
	id                 BatchID
	canonicalWorkerIDs []WorkerID
	resultOrder        []WorkerID
	results            map[WorkerID]*Result
	pendingCount       int
}

func NewProcessor(
	applicationContext context.Context,
	workers []Worker,
	config ProcessorConfig,
) (*Processor, error) {
	if applicationContext == nil {
		return nil, fmt.Errorf("%w: application context is required", ErrInvalidProcessor)
	}
	if len(workers) == 0 {
		return nil, fmt.Errorf("%w: at least one worker is required", ErrInvalidProcessor)
	}
	if config.Provider == nil {
		return nil, fmt.Errorf("%w: payment provider is required", ErrInvalidProcessor)
	}
	if config.ProviderTimeout <= 0 {
		return nil, fmt.Errorf("%w: provider timeout must be positive", ErrInvalidProcessor)
	}

	workerOrder := make([]WorkerID, 0, len(workers))
	obligations := make(map[WorkerID]*obligation, len(workers))
	for _, worker := range workers {
		if _, exists := obligations[worker.ID()]; exists {
			return nil, fmt.Errorf("%w: duplicate worker ID %q", ErrInvalidProcessor, worker.ID())
		}
		workerOrder = append(workerOrder, worker.ID())
		obligations[worker.ID()] = &obligation{
			worker: worker,
			status: obligationAvailable,
		}
	}

	return &Processor{
		applicationContext: applicationContext,
		provider:           config.Provider,
		providerTimeout:    config.ProviderTimeout,
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
		p.mu.Unlock()
		if slices.Equal(existingBatch.canonicalWorkerIDs, canonicalWorkerIDs) {
			return Submission{BatchID: batchID, Created: false}, nil
		}
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
		return Submission{}, &WorkersUnavailableError{Workers: unavailableWorkers}
	}

	results := make(map[WorkerID]*Result, len(workerIDs))
	requests := make([]PaymentRequest, 0, len(workerIDs))
	for _, workerID := range workerIDs {
		currentObligation := p.obligations[workerID]
		disbursementID := DisbursementID(randomID("disb"))
		result := &Result{
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
	results := make([]Result, 0, len(storedBatch.resultOrder))
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
		case obligationBlocked:
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
	defer p.mu.Unlock()

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
}

func (p *Processor) recordFailureLocked(result *Result, currentObligation *obligation, err error) {
	var providerFailure *ProviderFailure
	switch {
	case errors.As(err, &providerFailure):
		result.ErrorCode = providerFailure.Code
		result.ErrorMessage = providerFailure.Message
		if providerFailure.OutcomeUnknown || providerFailure.Code == ProviderTimeout {
			result.Status = StatusOutcomeUnknown
			currentObligation.status = obligationBlocked
			return
		}
		result.Status = StatusFailed
		currentObligation.status = obligationAvailable
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		result.Status = StatusOutcomeUnknown
		result.ErrorCode = ProviderTimeout
		result.ErrorMessage = "the provider did not confirm whether the payment completed"
		currentObligation.status = obligationBlocked
	default:
		result.Status = StatusFailed
		result.ErrorCode = ProviderError
		result.ErrorMessage = "the provider could not process the payment"
		currentObligation.status = obligationAvailable
	}
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
