package disbursement_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/marcosnikel/cadana-disbursement-tool/backend/internal/disbursement"
)

type resultProvider struct {
	result disbursement.PaymentResult
	err    error
}

func (provider resultProvider) Pay(
	context.Context,
	disbursement.PaymentRequest,
) (disbursement.PaymentResult, error) {
	return provider.result, provider.err
}

func TestProcessorExposesConcurrentPendingWorkAndIndependentResults(t *testing.T) {
	t.Parallel()

	provider := newGatedProvider("w-002")
	processor := newTestProcessor(t, provider, 2)

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
	for _, request := range waitForPaymentStarts(t, provider.started, 2) {
		startedWorkers[request.WorkerID] = true
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

	processor := newTestProcessorWithTimeout(t, timeoutProvider{}, 1, 10*time.Millisecond)

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

func TestProcessorRecordsProviderOutcomes(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name                 string
		provider             disbursement.PaymentProvider
		wantStatus           disbursement.DisbursementStatus
		wantErrorCode        disbursement.ProviderErrorCode
		wantTransactionID    disbursement.ProviderTransactionID
		wantAvailableWorkers int
	}{
		{
			name: "success with transaction ID",
			provider: resultProvider{result: disbursement.PaymentResult{
				ProviderTransactionID: "ptx-success",
			}},
			wantStatus:           disbursement.StatusSuccess,
			wantTransactionID:    "ptx-success",
			wantAvailableWorkers: 0,
		},
		{
			name: "typed provider decline",
			provider: resultProvider{err: &disbursement.ProviderFailure{
				Code: disbursement.ProviderDeclined, Message: "the provider declined this payment",
			}},
			wantStatus:           disbursement.StatusFailed,
			wantErrorCode:        disbursement.ProviderDeclined,
			wantAvailableWorkers: 1,
		},
		{
			name:                 "provider deadline",
			provider:             resultProvider{err: context.DeadlineExceeded},
			wantStatus:           disbursement.StatusFailed,
			wantErrorCode:        disbursement.ProviderTimeout,
			wantAvailableWorkers: 1,
		},
		{
			name:                 "generic provider error",
			provider:             resultProvider{err: errors.New("provider unavailable")},
			wantStatus:           disbursement.StatusFailed,
			wantErrorCode:        disbursement.ProviderError,
			wantAvailableWorkers: 1,
		},
		{
			name:                 "missing transaction ID",
			provider:             resultProvider{},
			wantStatus:           disbursement.StatusFailed,
			wantErrorCode:        disbursement.ProviderError,
			wantAvailableWorkers: 1,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			processor := newTestProcessor(t, testCase.provider, 1)
			const batchID = disbursement.BatchID("batch-provider-outcome")
			if _, err := processor.Submit(
				context.Background(),
				batchID,
				[]disbursement.WorkerID{"w-001"},
			); err != nil {
				t.Fatalf("Submit(%q) error = %v", batchID, err)
			}

			completed := waitForCompletedBatch(t, processor, batchID)
			result := completed.Results[0]
			if got := result.Status; got != testCase.wantStatus {
				t.Errorf("Batch(%q) result status = %q, want %q", batchID, got, testCase.wantStatus)
			}
			if got := result.ErrorCode; got != testCase.wantErrorCode {
				t.Errorf("Batch(%q) error code = %q, want %q", batchID, got, testCase.wantErrorCode)
			}
			if got := result.ProviderTransactionID; got != testCase.wantTransactionID {
				t.Errorf(
					"Batch(%q) provider transaction ID = %q, want %q",
					batchID,
					got,
					testCase.wantTransactionID,
				)
			}
			if got := len(processor.AvailableWorkers()); got != testCase.wantAvailableWorkers {
				t.Errorf(
					"len(AvailableWorkers()) = %d, want %d",
					got,
					testCase.wantAvailableWorkers,
				)
			}
		})
	}
}
