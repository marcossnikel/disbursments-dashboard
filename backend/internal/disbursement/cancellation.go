package disbursement

import (
	"context"
	"fmt"
)

// Cancellation is the authoritative result of one idempotent batch cancellation request.
type Cancellation struct {
	Batch         BatchSnapshot
	CanceledCount int
}

// CancelBatch cancels only payments whose provider call has not started.
// In-flight and terminal payments are left unchanged.
func (p *Processor) CancelBatch(ctx context.Context, batchID BatchID) (Cancellation, error) {
	p.mu.Lock()
	storedBatch, exists := p.batches[batchID]
	if !exists {
		p.mu.Unlock()
		return Cancellation{}, fmt.Errorf("%w: %q", ErrBatchNotFound, batchID)
	}

	canceledCount := 0
	for _, workerID := range storedBatch.resultOrder {
		result := storedBatch.results[workerID]
		if result.Status != StatusPending {
			continue
		}

		result.Status = StatusCanceled
		p.obligations[workerID].status = obligationAvailable
		storedBatch.activeCount--
		canceledCount++
	}
	if !storedBatch.cancelRequested {
		close(storedBatch.cancelPending)
		storedBatch.cancelRequested = true
	}
	cancellation := Cancellation{
		Batch:         snapshotBatchLocked(storedBatch),
		CanceledCount: canceledCount,
	}
	p.mu.Unlock()

	p.logger.InfoContext(
		ctx,
		"disbursement batch cancellation applied",
		"batch_id", batchID,
		"canceled_count", canceledCount,
		"batch_status", cancellation.Batch.Status,
	)
	return cancellation, nil
}
