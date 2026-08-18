package disbursement_test

import (
	"errors"
	"testing"
	"time"

	"github.com/marcosnikel/cadana-disbursement-tool/backend/internal/disbursement"
)

func TestNewProcessorPreservesConfiguredWorkerOrder(t *testing.T) {
	t.Parallel()

	processor := newTestProcessor(t, timeoutProvider{}, 2)
	availableWorkers := processor.AvailableWorkers()
	if got, want := len(availableWorkers), 2; got != want {
		t.Fatalf("len(AvailableWorkers()) = %d, want %d", got, want)
	}
	for index, want := range []disbursement.WorkerID{"w-001", "w-002"} {
		if got := availableWorkers[index].ID(); got != want {
			t.Errorf("AvailableWorkers()[%d].ID() = %q, want %q", index, got, want)
		}
	}
}

func TestNewProcessorRejectsInvalidConfiguration(t *testing.T) {
	t.Parallel()

	validWorkers := testWorkers(t, 2)
	duplicateWorkers := append([]disbursement.Worker(nil), validWorkers...)
	duplicateWorkers[1] = duplicateWorkers[0]
	validConfig := disbursement.ProcessorConfig{
		Provider:        timeoutProvider{},
		ProviderTimeout: time.Second,
	}
	testCases := []struct {
		name    string
		workers []disbursement.Worker
		config  disbursement.ProcessorConfig
	}{
		{name: "missing workers", config: validConfig},
		{name: "missing provider", workers: validWorkers, config: disbursement.ProcessorConfig{ProviderTimeout: time.Second}},
		{name: "zero provider timeout", workers: validWorkers, config: disbursement.ProcessorConfig{Provider: timeoutProvider{}}},
		{name: "negative provider timeout", workers: validWorkers, config: disbursement.ProcessorConfig{Provider: timeoutProvider{}, ProviderTimeout: -time.Second}},
		{name: "negative provider concurrency", workers: validWorkers, config: disbursement.ProcessorConfig{
			Provider: timeoutProvider{}, ProviderTimeout: time.Second, ProviderMaxConcurrentCalls: -1,
		}},
		{name: "duplicate worker ID", workers: duplicateWorkers, config: validConfig},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := disbursement.NewProcessor(testCase.workers, testCase.config)
			if !errors.Is(err, disbursement.ErrInvalidProcessor) {
				t.Fatalf("NewProcessor() error = %v, want ErrInvalidProcessor", err)
			}
		})
	}
}
