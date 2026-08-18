package disbursement

// ResetDemo restores every seeded obligation and removes process-local batch history.
// It is a reviewer convenience, not a production payment operation, and is rejected
// while any provider call is still in progress.
func (p *Processor) ResetDemo() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, storedBatch := range p.batches {
		if storedBatch.activeCount > 0 {
			return ErrDemoResetInProgress
		}
	}

	for _, currentObligation := range p.obligations {
		currentObligation.status = obligationAvailable
		currentObligation.batchID = ""
		currentObligation.disbursementID = ""
	}
	p.batches = make(map[BatchID]*storedBatch)
	p.logger.Info("demo state reset", "worker_count", len(p.obligations))

	return nil
}
