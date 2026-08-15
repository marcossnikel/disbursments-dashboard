package disbursement_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/marcosnikel/cadana-disbursement-tool/backend/internal/disbursement"
)

type resultProvider struct {
	result disbursement.PaymentResult
	err    error
}

type scriptedProvider struct {
	mu       sync.Mutex
	errors   []error
	requests []disbursement.PaymentRequest
}

type retryGateProvider struct {
	mu                   sync.Mutex
	requests             []disbursement.PaymentRequest
	firstAttemptFinished chan struct{}
	retryStarted         chan struct{}
	releaseRetry         chan struct{}
}

func (provider resultProvider) Pay(
	context.Context,
	disbursement.PaymentRequest,
) (disbursement.PaymentResult, error) {
	return provider.result, provider.err
}

func (provider *scriptedProvider) Pay(
	_ context.Context,
	request disbursement.PaymentRequest,
) (disbursement.PaymentResult, error) {
	provider.mu.Lock()
	defer provider.mu.Unlock()

	provider.requests = append(provider.requests, request)
	callIndex := len(provider.requests) - 1
	if callIndex < len(provider.errors) {
		return disbursement.PaymentResult{}, provider.errors[callIndex]
	}
	return disbursement.PaymentResult{
		ProviderTransactionID: "ptx-after-retry",
	}, nil
}

func (provider *scriptedProvider) recordedRequests() []disbursement.PaymentRequest {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	return append([]disbursement.PaymentRequest(nil), provider.requests...)
}

func newRetryGateProvider() *retryGateProvider {
	return &retryGateProvider{
		firstAttemptFinished: make(chan struct{}),
		retryStarted:         make(chan struct{}),
		releaseRetry:         make(chan struct{}),
	}
}

func (provider *retryGateProvider) Pay(
	ctx context.Context,
	request disbursement.PaymentRequest,
) (disbursement.PaymentResult, error) {
	provider.mu.Lock()
	provider.requests = append(provider.requests, request)
	attempt := len(provider.requests)
	provider.mu.Unlock()

	if attempt == 1 {
		close(provider.firstAttemptFinished)
		return disbursement.PaymentResult{}, &disbursement.ProviderFailure{
			Code: disbursement.ProviderTimeout, Message: "temporary timeout",
		}
	}

	close(provider.retryStarted)
	select {
	case <-provider.releaseRetry:
		return disbursement.PaymentResult{ProviderTransactionID: "ptx-retry"}, nil
	case <-ctx.Done():
		return disbursement.PaymentResult{}, ctx.Err()
	}
}

func TestProcessorAutomaticallyRetriesTransientFailureInSameBatch(t *testing.T) {
	t.Parallel()

	provider := &scriptedProvider{errors: []error{&disbursement.ProviderFailure{
		Code: disbursement.ProviderTimeout, Message: "temporary timeout",
	}}}
	processor, err := disbursement.NewProcessor(
		testWorkers(t, 1),
		disbursement.ProcessorConfig{
			Provider:            provider,
			ProviderTimeout:     time.Second,
			ProviderMaxAttempts: 2,
			ProviderRetryDelay:  0,
		},
	)
	if err != nil {
		t.Fatalf("NewProcessor() error = %v", err)
	}

	const batchID = disbursement.BatchID("batch-automatic-retry")
	if _, err := processor.Submit(
		context.Background(),
		batchID,
		[]disbursement.WorkerID{"w-001"},
	); err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	completed := waitForCompletedBatch(t, processor, batchID)
	result := completed.Results[0]
	if got, want := result.Status, disbursement.StatusSuccess; got != want {
		t.Errorf("result status = %q, want %q", got, want)
	}
	if got, want := result.Attempts, 2; got != want {
		t.Errorf("result attempts = %d, want %d", got, want)
	}

	requests := provider.recordedRequests()
	if got, want := len(requests), 2; got != want {
		t.Fatalf("provider call count = %d, want %d", got, want)
	}
	if got, want := requests[1].DisbursementID, requests[0].DisbursementID; got != want {
		t.Errorf("retry disbursement ID = %q, want stable ID %q", got, want)
	}

	replay, err := processor.Submit(
		context.Background(),
		batchID,
		[]disbursement.WorkerID{"w-001"},
	)
	if err != nil {
		t.Fatalf("replayed Submit() error = %v", err)
	}
	if replay.Created {
		t.Error("replayed Submit() Created = true, want false")
	}
	if got, want := len(provider.recordedRequests()), 2; got != want {
		t.Errorf("provider calls after replay = %d, want %d", got, want)
	}
}

