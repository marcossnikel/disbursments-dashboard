package disbursement_test

import (
	"errors"
	"testing"

	"github.com/marcosnikel/cadana-disbursement-tool/backend/internal/disbursement"
)

func TestNewWorkerCreatesAValidPaymentObligation(t *testing.T) {
	t.Parallel()

	amount, err := disbursement.ParseMoney("100.25", disbursement.USD)
	if err != nil {
		t.Fatalf("ParseMoney() error = %v", err)
	}
	worker, err := disbursement.NewWorker("w-001", "Maya Thompson", amount)
	if err != nil {
		t.Fatalf("NewWorker() error = %v", err)
	}

	if got, want := worker.ID(), disbursement.WorkerID("w-001"); got != want {
		t.Errorf("ID() = %q, want %q", got, want)
	}
	if got, want := worker.Name(), "Maya Thompson"; got != want {
		t.Errorf("Name() = %q, want %q", got, want)
	}
	if got, want := worker.Amount(), amount; got != want {
		t.Errorf("Amount() = %v, want %v", got, want)
	}
}

func TestNewWorkerRejectsInvalidPaymentObligations(t *testing.T) {
	t.Parallel()

	validAmount, err := disbursement.ParseMoney("100.25", disbursement.USD)
	if err != nil {
		t.Fatalf("ParseMoney() error = %v", err)
	}
	testCases := []struct {
		name       string
		id         disbursement.WorkerID
		workerName string
		amount     disbursement.Money
	}{
		{name: "missing worker ID", workerName: "Maya Thompson", amount: validAmount},
		{name: "blank worker ID", id: "  ", workerName: "Maya Thompson", amount: validAmount},
		{name: "missing worker name", id: "w-001", amount: validAmount},
		{name: "blank worker name", id: "w-001", workerName: "  ", amount: validAmount},
		{name: "zero amount", id: "w-001", workerName: "Maya Thompson"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := disbursement.NewWorker(testCase.id, testCase.workerName, testCase.amount)
			if !errors.Is(err, disbursement.ErrInvalidWorker) {
				t.Fatalf("NewWorker() error = %v, want ErrInvalidWorker", err)
			}
		})
	}
}
