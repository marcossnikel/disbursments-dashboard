package disbursement_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/marcosnikel/cadana-disbursement-tool/backend/internal/disbursement"
)

func TestProcessorCreatesProviderWorkOnceForSimultaneousReplays(t *testing.T) {
	t.Parallel()

	provider := newCountingProvider()
	processor, err := disbursement.NewProcessor(testWorkers(t, 2), disbursement.ProcessorConfig{
		Provider: provider, ProviderTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("NewProcessor() error = %v", err)
	}

	const submitterCount = 20
	start := make(chan struct{})
	submissions := make(chan disbursement.Submission, submitterCount)
	errorsChannel := make(chan error, submitterCount)
	var submitters sync.WaitGroup
	for range submitterCount {
		submitters.Go(func() {
			<-start
			submission, submitErr := processor.Submit(
				context.Background(),
				"batch-idempotent",
				[]disbursement.WorkerID{"w-001", "w-002"},
			)
			submissions <- submission
			errorsChannel <- submitErr
		})
	}
	close(start)
	submitters.Wait()
	close(submissions)
	close(errorsChannel)

	createdCount := 0
	for submitErr := range errorsChannel {
		if submitErr != nil {
			t.Errorf("Submit() error = %v", submitErr)
		}
	}
	for submission := range submissions {
		if submission.BatchID != "batch-idempotent" {
			t.Errorf("Submit() batch ID = %q, want batch-idempotent", submission.BatchID)
		}
		if submission.Created {
			createdCount++
		}
	}
	if got, want := createdCount, 1; got != want {
		t.Fatalf("created submission count = %d, want %d", got, want)
	}

	for range 2 {
		select {
		case <-provider.started:
		case <-time.After(time.Second):
			t.Fatal("expected provider work was not started")
		}
	}
	close(provider.release)
	waitForCompletedBatch(t, processor, "batch-idempotent")
	for _, workerID := range []disbursement.WorkerID{"w-001", "w-002"} {
		if got, want := provider.callCount(workerID), 1; got != want {
			t.Errorf("provider calls for %q = %d, want %d", workerID, got, want)
		}
	}
}

func TestProcessorRejectsAnUnknownWorkerBeforeStartingPayments(t *testing.T) {
	t.Parallel()

	provider := newCountingProvider()
	processor, err := disbursement.NewProcessor(testWorkers(t, 1), disbursement.ProcessorConfig{
		Provider: provider, ProviderTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("NewProcessor() error = %v", err)
	}

	_, submitErr := processor.Submit(context.Background(), "batch-invalid", []disbursement.WorkerID{"w-999"})
	if !errors.Is(submitErr, disbursement.ErrInvalidSubmission) {
		t.Fatalf("Submit() error = %v, want ErrInvalidSubmission", submitErr)
	}
	if got := provider.callCount("w-999"); got != 0 {
		t.Fatalf("provider calls for unknown worker = %d, want 0", got)
	}
}

func TestProcessorRejectsAnEntireBatchWhenOneWorkerIsPending(t *testing.T) {
	t.Parallel()

	provider := newCountingProvider()
	processor, err := disbursement.NewProcessor(testWorkers(t, 2), disbursement.ProcessorConfig{
		Provider: provider, ProviderTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("NewProcessor() error = %v", err)
	}

	if _, err := processor.Submit(context.Background(), "batch-first", []disbursement.WorkerID{"w-001"}); err != nil {
		t.Fatalf("first Submit() error = %v", err)
	}
	select {
	case <-provider.started:
	case <-time.After(time.Second):
		t.Fatal("first provider payment did not start")
	}

	_, submitErr := processor.Submit(
		context.Background(),
		"batch-blocked",
		[]disbursement.WorkerID{"w-001", "w-002"},
	)
	var unavailableError *disbursement.WorkersUnavailableError
	if !errors.As(submitErr, &unavailableError) {
		t.Fatalf("second Submit() error = %v, want WorkersUnavailableError", submitErr)
	}
	if got, want := unavailableError.Workers[0].Reason, disbursement.AlreadyPending; got != want {
		t.Errorf("unavailable reason = %q, want %q", got, want)
	}
	if _, found := processor.Batch("batch-blocked"); found {
		t.Error("Batch(batch-blocked) found = true, want false")
	}
	if got := provider.callCount("w-002"); got != 0 {
		t.Errorf("provider calls for available sibling = %d, want 0", got)
	}

	close(provider.release)
	waitForCompletedBatch(t, processor, "batch-first")
}

func TestProcessorRejectsChangedWorkersForAnExistingBatchID(t *testing.T) {
	t.Parallel()

	provider := newCountingProvider()
	processor, err := disbursement.NewProcessor(testWorkers(t, 2), disbursement.ProcessorConfig{
		Provider: provider, ProviderTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("NewProcessor() error = %v", err)
	}

	if _, err := processor.Submit(context.Background(), "batch-conflict", []disbursement.WorkerID{"w-001"}); err != nil {
		t.Fatalf("first Submit() error = %v", err)
	}
	_, submitErr := processor.Submit(
		context.Background(),
		"batch-conflict",
		[]disbursement.WorkerID{"w-001", "w-002"},
	)
	var conflictError *disbursement.IdempotencyConflictError
	if !errors.As(submitErr, &conflictError) {
		t.Fatalf("second Submit() error = %v, want IdempotencyConflictError", submitErr)
	}
	if got := provider.callCount("w-002"); got != 0 {
		t.Errorf("provider calls for changed worker = %d, want 0", got)
	}

	select {
	case <-provider.started:
	case <-time.After(time.Second):
		t.Fatal("original provider payment did not start")
	}
	close(provider.release)
	waitForCompletedBatch(t, processor, "batch-conflict")
}
