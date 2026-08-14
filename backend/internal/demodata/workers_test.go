package demodata_test

import (
	"testing"

	"github.com/marcosnikel/cadana-disbursement-tool/backend/internal/demodata"
	"github.com/marcosnikel/cadana-disbursement-tool/backend/internal/disbursement"
)

func TestWorkersBuildsDashboardFixturesWithExactMoney(t *testing.T) {
	t.Parallel()

	workers, err := demodata.Workers()
	if err != nil {
		t.Fatalf("Workers() error = %v", err)
	}
	if got, want := len(workers), 10; got != want {
		t.Fatalf("worker count = %d, want %d", got, want)
	}

	firstWorker := workers[0]
	if got, want := firstWorker.ID(), disbursement.WorkerID("w-001"); got != want {
		t.Errorf("first worker ID = %q, want %q", got, want)
	}
	if got, want := firstWorker.Name(), "Maya Thompson"; got != want {
		t.Errorf("first worker name = %q, want %q", got, want)
	}
	if got, want := firstWorker.Amount().String(), "1500.50"; got != want {
		t.Errorf("first worker amount = %q, want %q", got, want)
	}

	currencyCounts := map[disbursement.Currency]int{}
	for _, worker := range workers {
		currencyCounts[worker.Amount().Currency()]++
	}
	if currencyCounts[disbursement.USD] == 0 || currencyCounts[disbursement.EUR] == 0 {
		t.Fatalf("seed currencies = %v, want both USD and EUR", currencyCounts)
	}
}