func TestProcessorKeepsObligationReservedDuringAutomaticRetry(t *testing.T) {
	t.Parallel()

	provider := newRetryGateProvider()
	processor, err := disbursement.NewProcessor(
		testWorkers(t, 1),
		disbursement.ProcessorConfig{
			Provider:            provider,
			ProviderTimeout:     time.Second,
			ProviderMaxAttempts: 2,
		},
	)
	if err != nil {
		t.Fatalf("NewProcessor() error = %v", err)
	}

	if _, err := processor.Submit(
		context.Background(),
		"batch-retrying",
		[]disbursement.WorkerID{"w-001"},
	); err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	<-provider.firstAttemptFinished
	<-provider.retryStarted

	_, competingErr := processor.Submit(
		context.Background(),
		"batch-competing",
		[]disbursement.WorkerID{"w-001"},
	)
	var unavailableError *disbursement.WorkersUnavailableError
	if !errors.As(competingErr, &unavailableError) {
		t.Fatalf("competing Submit() error = %v, want WorkersUnavailableError", competingErr)
	}
	if got, want := unavailableError.Workers[0].Reason, disbursement.AlreadyPending; got != want {
		t.Errorf("unavailable reason = %q, want %q", got, want)
	}
	if _, found := processor.Batch("batch-competing"); found {
		t.Error("Batch(batch-competing) found = true, want false")
	}

	close(provider.releaseRetry)
	completed := waitForCompletedBatch(t, processor, "batch-retrying")
	if got, want := completed.Results[0].Status, disbursement.StatusSuccess; got != want {
		t.Errorf("result status = %q, want %q", got, want)
	}
}

func TestProcessorDoesNotAutomaticallyRetryProviderDecline(t *testing.T) {
	t.Parallel()

	provider := &scriptedProvider{errors: []error{&disbursement.ProviderFailure{
		Code: disbursement.ProviderDeclined, Message: "payment was declined",
	}}}
	processor, err := disbursement.NewProcessor(
		testWorkers(t, 1),
		disbursement.ProcessorConfig{
			Provider:            provider,
			ProviderTimeout:     time.Second,
			ProviderMaxAttempts: 2,
			ProviderRetryDelay:  0,
		},
	)
	if err != nil {
		t.Fatalf("NewProcessor() error = %v", err)
	}

	if _, err := processor.Submit(
		context.Background(),
		"batch-declined",
		[]disbursement.WorkerID{"w-001"},
	); err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	completed := waitForCompletedBatch(t, processor, "batch-declined")
	result := completed.Results[0]
	if got, want := result.Status, disbursement.StatusFailed; got != want {
		t.Errorf("result status = %q, want %q", got, want)
	}
	if got, want := result.Attempts, 1; got != want {
		t.Errorf("result attempts = %d, want %d", got, want)
	}
	if got, want := len(provider.recordedRequests()), 1; got != want {
		t.Errorf("provider call count = %d, want %d", got, want)
	}
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

func TestProcessorReleasesWorkerAfterAutomaticRetriesAreExhausted(t *testing.T) {
	t.Parallel()

	provider := &scriptedProvider{errors: []error{
		&disbursement.ProviderFailure{Code: disbursement.ProviderTimeout, Message: "first timeout"},
		&disbursement.ProviderFailure{Code: disbursement.ProviderTimeout, Message: "second timeout"},
	}}
	processor, err := disbursement.NewProcessor(
		testWorkers(t, 1),
		disbursement.ProcessorConfig{
			Provider:            provider,
			ProviderTimeout:     time.Second,
			ProviderMaxAttempts: 2,
		},
	)
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
	if got, want := completed.Results[0].Attempts, 2; got != want {
		t.Errorf("result attempts = %d, want %d", got, want)
	}
	if got, want := len(processor.AvailableWorkers()), 1; got != want {
		t.Fatalf("available worker count = %d, want %d", got, want)
	}

	if _, err := processor.Submit(context.Background(), "batch-new-attempt", []disbursement.WorkerID{"w-001"}); err != nil {
		t.Fatalf("new attempt Submit() error = %v", err)
	}
	newAttempt := waitForCompletedBatch(t, processor, "batch-new-attempt")
	if got, want := newAttempt.Results[0].Status, disbursement.StatusSuccess; got != want {
		t.Errorf("new attempt status = %q, want %q", got, want)
	}

	requests := provider.recordedRequests()
	if got, want := len(requests), 3; got != want {
		t.Fatalf("provider call count = %d, want %d", got, want)
	}
	if got, want := requests[1].DisbursementID, requests[0].DisbursementID; got != want {
		t.Errorf("automatic retry disbursement ID = %q, want %q", got, want)
	}
	if got, previous := requests[2].DisbursementID, requests[1].DisbursementID; got == previous {
		t.Errorf("new batch disbursement ID = %q, want a new ID", got)
	}
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
