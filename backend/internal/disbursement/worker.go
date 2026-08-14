package disbursement

import (
	"errors"
	"fmt"
	"strings"
)

type WorkerID string

// Worker identifies a recipient and the current payment obligation shown in the dashboard.
type Worker struct {
	id     WorkerID
	name   string
	amount Money
}

var ErrInvalidWorker = errors.New("invalid worker")

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

func (w Worker) ID() WorkerID {
	return w.id
}

func (w Worker) Name() string {
	return w.name
}

func (w Worker) Amount() Money {
	return w.amount
}

func SeedWorkers() ([]Worker, error) {
	seeds := []struct {
		id       WorkerID
		name     string
		amount   string
		currency Currency
	}{
		{id: "wrk_001", name: "Maya Thompson", amount: "1500.50", currency: USD},
		{id: "wrk_002", name: "Daniel Kim", amount: "2300.00", currency: EUR},
		{id: "wrk_003", name: "Sofia Martinez", amount: "1875.25", currency: USD},
		{id: "wrk_004", name: "Lucas Ferreira", amount: "2140.80", currency: EUR},
		{id: "wrk_005", name: "Amina Diallo", amount: "1980.45", currency: USD},
		{id: "wrk_006", name: "Noah Williams", amount: "1750.00", currency: EUR},
		{id: "wrk_007", name: "Priya Shah", amount: "2200.10", currency: USD},
		{id: "wrk_008", name: "Mateo Ruiz", amount: "2450.75", currency: EUR},
		{id: "wrk_009", name: "Lina Haddad", amount: "1625.30", currency: USD},
		{id: "wrk_010", name: "Jonas Berg", amount: "2050.60", currency: EUR},
	}

	workers := make([]Worker, 0, len(seeds))
	for _, seed := range seeds {
		amount, err := ParseMoney(seed.amount, seed.currency)
		if err != nil {
			return nil, fmt.Errorf("seed worker %q amount: %w", seed.id, err)
		}
		worker, err := NewWorker(seed.id, seed.name, amount)
		if err != nil {
			return nil, fmt.Errorf("seed worker %q: %w", seed.id, err)
		}
		workers = append(workers, worker)
	}

	return workers, nil
}
