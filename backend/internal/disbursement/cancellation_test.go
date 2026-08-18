package disbursement_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/marcosnikel/cadana-disbursement-tool/backend/internal/disbursement"
)

type mixedGatedProvider struct {
	started chan disbursement.PaymentRequest
	release chan struct{}

	mu        sync.Mutex
	callCount int
}

func newMixedGatedProvider() *mixedGatedProvider {
	return &mixedGatedProvider{
		started: make(chan disbursement.PaymentRequest, 3),
		release: make(chan struct{}),
	}
}

func (p *mixedGatedProvider) Pay(
	ctx context.Context,
	request disbursement.PaymentRequest,
) (disbursement.PaymentResult, error) {
	p.mu.Lock()
	p.callCount++
	callNumber := p.callCount
	p.mu.Unlock()
	p.started <- request

	select {
	case <-p.release:
	case <-ctx.Done():
		return disbursement.PaymentResult{}, ctx.Err()
	}
	if callNumber == 2 {
		return disbursement.PaymentResult{}, &disbursement.ProviderFailure{
			Code: disbursement.ProviderDeclined, Message: "the provider declined this payment",
		}
	}
	return disbursement.PaymentResult{
		ProviderTransactionID: disbursement.ProviderTransactionID("ptx-" + request.WorkerID),
	}, nil
}

func TestProcessorCancelsQueuedPaymentsWithoutInterruptingProviderCalls(t *testing.T) {
	t.Parallel()

	provider := newMixedGatedProvider()
	processor, err := disbursement.NewProcessor(
		testWorkers(t, 3),
		disbursement.ProcessorConfig{
			Provider:                   provider,
			ProviderTimeout:            time.Second,
			ProviderMaxConcurrentCalls: 2,
		},
	)
	if err != nil {
		t.Fatalf("NewProcessor() error = %v", err)
	}

	const batchID = disbursement.BatchID("batch-cancel-mixed")
	workerIDs := []disbursement.WorkerID{"w-001", "w-002", "w-003"}
	if _, err := processor.Submit(context.Background(), batchID, workerIDs); err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	started := waitForPaymentStarts(t, provider.started, 2)
	startedWorkers := map[disbursement.WorkerID]bool{}
	for _, request := range started {
		startedWorkers[request.WorkerID] = true
	}

	cancellation, err := processor.CancelBatch(context.Background(), batchID)
	if err != nil {
		t.Fatalf("CancelBatch() error = %v", err)
	}
	if got, want := cancellation.CanceledCount, 1; got != want {
		t.Fatalf("CancelBatch() canceled count = %d, want %d", got, want)
	}
	results := resultsByWorker(cancellation.Batch)
	canceledWorkerID := disbursement.WorkerID("")
	for workerID, result := range results {
		if result.Status == disbursement.StatusCanceled {
			canceledWorkerID = workerID
			continue
		}
		if result.Status != disbursement.StatusInFlight {
			t.Errorf("started worker %q status = %q, want %q", workerID, result.Status, disbursement.StatusInFlight)
		}
	}
	if canceledWorkerID == "" {
		t.Fatal("canceled worker ID is empty, want one canceled payment")
	}
	if startedWorkers[canceledWorkerID] {
		t.Errorf("canceled worker %q had already reached the provider", canceledWorkerID)
	}
	select {
	case request := <-provider.started:
		t.Fatalf("canceled payment reached provider for worker %q", request.WorkerID)
	default:
	}

	repeated, err := processor.CancelBatch(context.Background(), batchID)
	if err != nil {
		t.Fatalf("repeated CancelBatch() error = %v", err)
	}
	if got := repeated.CanceledCount; got != 0 {
		t.Errorf("repeated CancelBatch() canceled count = %d, want 0", got)
	}

	close(provider.release)
	completed := waitForCompletedBatch(t, processor, batchID)
	results = resultsByWorker(completed)
	statusCounts := map[disbursement.DisbursementStatus]int{}
	for _, result := range results {
		statusCounts[result.Status]++
	}
	for _, status := range []disbursement.DisbursementStatus{
		disbursement.StatusSuccess,
		disbursement.StatusFailed,
		disbursement.StatusCanceled,
	} {
		if got, want := statusCounts[status], 1; got != want {
			t.Errorf("final %q count = %d, want %d", status, got, want)
		}
	}

	available := map[disbursement.WorkerID]bool{}
	for _, worker := range processor.AvailableWorkers() {
		available[worker.ID()] = true
	}
	if got, want := len(available), 2; got != want {
		t.Errorf("available worker count = %d, want %d", got, want)
	}
	if !available[canceledWorkerID] {
		t.Errorf("canceled worker %q is not available for a fresh batch", canceledWorkerID)
	}

	replay, err := processor.Submit(context.Background(), batchID, workerIDs)
	if err != nil {
		t.Fatalf("replayed Submit() error = %v", err)
	}
	if replay.Created {
		t.Error("replayed Submit() Created = true, want false")
	}
	select {
	case request := <-provider.started:
		t.Fatalf("replayed batch created provider work for worker %q", request.WorkerID)
	default:
	}

	if _, err := processor.Submit(
		context.Background(),
		"batch-after-cancel",
		[]disbursement.WorkerID{canceledWorkerID},
	); err != nil {
		t.Fatalf("Submit() for canceled obligation error = %v", err)
	}
	newRequest := waitForPaymentStarts(t, provider.started, 1)[0]
	if got, want := newRequest.WorkerID, canceledWorkerID; got != want {
		t.Errorf("new provider worker = %q, want %q", got, want)
	}
	waitForCompletedBatch(t, processor, "batch-after-cancel")
}

func TestProcessorRejectsCancellationForUnknownBatch(t *testing.T) {
	t.Parallel()

	processor := newTestProcessor(t, resultProvider{}, 1)
	_, err := processor.CancelBatch(context.Background(), "batch-missing")
	if !errors.Is(err, disbursement.ErrBatchNotFound) {
		t.Fatalf("CancelBatch() error = %v, want ErrBatchNotFound", err)
	}
}

func resultsByWorker(snapshot disbursement.BatchSnapshot) map[disbursement.WorkerID]disbursement.DisbursementResult {
	results := make(map[disbursement.WorkerID]disbursement.DisbursementResult, len(snapshot.Results))
	for _, result := range snapshot.Results {
		results[result.Worker.ID()] = result
	}
	return results
}
