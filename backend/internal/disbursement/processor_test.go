package disbursement_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/marcosnikel/cadana-disbursement-tool/backend/internal/disbursement"
)

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

func (timeoutProvider) Pay(ctx context.Context, _ disbursement.PaymentRequest) (disbursement.PaymentResult, error) {
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

func (p *countingProvider) Pay(ctx context.Context, request disbursement.PaymentRequest) (disbursement.PaymentResult, error) {
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

func newTestProcessor(
	t *testing.T,
	provider disbursement.PaymentProvider,
	workerCount int,
) *disbursement.Processor {
	t.Helper()
	return newTestProcessorWithTimeout(t, provider, workerCount, time.Second)
}

func newTestProcessorWithTimeout(
	t *testing.T,
	provider disbursement.PaymentProvider,
	workerCount int,
	providerTimeout time.Duration,
) *disbursement.Processor {
	t.Helper()

	processor, err := disbursement.NewProcessor(
		testWorkers(t, workerCount),
		disbursement.ProcessorConfig{
			Provider:            provider,
			ProviderTimeout:     providerTimeout,
			ProviderMaxAttempts: 1,
		},
	)
	if err != nil {
		t.Fatalf("NewProcessor() error = %v", err)
	}
	return processor
}

func waitForPaymentStarts(
	t *testing.T,
	started <-chan disbursement.PaymentRequest,
	count int,
) []disbursement.PaymentRequest {
	t.Helper()

	requests := make([]disbursement.PaymentRequest, 0, count)
	for range count {
		select {
		case request := <-started:
			requests = append(requests, request)
		case <-time.After(time.Second):
			t.Fatalf("provider calls started = %d, want %d", len(requests), count)
		}
	}
	return requests
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
			Code: disbursement.ProviderDeclined, Message: "the provider declined this payment",
		}
	}
	return disbursement.PaymentResult{
		ProviderTransactionID: disbursement.ProviderTransactionID(fmt.Sprintf("ptx-%s", request.WorkerID)),
	}, nil
}

func testWorkers(t *testing.T, count int) []disbursement.Worker {
	t.Helper()
	definitions := []struct {
		id           disbursement.WorkerID
		name, amount string
		currency     disbursement.Currency
	}{
		{id: "w-001", name: "Worker One", amount: "100.00", currency: disbursement.USD},
		{id: "w-002", name: "Worker Two", amount: "200.00", currency: disbursement.EUR},
	}
	workers := make([]disbursement.Worker, 0, count)
	for _, definition := range definitions[:count] {
		amount, err := disbursement.ParseMoney(definition.amount, definition.currency)
		if err != nil {
			t.Fatalf("ParseMoney() error = %v", err)
		}
		worker, err := disbursement.NewWorker(definition.id, definition.name, amount)
		if err != nil {
			t.Fatalf("NewWorker() error = %v", err)
		}
		workers = append(workers, worker)
	}
	return workers
}

func waitForCompletedBatch(t *testing.T, processor *disbursement.Processor, batchID disbursement.BatchID) disbursement.BatchSnapshot {
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
