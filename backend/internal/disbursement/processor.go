package disbursement

import (
	"errors"
	"fmt"
	"slices"
)

// Processor owns worker availability and batch processing state.
type Processor struct {
	workers []Worker
}

var ErrInvalidProcessor = errors.New("invalid processor")

func NewProcessor(workers []Worker) (*Processor, error) {
	if len(workers) == 0 {
		return nil, fmt.Errorf("%w: at least one worker is required", ErrInvalidProcessor)
	}

	knownWorkers := make(map[WorkerID]struct{}, len(workers))
	for _, worker := range workers {
		if _, exists := knownWorkers[worker.ID()]; exists {
			return nil, fmt.Errorf("%w: duplicate worker ID %q", ErrInvalidProcessor, worker.ID())
		}
		knownWorkers[worker.ID()] = struct{}{}
	}

	return &Processor{workers: slices.Clone(workers)}, nil
}

func (p *Processor) AvailableWorkers() []Worker {
	return slices.Clone(p.workers)
}
