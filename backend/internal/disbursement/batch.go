package disbursement

import (
	"fmt"
	"strings"
)

type BatchID string

type BatchStatus string

const (
	BatchProcessing BatchStatus = "processing"
	BatchCompleted  BatchStatus = "completed"
)

type DisbursementStatus string

const (
	StatusPending DisbursementStatus = "pending"
	StatusSuccess DisbursementStatus = "success"
	StatusFailed  DisbursementStatus = "failed"
)

type DisbursementResult struct {
	DisbursementID        DisbursementID
	Worker                Worker
	Status                DisbursementStatus
	ProviderTransactionID ProviderTransactionID
	ErrorCode             ProviderErrorCode
	ErrorMessage          string
}

type BatchSnapshot struct {
	BatchID BatchID
	Status  BatchStatus
	Results []DisbursementResult
}

type Submission struct {
	BatchID BatchID
	Created bool
}

type UnavailableReason string

const (
	AlreadyPending UnavailableReason = "already_pending"
	AlreadyPaid    UnavailableReason = "already_paid"
)

type UnavailableWorker struct {
	Worker         Worker
	Reason         UnavailableReason
	BatchID        BatchID
	DisbursementID DisbursementID
}

type IdempotencyConflictError struct {
	BatchID BatchID
}

func (e *IdempotencyConflictError) Error() string {
	return fmt.Sprintf("batch ID %q was already used with a different worker set", e.BatchID)
}

type WorkersUnavailableError struct {
	Workers []UnavailableWorker
}

func (e *WorkersUnavailableError) Error() string {
	workerIDs := make([]string, 0, len(e.Workers))
	for _, unavailableWorker := range e.Workers {
		workerIDs = append(workerIDs, string(unavailableWorker.Worker.ID()))
	}
	return fmt.Sprintf("workers are unavailable: %s", strings.Join(workerIDs, ", "))
}
