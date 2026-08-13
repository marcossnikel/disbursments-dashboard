package mockpayment_test

import (
	"context"
	"errors"
	"testing"

	"github.com/marcosnikel/cadana-disbursement-tool/backend/internal/disbursement"
	"github.com/marcosnikel/cadana-disbursement-tool/backend/internal/mockpayment"
)

func TestProviderHonorsCancellationBeforePayment(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := mockpayment.New().Pay(ctx, disbursement.PaymentRequest{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Pay() error = %v, want context.Canceled", err)
	}
}
