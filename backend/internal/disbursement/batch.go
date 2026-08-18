package disbursement

// BatchID uniquely identifies one idempotent submission.
type BatchID string

// BatchStatus describes whether a batch still has nonterminal payments.
type BatchStatus string

const (
	// BatchProcessing means at least one payment is pending or in flight.
	BatchProcessing BatchStatus = "processing"
	// BatchCompleted means every payment reached a terminal state.
	BatchCompleted BatchStatus = "completed"
)

// DisbursementStatus describes the state of one payment attempt.
type DisbursementStatus string

const (
	// StatusPending means the provider call has not started.
	StatusPending DisbursementStatus = "pending"
	// StatusInFlight means the provider call has started but has not completed.
	StatusInFlight DisbursementStatus = "in_flight"
	// StatusSuccess means the provider returned a transaction ID.
	StatusSuccess DisbursementStatus = "success"
	// StatusFailed means the provider call returned a terminal failure.
	StatusFailed DisbursementStatus = "failed"
	// StatusCanceled means the provider was never called for this payment.
	StatusCanceled DisbursementStatus = "canceled"
)

// DisbursementResult is the current outcome of one worker payment.
type DisbursementResult struct {
	DisbursementID        DisbursementID
	Worker                Worker
	Status                DisbursementStatus
	ProviderTransactionID ProviderTransactionID
	ErrorCode             ProviderErrorCode
	ErrorMessage          string
}

// BatchSnapshot is a point-in-time copy safe for callers to read.
type BatchSnapshot struct {
	BatchID BatchID
	Status  BatchStatus
	Results []DisbursementResult
}

type storedBatch struct {
	id                 BatchID
	canonicalWorkerIDs []WorkerID
	resultOrder        []WorkerID
	results            map[WorkerID]*DisbursementResult
	activeCount        int
	cancelPending      chan struct{}
	cancelRequested    bool
}

// Batch returns a snapshot of a submitted batch.
func (p *Processor) Batch(batchID BatchID) (BatchSnapshot, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	storedBatch, exists := p.batches[batchID]
	if !exists {
		return BatchSnapshot{}, false
	}

	return snapshotBatchLocked(storedBatch), true
}

func snapshotBatchLocked(storedBatch *storedBatch) BatchSnapshot {
	status := BatchProcessing
	if storedBatch.activeCount == 0 {
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
	}
}
