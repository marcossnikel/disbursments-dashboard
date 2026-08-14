package disbursement_test

import (
	"context"
	"testing"
	"time"

	"github.com/marcosnikel/cadana-disbursement-tool/backend/internal/disbursement"
)

func TestProcessorExposesConcurrentPendingWorkAndIndependentResults(t *testing.T) {
	t.Parallel()

	workers := testWorkers(t, 2)
	provider := newGatedProvider("w-002")
	processor, err := disbursement.NewProcessor(workers, disbursement.ProcessorConfig{
		Provider: provider, ProviderTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("NewProcessor() error = %v", err)
	}

	submission, err := processor.Submit(
		context.Background(),
		"batch-concurrent",
		[]disbursement.WorkerID{"w-001", "w-002"},
	)
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	if !submission.Created {
		t.Fatal("Submit() Created = false, want true")
	}

	snapshot, found := processor.Batch(submission.BatchID)
	if !found {
		t.Fatal("Batch() found = false, want true")
	}
	if got, want := snapshot.Status, disbursement.BatchProcessing; got != want {
		t.Errorf("batch status = %q, want %q", got, want)
	}
	for _, result := range snapshot.Results {
		if got, want := result.Status, disbursement.StatusPending; got != want {
			t.Errorf("worker %q status = %q, want %q", result.Worker.ID(), got, want)
		}
	}

	startedWorkers := map[disbursement.WorkerID]bool{}
	for range 2 {
		select {
		case request := <-provider.started:
			startedWorkers[request.WorkerID] = true
		case <-time.After(time.Second):
			t.Fatal("provider calls did not overlap before either was released")
		}
	}
	if !startedWorkers["w-001"] || !startedWorkers["w-002"] {
		t.Fatalf("started workers = %v, want w-001 and w-002", startedWorkers)
	}

	close(provider.release)
	completed := waitForCompletedBatch(t, processor, submission.BatchID)
	resultsByWorker := make(map[disbursement.WorkerID]disbursement.DisbursementResult, len(completed.Results))
	for _, result := range completed.Results {
		resultsByWorker[result.Worker.ID()] = result
	}
	if got, want := resultsByWorker["w-001"].Status, disbursement.StatusSuccess; got != want {
		t.Errorf("w-001 status = %q, want %q", got, want)
	}
	if got, want := resultsByWorker["w-001"].ProviderTransactionID, disbursement.ProviderTransactionID("ptx-w-001"); got != want {
		t.Errorf("w-001 provider transaction ID = %q, want %q", got, want)
	}
	if got, want := resultsByWorker["w-002"].Status, disbursement.StatusFailed; got != want {
		t.Errorf("w-002 status = %q, want %q", got, want)
	}
	if got, want := resultsByWorker["w-002"].ErrorCode, disbursement.ProviderDeclined; got != want {
		t.Errorf("w-002 error code = %q, want %q", got, want)
	}

	availableWorkers := processor.AvailableWorkers()
	if got, want := len(availableWorkers), 1; got != want {
		t.Fatalf("len(AvailableWorkers()) = %d, want %d", got, want)
	}
	if got, want := availableWorkers[0].ID(), disbursement.WorkerID("w-002"); got != want {
		t.Errorf("available worker ID = %q, want %q", got, want)
	}
}

func TestProcessorMakesTimedOutWorkerAvailableForANewAttempt(t *testing.T) {
	t.Parallel()

	processor, err := disbursement.NewProcessor(testWorkers(t, 1), disbursement.ProcessorConfig{
		Provider: timeoutProvider{}, ProviderTimeout: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewProcessor() error = %v", err)
	}

	if _, err := processor.Submit(context.Background(), "batch-timeout", []disbursement.WorkerID{"w-001"}); err != nil {
		t.Fatalf("first Submit() error = %v", err)
	}
	completed := waitForCompletedBatch(t, processor, "batch-timeout")
	if got, want := completed.Results[0].Status, disbursement.StatusFailed; got != want {
		t.Errorf("result status = %q, want %q", got, want)
	}
	if got, want := completed.Results[0].ErrorCode, disbursement.ProviderTimeout; got != want {
		t.Errorf("result error code = %q, want %q", got, want)
	}
	if got, want := len(processor.AvailableWorkers()), 1; got != want {
		t.Fatalf("available worker count = %d, want %d", got, want)
	}

	if _, err := processor.Submit(context.Background(), "batch-new-attempt", []disbursement.WorkerID{"w-001"}); err != nil {
		t.Fatalf("new attempt Submit() error = %v", err)
	}
	waitForCompletedBatch(t, processor, "batch-new-attempt")
}
