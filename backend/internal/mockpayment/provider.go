// Package mockpayment simulates the unreliable provider required by the exercise.
package mockpayment

import (
	"context"
	cryptorand "crypto/rand"
	mathrand "math/rand/v2"
	"strings"
	"time"

	"github.com/marcosnikel/cadana-disbursement-tool/backend/internal/disbursement"
)

const (
	minimumLatency = 50 * time.Millisecond
	maximumLatency = 200 * time.Millisecond
	failureRate    = 0.30
)

// Provider simulates the unreliable downstream payment provider from the exercise.
type Provider struct{}

// New returns a stateless simulated payment provider.
func New() Provider {
	return Provider{}
}

// Pay waits for randomized latency, then succeeds or fails at the configured rate.
func (Provider) Pay(
	ctx context.Context,
	_ disbursement.PaymentRequest,
) (disbursement.PaymentResult, error) {
	if err := waitForLatency(ctx, generateLatency()); err != nil {
		return disbursement.PaymentResult{}, err
	}

	if mathrand.Float64() < failureRate {
		return disbursement.PaymentResult{}, &disbursement.ProviderFailure{
			Code:    disbursement.ProviderTimeout,
			Message: "the mock provider timed out",
		}
	}

	return disbursement.PaymentResult{
		ProviderTransactionID: disbursement.ProviderTransactionID(
			"ptx-" + strings.ToLower(cryptorand.Text()),
		),
	}, nil
}

func generateLatency() time.Duration {
	latencyRange := maximumLatency - minimumLatency
	return minimumLatency + time.Duration(mathrand.Int64N(int64(latencyRange)+1))
}

func waitForLatency(ctx context.Context, latency time.Duration) error {
	timer := time.NewTimer(latency)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
