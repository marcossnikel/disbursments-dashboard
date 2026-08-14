package disbursement_test

import (
	"context"
	"testing"
	"time"

	"github.com/marcosnikel/cadana-disbursement-tool/backend/internal/disbursement"
)

func TestProcessorResetRestoresWorkersAndClearsCompletedBatches(t *testing.T) {
	t.Parallel()

	provider := newCountingProvider()
	processor, err := disbursement.NewProcessor(testWorkers(t, 2), disbursement.ProcessorConfig{
		Provider: provider, ProviderTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("NewProcessor() error = %v", err)
	}

	if _, err := processor.Submit(context.Background(), "batch-before-reset", []disbursement.WorkerID{"w-001"}); err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	select {
	case <-provider.started:
	case <-time.After(time.Second):
		t.Fatal("provider payment did not start")
	}
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
}
