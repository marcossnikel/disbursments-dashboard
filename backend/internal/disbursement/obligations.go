package disbursement

import "fmt"

// paymentObligationStatus prevents the same seeded obligation from being paid twice.
type paymentObligationStatus uint8

const (
	obligationAvailable paymentObligationStatus = iota
	obligationReserved
	obligationPaid
)

type paymentObligation struct {
	worker         Worker
	status         paymentObligationStatus
	batchID        BatchID
	disbursementID DisbursementID
}

// AvailableWorkers returns obligations that are neither reserved nor paid.
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
