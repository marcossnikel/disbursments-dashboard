package disbursement_test

import (
	"context"
	"testing"

	"github.com/marcosnikel/cadana-disbursement-tool/backend/internal/disbursement"
)

func TestProcessorResetRestoresWorkersAndClearsCompletedBatches(t *testing.T) {
	t.Parallel()

	provider := newCountingProvider()
	processor := newTestProcessor(t, provider, 2)

	if _, err := processor.Submit(context.Background(), "batch-before-reset", []disbursement.WorkerID{"w-001"}); err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	waitForPaymentStarts(t, provider.started, 1)
	close(provider.release)
	waitForCompletedBatch(t, processor, "batch-before-reset")

	if err := processor.ResetDemo(); err != nil {
		t.Fatalf("ResetDemo() error = %v", err)
	}
	if got, want := len(processor.AvailableWorkers()), 2; got != want {
		t.Errorf("available workers after reset = %d, want %d", got, want)
	}
	if _, found := processor.Batch("batch-before-reset"); found {
		t.Error("Batch(batch-before-reset) found = true after reset, want false")
	}
	if got := len(processor.Batches()); got != 0 {
		t.Errorf("Batches() length after reset = %d, want 0", got)
	}
}
