package disbursement_test

import (
	"testing"

	"github.com/marcosnikel/cadana-disbursement-tool/backend/internal/disbursement"
)

func TestProcessorListsSeededWorkersWithExactMoney(t *testing.T) {
	t.Parallel()

	workers, err := disbursement.SeedWorkers()
	if err != nil {
		t.Fatalf("SeedWorkers() error = %v", err)
	}
	processor, err := disbursement.NewProcessor(workers)
	if err != nil {
		t.Fatalf("NewProcessor() error = %v", err)
	}

	availableWorkers := processor.AvailableWorkers()
	if got, want := len(availableWorkers), 10; got != want {
		t.Fatalf("len(AvailableWorkers()) = %d, want %d", got, want)
	}

	firstWorker := availableWorkers[0]
	if got, want := firstWorker.ID(), disbursement.WorkerID("w-001"); got != want {
		t.Errorf("first worker ID = %q, want %q", got, want)
	}
	if got, want := firstWorker.Name(), "Ada Lovelace"; got != want {
		t.Errorf("first worker name = %q, want %q", got, want)
	}
	if got, want := firstWorker.Amount().String(), "1500.50"; got != want {
		t.Errorf("first worker amount = %q, want %q", got, want)
	}

	currencyCounts := map[disbursement.Currency]int{}
	for _, worker := range availableWorkers {
		currencyCounts[worker.Amount().Currency()]++
	}
	if currencyCounts[disbursement.USD] == 0 || currencyCounts[disbursement.EUR] == 0 {
		t.Fatalf("seed currencies = %v, want both USD and EUR", currencyCounts)
	}
}
