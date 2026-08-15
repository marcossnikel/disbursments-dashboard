package disbursement

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

type preparedSubmission struct {
	requests []PaymentRequest
	created  bool
}

// Submission identifies a batch and whether this call created its provider work.
type Submission struct {
	BatchID BatchID
	Created bool
}

// UnavailableReason explains why an obligation cannot join a new batch.
type UnavailableReason string

const (
	// AlreadyPending means another batch currently owns the obligation.
	AlreadyPending UnavailableReason = "already_pending"
	// AlreadyPaid means an earlier batch completed the obligation successfully.
	AlreadyPaid UnavailableReason = "already_paid"
)

// UnavailableWorker carries the details needed to explain a rejected batch.
type UnavailableWorker struct {
	Worker         Worker
	Reason         UnavailableReason
	BatchID        BatchID
	DisbursementID DisbursementID
}

// IdempotencyConflictError reports reuse of a batch ID with a different worker set.
type IdempotencyConflictError struct {
	BatchID BatchID
}

func (e *IdempotencyConflictError) Error() string {
	return fmt.Sprintf("batch ID %q was already used with a different worker set", e.BatchID)
}

// WorkersUnavailableError reports obligations already reserved or paid.
type WorkersUnavailableError struct {
	Workers []UnavailableWorker
}

func (e *WorkersUnavailableError) Error() string {
	workerIDs := make([]string, 0, len(e.Workers))
	for _, unavailableWorker := range e.Workers {
		workerIDs = append(workerIDs, string(unavailableWorker.Worker.ID()))
	}
	return fmt.Sprintf("workers are unavailable: %s", strings.Join(workerIDs, ", "))
}

// Submit atomically validates and reserves every requested obligation before
// starting concurrent provider calls. An exact batch replay returns Created=false
// and never creates duplicate provider work. Results complete asynchronously.
func (p *Processor) Submit(
	ctx context.Context,
	batchID BatchID,
	workerIDs []WorkerID,
) (Submission, error) {
	if ctx == nil {
		return Submission{}, fmt.Errorf("%w: context is required", ErrInvalidSubmission)
	}
	canonicalWorkerIDs, err := canonicalWorkerSet(batchID, workerIDs)
	if err != nil {
		return Submission{}, err
	}

	prepared, err := p.prepareSubmission(batchID, workerIDs, canonicalWorkerIDs)
	if err != nil {
		p.logRejectedSubmission(ctx, batchID, err)
		return Submission{}, err
	}
	if !prepared.created {
		p.logger.InfoContext(ctx, "disbursement batch replayed", "batch_id", batchID)
		return Submission{BatchID: batchID, Created: false}, nil
	}

	p.logger.InfoContext(
		ctx,
		"disbursement batch accepted",
		"batch_id", batchID,
		"worker_count", len(workerIDs),
	)
	for _, request := range prepared.requests {
		p.jobs.Go(func() {
			p.processPayment(ctx, batchID, request)
		})
	}

	return Submission{BatchID: batchID, Created: true}, nil
}

// prepareSubmission performs the idempotency check, validates every worker,
// reserves every obligation, and stores the pending batch as one atomic operation.
// Provider goroutines are started only after this method releases the mutex.
func (p *Processor) prepareSubmission(
	batchID BatchID,
	workerIDs []WorkerID,
	canonicalWorkerIDs []WorkerID,
) (preparedSubmission, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if existingBatch, exists := p.batches[batchID]; exists {
		if slices.Equal(existingBatch.canonicalWorkerIDs, canonicalWorkerIDs) {
			return preparedSubmission{created: false}, nil
		}
		return preparedSubmission{}, &IdempotencyConflictError{BatchID: batchID}
	}

	for _, workerID := range workerIDs {
		if _, exists := p.obligations[workerID]; !exists {
			return preparedSubmission{}, fmt.Errorf(
				"%w: unknown worker ID %q",
				ErrInvalidSubmission,
				workerID,
			)
		}
	}

	unavailableWorkers := p.unavailableWorkersLocked(workerIDs)
	if len(unavailableWorkers) > 0 {
		return preparedSubmission{}, &WorkersUnavailableError{Workers: unavailableWorkers}
	}

	results := make(map[WorkerID]*DisbursementResult, len(workerIDs))
	requests := make([]PaymentRequest, 0, len(workerIDs))
	for _, workerID := range workerIDs {
		currentObligation := p.obligations[workerID]
		disbursementID := DisbursementID(randomID("disb"))
		results[workerID] = &DisbursementResult{
			DisbursementID: disbursementID,
			Worker:         currentObligation.worker,
			Status:         StatusPending,
			Attempts:       1,
		}
		requests = append(requests, PaymentRequest{
			DisbursementID: disbursementID,
			WorkerID:       workerID,
			Amount:         currentObligation.worker.Amount(),
		})

		currentObligation.status = obligationReserved
		currentObligation.batchID = batchID
		currentObligation.disbursementID = disbursementID
	}

	p.batches[batchID] = &storedBatch{
		id:                 batchID,
		createdAt:          time.Now().UTC(),
		canonicalWorkerIDs: canonicalWorkerIDs,
		resultOrder:        slices.Clone(workerIDs),
		results:            results,
		pendingCount:       len(workerIDs),
	}
	p.batchOrder = append(p.batchOrder, batchID)
	return preparedSubmission{requests: requests, created: true}, nil
}

func (p *Processor) logRejectedSubmission(ctx context.Context, batchID BatchID, err error) {
	var idempotencyConflict *IdempotencyConflictError
	var workersUnavailable *WorkersUnavailableError
	switch {
	case errors.As(err, &idempotencyConflict):
		p.logger.WarnContext(ctx, "disbursement batch ID conflict", "batch_id", batchID)
	case errors.As(err, &workersUnavailable):
		p.logger.WarnContext(
			ctx,
			"disbursement batch rejected",
			"batch_id", batchID,
			"unavailable_worker_count", len(workersUnavailable.Workers),
		)
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
