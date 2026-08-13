package disbursement_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/marcosnikel/cadana-disbursement-tool/backend/internal/disbursement"
)

func TestProcessorExposesConcurrentPendingWorkAndIndependentResults(t *testing.T) {
	t.Parallel()

	workers := firstSeedWorkers(t, 2)
	provider := newGatedProvider("w-002")
	processor, err := disbursement.NewProcessor(
		context.Background(),
		workers,
		disbursement.ProcessorConfig{
			Provider:        provider,
			ProviderTimeout: time.Second,
		},
	)
	if err != nil {
		t.Fatalf("NewProcessor() error = %v", err)
	}

	submission, err := processor.Submit(
		disbursement.BatchID("batch-concurrent"),
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

	resultsByWorker := make(map[disbursement.WorkerID]disbursement.Result, len(completed.Results))
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

func TestProcessorCreatesProviderWorkOnceForSimultaneousReplays(t *testing.T) {
	t.Parallel()

	workers := firstSeedWorkers(t, 2)
	provider := newCountingProvider()
	processor, err := disbursement.NewProcessor(
		context.Background(),
		workers,
		disbursement.ProcessorConfig{
			Provider:        provider,
			ProviderTimeout: time.Second,
		},
	)
	if err != nil {
		t.Fatalf("NewProcessor() error = %v", err)
	}

	const submitterCount = 20
	start := make(chan struct{})
	submissions := make(chan disbursement.Submission, submitterCount)
	errors := make(chan error, submitterCount)
	var submitters sync.WaitGroup
	for range submitterCount {
		submitters.Go(func() {
			<-start
			submission, submitErr := processor.Submit(
				disbursement.BatchID("batch-idempotent"),
				[]disbursement.WorkerID{"w-001", "w-002"},
			)
			submissions <- submission
			errors <- submitErr
		})
	}
	close(start)
	submitters.Wait()
	close(submissions)
	close(errors)

	createdCount := 0
	for submitErr := range errors {
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

	workers := firstSeedWorkers(t, 1)
	provider := newCountingProvider()
	processor, err := disbursement.NewProcessor(
		context.Background(),
		workers,
		disbursement.ProcessorConfig{
			Provider:        provider,
			ProviderTimeout: time.Second,
		},
	)
	if err != nil {
		t.Fatalf("NewProcessor() error = %v", err)
	}

	_, submitErr := processor.Submit("batch-invalid", []disbursement.WorkerID{"w-999"})
	if !errors.Is(submitErr, disbursement.ErrInvalidSubmission) {
		t.Fatalf("Submit() error = %v, want ErrInvalidSubmission", submitErr)
	}
	if got := provider.callCount("w-999"); got != 0 {
		t.Fatalf("provider calls for unknown worker = %d, want 0", got)
	}
}

func TestProcessorRejectsAnEntireBatchWhenOneWorkerIsPending(t *testing.T) {
	t.Parallel()

	workers := firstSeedWorkers(t, 2)
	provider := newCountingProvider()
	processor, err := disbursement.NewProcessor(
		context.Background(),
		workers,
		disbursement.ProcessorConfig{
			Provider:        provider,
			ProviderTimeout: time.Second,
		},
	)
	if err != nil {
		t.Fatalf("NewProcessor() error = %v", err)
	}

	if _, err := processor.Submit("batch-first", []disbursement.WorkerID{"w-001"}); err != nil {
		t.Fatalf("first Submit() error = %v", err)
	}
	select {
	case <-provider.started:
	case <-time.After(time.Second):
		t.Fatal("first provider payment did not start")
	}

	_, submitErr := processor.Submit(
		"batch-blocked",
		[]disbursement.WorkerID{"w-001", "w-002"},
	)
	var unavailableError *disbursement.WorkersUnavailableError
	if !errors.As(submitErr, &unavailableError) {
		t.Fatalf("second Submit() error = %v, want WorkersUnavailableError", submitErr)
	}
	if got, want := len(unavailableError.Workers), 1; got != want {
		t.Fatalf("unavailable worker count = %d, want %d", got, want)
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

func TestProcessorBlocksAnotherAttemptWhenProviderOutcomeIsUnknown(t *testing.T) {
	t.Parallel()

	workers := firstSeedWorkers(t, 1)
	processor, err := disbursement.NewProcessor(
		context.Background(),
		workers,
		disbursement.ProcessorConfig{
			Provider:        timeoutProvider{},
			ProviderTimeout: 10 * time.Millisecond,
		},
	)
	if err != nil {
		t.Fatalf("NewProcessor() error = %v", err)
	}

	if _, err := processor.Submit("batch-unknown", []disbursement.WorkerID{"w-001"}); err != nil {
		t.Fatalf("first Submit() error = %v", err)
	}
	completed := waitForCompletedBatch(t, processor, "batch-unknown")
	if got, want := completed.Results[0].Status, disbursement.StatusOutcomeUnknown; got != want {
		t.Errorf("result status = %q, want %q", got, want)
	}
	if got, want := completed.Results[0].ErrorCode, disbursement.ProviderTimeout; got != want {
		t.Errorf("result error code = %q, want %q", got, want)
	}

	_, submitErr := processor.Submit("batch-unsafe-retry", []disbursement.WorkerID{"w-001"})
	var unavailableError *disbursement.WorkersUnavailableError
	if !errors.As(submitErr, &unavailableError) {
		t.Fatalf("retry Submit() error = %v, want WorkersUnavailableError", submitErr)
	}
	if got, want := unavailableError.Workers[0].Reason, disbursement.OutcomeUnknown; got != want {
		t.Errorf("unavailable reason = %q, want %q", got, want)
	}
}

func TestProcessorRejectsChangedWorkersForAnExistingBatchID(t *testing.T) {
	t.Parallel()

	workers := firstSeedWorkers(t, 2)
	provider := newCountingProvider()
	processor, err := disbursement.NewProcessor(
		context.Background(),
		workers,
		disbursement.ProcessorConfig{
			Provider:        provider,
			ProviderTimeout: time.Second,
		},
	)
	if err != nil {
		t.Fatalf("NewProcessor() error = %v", err)
	}

	if _, err := processor.Submit("batch-conflict", []disbursement.WorkerID{"w-001"}); err != nil {
		t.Fatalf("first Submit() error = %v", err)
	}
	_, submitErr := processor.Submit(
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

type gatedProvider struct {
	started       chan disbursement.PaymentRequest
	release       chan struct{}
	failingWorker disbursement.WorkerID
}

type countingProvider struct {
	mu      sync.Mutex
	calls   map[disbursement.WorkerID]int
	started chan disbursement.PaymentRequest
	release chan struct{}
}

type timeoutProvider struct{}

func (timeoutProvider) Pay(
	ctx context.Context,
	_ disbursement.PaymentRequest,
) (disbursement.PaymentResult, error) {
	<-ctx.Done()
	return disbursement.PaymentResult{}, ctx.Err()
}

func newCountingProvider() *countingProvider {
	return &countingProvider{
		calls:   make(map[disbursement.WorkerID]int),
		started: make(chan disbursement.PaymentRequest, 2),
		release: make(chan struct{}),
	}
}

func (p *countingProvider) Pay(
	ctx context.Context,
	request disbursement.PaymentRequest,
) (disbursement.PaymentResult, error) {
	p.mu.Lock()
	p.calls[request.WorkerID]++
	p.mu.Unlock()
	p.started <- request

	select {
	case <-p.release:
	case <-ctx.Done():
		return disbursement.PaymentResult{}, ctx.Err()
	}

	return disbursement.PaymentResult{
		ProviderTransactionID: disbursement.ProviderTransactionID("ptx-" + request.WorkerID),
	}, nil
}

func (p *countingProvider) callCount(workerID disbursement.WorkerID) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls[workerID]
}

func newGatedProvider(failingWorker disbursement.WorkerID) *gatedProvider {
	return &gatedProvider{
		started:       make(chan disbursement.PaymentRequest, 2),
		release:       make(chan struct{}),
		failingWorker: failingWorker,
	}
}

func (p *gatedProvider) Pay(ctx context.Context, request disbursement.PaymentRequest) (disbursement.PaymentResult, error) {
	p.started <- request

	select {
	case <-p.release:
	case <-ctx.Done():
		return disbursement.PaymentResult{}, ctx.Err()
	}

	if request.WorkerID == p.failingWorker {
		return disbursement.PaymentResult{}, &disbursement.ProviderFailure{
			Code:    disbursement.ProviderDeclined,
			Message: "the provider declined this payment",
		}
	}

	return disbursement.PaymentResult{
		ProviderTransactionID: disbursement.ProviderTransactionID(fmt.Sprintf("ptx-%s", request.WorkerID)),
	}, nil
}

func firstSeedWorkers(t *testing.T, count int) []disbursement.Worker {
	t.Helper()

	workers, err := disbursement.SeedWorkers()
	if err != nil {
		t.Fatalf("SeedWorkers() error = %v", err)
	}
	return workers[:count]
}

func waitForCompletedBatch(
	t *testing.T,
	processor *disbursement.Processor,
	batchID disbursement.BatchID,
) disbursement.BatchSnapshot {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		snapshot, found := processor.Batch(batchID)
		if found && snapshot.Status == disbursement.BatchCompleted {
			return snapshot
		}
		time.Sleep(time.Millisecond)
	}

	t.Fatalf("batch %q did not complete before the deadline", batchID)
	return disbursement.BatchSnapshot{}
}
