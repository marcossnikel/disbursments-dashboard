package disbursement

import (
	"context"
	"fmt"
)

// DisbursementID uniquely identifies one internal payment attempt.
type DisbursementID string

// ProviderTransactionID identifies a successful payment at the provider.
type ProviderTransactionID string

// PaymentRequest contains the values required by a payment provider.
type PaymentRequest struct {
	DisbursementID DisbursementID
	WorkerID       WorkerID
	Amount         Money
}

// PaymentResult contains the provider's successful payment identifier.
type PaymentResult struct {
	ProviderTransactionID ProviderTransactionID
}

// PaymentProvider executes one payment attempt.
type PaymentProvider interface {
	Pay(ctx context.Context, request PaymentRequest) (PaymentResult, error)
}

// ProviderErrorCode classifies a terminal payment failure.
type ProviderErrorCode string

const (
	// ProviderDeclined means the provider rejected the payment.
	ProviderDeclined ProviderErrorCode = "provider_declined"
	// ProviderError means the provider failed without a more specific classification.
	ProviderError ProviderErrorCode = "provider_error"
	// ProviderTimeout means the provider did not complete within the deadline.
	ProviderTimeout ProviderErrorCode = "provider_timeout"
)

// ProviderFailure classifies an error returned by the exercise's mock provider.
type ProviderFailure struct {
	Code    ProviderErrorCode
	Message string
}

func (e *ProviderFailure) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}
