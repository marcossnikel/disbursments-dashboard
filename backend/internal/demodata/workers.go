// Package demodata builds the in-memory fixtures used by the reviewer-facing demo.
package demodata

import (
	"fmt"

	"github.com/marcosnikel/cadana-disbursement-tool/backend/internal/disbursement"
)

// Workers returns the sample payment obligations displayed by the dashboard.
func Workers() ([]disbursement.Worker, error) {
	seeds := []struct {
		id       disbursement.WorkerID
		name     string
		amount   string
		currency disbursement.Currency
	}{
		{id: "w-001", name: "Maya Thompson", amount: "1500.50", currency: disbursement.USD},
		{id: "w-002", name: "Daniel Kim", amount: "2300.00", currency: disbursement.EUR},
		{id: "w-003", name: "Sofia Martinez", amount: "1875.25", currency: disbursement.USD},
		{id: "w-004", name: "Lucas Ferreira", amount: "2140.80", currency: disbursement.EUR},
		{id: "w-005", name: "Amina Diallo", amount: "1980.45", currency: disbursement.USD},
		{id: "w-006", name: "Noah Williams", amount: "1750.00", currency: disbursement.EUR},
		{id: "w-007", name: "Priya Shah", amount: "2200.10", currency: disbursement.USD},
		{id: "w-008", name: "Mateo Ruiz", amount: "2450.75", currency: disbursement.EUR},
		{id: "w-009", name: "Lina Haddad", amount: "1625.30", currency: disbursement.USD},
		{id: "w-010", name: "Jonas Berg", amount: "2050.60", currency: disbursement.EUR},
	}

	workers := make([]disbursement.Worker, 0, len(seeds))
	for _, seed := range seeds {
		amount, err := disbursement.ParseMoney(seed.amount, seed.currency)
		if err != nil {
			return nil, fmt.Errorf("seed worker %q amount: %w", seed.id, err)
		}
		worker, err := disbursement.NewWorker(seed.id, seed.name, amount)
		if err != nil {
			return nil, fmt.Errorf("seed worker %q: %w", seed.id, err)
		}
		workers = append(workers, worker)
	}

	return workers, nil
}
