package disbursement

import (
	"context"
	"fmt"
)

type DisbursementID string
type ProviderTransactionID string

type PaymentRequest struct {
	DisbursementID DisbursementID
	WorkerID       WorkerID
	Amount         Money
}

type PaymentResult struct {
	ProviderTransactionID ProviderTransactionID
}

type PaymentProvider interface {
	Pay(ctx context.Context, request PaymentRequest) (PaymentResult, error)
}

type ProviderErrorCode string

const (
	ProviderDeclined ProviderErrorCode = "provider_declined"
	ProviderError    ProviderErrorCode = "provider_error"
	ProviderTimeout  ProviderErrorCode = "provider_timeout"
)

// ProviderFailure classifies an error returned by the exercise's mock provider.
type ProviderFailure struct {
	Code    ProviderErrorCode
	Message string
}

func (e *ProviderFailure) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}
