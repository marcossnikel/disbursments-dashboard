package disbursement

// BatchID uniquely identifies one idempotent submission.
type BatchID string

// BatchStatus describes whether a batch still has pending provider calls.
type BatchStatus string

const (
	// BatchProcessing means at least one payment is still pending.
	BatchProcessing BatchStatus = "processing"
	// BatchCompleted means every payment reached a terminal state.
	BatchCompleted BatchStatus = "completed"
)

// DisbursementStatus describes the state of one payment attempt.
type DisbursementStatus string

const (
	// StatusPending means the provider call has not completed.
	StatusPending DisbursementStatus = "pending"
	// StatusSuccess means the provider returned a transaction ID.
	StatusSuccess DisbursementStatus = "success"
	// StatusFailed means the provider call returned a terminal failure.
	StatusFailed DisbursementStatus = "failed"
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
	pendingCount       int
}

// Batch returns a snapshot of a submitted batch.
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
