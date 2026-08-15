// Package disbursement models payment obligations and processes idempotent batches.
package disbursement

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Processor owns worker availability and batch processing state.
type Processor struct {
	provider            PaymentProvider
	providerTimeout     time.Duration
	providerMaxAttempts int
	providerRetryDelay  time.Duration
	logger              *slog.Logger

	mu          sync.RWMutex // protects obligations and batches
	workerOrder []WorkerID
	obligations map[WorkerID]*paymentObligation
	batches     map[BatchID]*storedBatch
	batchOrder  []BatchID
	jobs        sync.WaitGroup
}

var (
	// ErrInvalidProcessor reports an invalid processor dependency or initial state.
	ErrInvalidProcessor = errors.New("invalid processor")
	// ErrInvalidSubmission reports a malformed batch submission.
	ErrInvalidSubmission = errors.New("invalid submission")
	// ErrDemoResetInProgress reports that demo data cannot reset while work is pending.
	ErrDemoResetInProgress = errors.New("cannot reset demo while a batch is processing")
)

// ProcessorConfig contains the dependencies, timeout, and retry policy used by a Processor.
type ProcessorConfig struct {
	Provider            PaymentProvider
	ProviderTimeout     time.Duration
	ProviderMaxAttempts int
	ProviderRetryDelay  time.Duration
	Logger              *slog.Logger
}

// NewProcessor creates a processor for a fixed set of payment obligations.
func NewProcessor(
	workers []Worker,
	config ProcessorConfig,
) (*Processor, error) {
	if err := validateProcessorConfiguration(workers, config); err != nil {
		return nil, err
	}

	logger := config.Logger
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}

	workerOrder, obligations, err := indexPaymentObligations(workers)
	if err != nil {
		return nil, err
	}

	return &Processor{
		provider:            config.Provider,
		providerTimeout:     config.ProviderTimeout,
		providerMaxAttempts: config.ProviderMaxAttempts,
		providerRetryDelay:  config.ProviderRetryDelay,
		logger:              logger,
		workerOrder:         workerOrder,
		obligations:         obligations,
		batches:             make(map[BatchID]*storedBatch),
	}, nil
}

// Wait blocks until every accepted payment job finishes. Callers must stop
// submissions before calling Wait.
func (p *Processor) Wait() {
	p.jobs.Wait()
}

func validateProcessorConfiguration(
	workers []Worker,
	config ProcessorConfig,
) error {
	if len(workers) == 0 {
		return fmt.Errorf("%w: at least one worker is required", ErrInvalidProcessor)
	}
	if config.Provider == nil {
		return fmt.Errorf("%w: payment provider is required", ErrInvalidProcessor)
	}
	if config.ProviderTimeout <= 0 {
		return fmt.Errorf("%w: provider timeout must be positive", ErrInvalidProcessor)
	}
	if config.ProviderMaxAttempts <= 0 {
		return fmt.Errorf("%w: provider max attempts must be positive", ErrInvalidProcessor)
	}
	if config.ProviderRetryDelay < 0 {
		return fmt.Errorf("%w: provider retry delay cannot be negative", ErrInvalidProcessor)
	}
	return nil
}
