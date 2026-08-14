package disbursement

import (
	"errors"
	"fmt"
	"strings"
)

// WorkerID uniquely identifies a payment recipient in the demo dataset.
type WorkerID string

// Worker identifies a recipient and the current payment obligation shown in the dashboard.
type Worker struct {
	id     WorkerID
	name   string
	amount Money
}

// ErrInvalidWorker reports an invalid worker identity or payment amount.
var ErrInvalidWorker = errors.New("invalid worker")

// NewWorker validates and creates a worker payment obligation.
func NewWorker(id WorkerID, name string, amount Money) (Worker, error) {
	if strings.TrimSpace(string(id)) == "" {
		return Worker{}, fmt.Errorf("%w: ID is required", ErrInvalidWorker)
	}
	if strings.TrimSpace(name) == "" {
		return Worker{}, fmt.Errorf("%w: name is required", ErrInvalidWorker)
	}
	if amount.MinorUnits() <= 0 {
		return Worker{}, fmt.Errorf("%w: amount must be positive", ErrInvalidWorker)
	}

	return Worker{id: id, name: name, amount: amount}, nil
}

// ID returns the worker identifier.
func (w Worker) ID() WorkerID {
	return w.id
}

// Name returns the worker display name.
func (w Worker) Name() string {
	return w.name
}

// Amount returns the exact pending payment amount.
func (w Worker) Amount() Money {
	return w.amount
}
