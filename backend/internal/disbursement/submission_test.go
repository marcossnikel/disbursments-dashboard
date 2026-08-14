package disbursement_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/marcosnikel/cadana-disbursement-tool/backend/internal/disbursement"
)

func TestProcessorCreatesProviderWorkOnceForSimultaneousReplays(t *testing.T) {
	t.Parallel()

	provider := newCountingProvider()
	processor := newTestProcessor(t, provider, 2)

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

	waitForPaymentStarts(t, provider.started, 2)
	close(provider.release)
	waitForCompletedBatch(t, processor, "batch-idempotent")
	for _, workerID := range []disbursement.WorkerID{"w-001", "w-002"} {
		if got, want := provider.callCount(workerID), 1; got != want {
			t.Errorf("provider calls for %q = %d, want %d", workerID, got, want)
		}
	}
}

func TestProcessorRejectsInvalidSubmissionsBeforeStartingPayments(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		ctx       context.Context
		batchID   disbursement.BatchID
		workerIDs []disbursement.WorkerID
	}{
		{
			name: "missing context", batchID: "batch-invalid",
			workerIDs: []disbursement.WorkerID{"w-001"},
		},
		{
			name: "missing batch ID", ctx: context.Background(),
			workerIDs: []disbursement.WorkerID{"w-001"},
		},
		{
			name: "blank batch ID", ctx: context.Background(), batchID: "  ",
			workerIDs: []disbursement.WorkerID{"w-001"},
		},
		{name: "missing workers", ctx: context.Background(), batchID: "batch-invalid"},
		{
			name: "blank worker ID", ctx: context.Background(), batchID: "batch-invalid",
			workerIDs: []disbursement.WorkerID{"  "},
		},
		{
			name: "duplicate worker ID", ctx: context.Background(), batchID: "batch-invalid",
			workerIDs: []disbursement.WorkerID{"w-001", "w-001"},
		},
		{
			name: "unknown worker ID", ctx: context.Background(), batchID: "batch-invalid",
			workerIDs: []disbursement.WorkerID{"w-999"},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			provider := newCountingProvider()
			processor := newTestProcessor(t, provider, 1)
			_, err := processor.Submit(testCase.ctx, testCase.batchID, testCase.workerIDs)
			if !errors.Is(err, disbursement.ErrInvalidSubmission) {
				t.Fatalf(
					"Submit(%q, %v) error = %v, want ErrInvalidSubmission",
					testCase.batchID,
					testCase.workerIDs,
					err,
				)
			}
			if _, found := processor.Batch(testCase.batchID); found {
				t.Errorf("Batch(%q) found = true, want false", testCase.batchID)
			}
			select {
			case request := <-provider.started:
				t.Errorf("provider received unexpected request for worker %q", request.WorkerID)
			default:
			}
		})
	}
}

func TestProcessorRejectsAnEntireBatchWhenOneWorkerIsPending(t *testing.T) {
	t.Parallel()

	provider := newCountingProvider()
	processor := newTestProcessor(t, provider, 2)

	if _, err := processor.Submit(context.Background(), "batch-first", []disbursement.WorkerID{"w-001"}); err != nil {
		t.Fatalf("first Submit() error = %v", err)
	}
	waitForPaymentStarts(t, provider.started, 1)

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
	processor := newTestProcessor(t, provider, 2)

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

	waitForPaymentStarts(t, provider.started, 1)
	close(provider.release)
	waitForCompletedBatch(t, processor, "batch-conflict")
}
