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
	minimumLatency     = 50 * time.Millisecond
	maximumLatency     = 200 * time.Millisecond
	unknownOutcomeRate = 0.10
	failureRate        = 0.30
)

// Provider simulates the unreliable downstream payment provider from the exercise.
type Provider struct{}

func New() *Provider {
	return &Provider{}
}

func (p *Provider) Pay(
	ctx context.Context,
	_ disbursement.PaymentRequest,
) (disbursement.PaymentResult, error) {
	latencyRange := maximumLatency - minimumLatency
	latency := minimumLatency + time.Duration(mathrand.Int64N(int64(latencyRange)+1))
	timer := time.NewTimer(latency)
	defer timer.Stop()

	select {
	case <-timer.C:
	case <-ctx.Done():
		return disbursement.PaymentResult{}, ctx.Err()
	}

	outcome := mathrand.Float64()
	if outcome < unknownOutcomeRate {
		return disbursement.PaymentResult{}, &disbursement.ProviderFailure{
			Code:           disbursement.ProviderTimeout,
			Message:        "the provider timed out before confirming the payment",
			OutcomeUnknown: true,
		}
	}
	if outcome < failureRate {
		return disbursement.PaymentResult{}, &disbursement.ProviderFailure{
			Code:    disbursement.ProviderDeclined,
			Message: "the provider declined the payment",
		}
	}

	return disbursement.PaymentResult{
		ProviderTransactionID: disbursement.ProviderTransactionID(
			"ptx-" + strings.ToLower(cryptorand.Text()),
		),
	}, nil
}
