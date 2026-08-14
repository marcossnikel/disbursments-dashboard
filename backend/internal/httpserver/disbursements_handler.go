package httpserver

import (
	"context"
	"errors"
	"net/http"

	"github.com/marcosnikel/cadana-disbursement-tool/backend/internal/disbursement"
	"github.com/marcosnikel/cadana-disbursement-tool/backend/internal/openapi"
)

func (s *Server) handleSubmitDisbursements(responseWriter http.ResponseWriter, request *http.Request) {
	var requestBody openapi.SubmitBatchRequest
	if err := decodeJSONBody(responseWriter, request, &requestBody); err != nil {
		s.writeError(
			responseWriter,
			request,
			http.StatusBadRequest,
			openapi.ErrorCodeInvalidRequest,
			"The request body is invalid: "+err.Error(),
		)
		return
	}

	workerIDs := make([]disbursement.WorkerID, 0, len(requestBody.WorkerIDs))
	for _, workerID := range requestBody.WorkerIDs {
		workerIDs = append(workerIDs, disbursement.WorkerID(workerID))
	}
	// The accepted batch outlives this HTTP request. Each provider call adds its own timeout.
	processingContext := context.WithoutCancel(request.Context())
	submission, err := s.processor.Submit(
		processingContext,
		disbursement.BatchID(requestBody.BatchID),
		workerIDs,
	)
	if err != nil {
		s.writeSubmissionError(responseWriter, request, err)
		return
	}

	statusCode := http.StatusAccepted
	if !submission.Created {
		statusCode = http.StatusOK
	}
	s.logger.InfoContext(
		request.Context(),
		"disbursement submission handled",
		"request_id", requestIDFrom(request),
		"batch_id", submission.BatchID,
		"created", submission.Created,
	)
	s.writeJSON(responseWriter, statusCode, openapi.SubmitBatchResponse{
		BatchID: string(submission.BatchID),
	})
}

func (s *Server) handleGetDisbursementBatch(responseWriter http.ResponseWriter, request *http.Request) {
	snapshot, found := s.processor.Batch(disbursement.BatchID(request.PathValue("batch_id")))
	if !found {
		s.writeError(
			responseWriter,
			request,
			http.StatusNotFound,
			openapi.ErrorCodeBatchNotFound,
			"The requested batch was not found.",
		)
		return
	}

	s.writeJSON(responseWriter, http.StatusOK, mapBatchSnapshot(snapshot))
}

func (s *Server) writeSubmissionError(
	responseWriter http.ResponseWriter,
	request *http.Request,
	err error,
) {
	var idempotencyConflict *disbursement.IdempotencyConflictError
	var workersUnavailable *disbursement.WorkersUnavailableError
	switch {
	case errors.Is(err, disbursement.ErrInvalidSubmission):
		s.writeError(
			responseWriter,
			request,
			http.StatusBadRequest,
			openapi.ErrorCodeInvalidRequest,
			err.Error(),
		)
	case errors.As(err, &idempotencyConflict):
		s.writeError(
			responseWriter,
			request,
			http.StatusConflict,
			openapi.ErrorCodeIdempotencyConflict,
			"This batch ID is already associated with a different worker selection. No disbursements were started.",
		)
	case errors.As(err, &workersUnavailable):
		details := make([]openapi.UnavailableWorker, 0, len(workersUnavailable.Workers))
		for _, unavailableWorker := range workersUnavailable.Workers {
			details = append(details, openapi.UnavailableWorker{
				BatchID:        string(unavailableWorker.BatchID),
				DisbursementID: string(unavailableWorker.DisbursementID),
				Reason:         openapi.UnavailableReason(unavailableWorker.Reason),
				WorkerID:       string(unavailableWorker.Worker.ID()),
				WorkerName:     unavailableWorker.Worker.Name(),
			})
		}
		errorResponse := newErrorResponse(
			request,
			openapi.ErrorCodeWorkersUnavailable,
			"No disbursements were started. One or more workers are no longer available.",
		)
		errorResponse.UnavailableWorkers = &details
		s.writeJSON(responseWriter, http.StatusConflict, errorResponse)
	default:
		s.logger.ErrorContext(
			request.Context(),
			"submit disbursement batch",
			"request_id", requestIDFrom(request),
			"error", err,
		)
		s.writeError(
			responseWriter,
			request,
			http.StatusInternalServerError,
			openapi.ErrorCodeInternalError,
			"The batch could not be submitted.",
		)
	}
}

func mapBatchSnapshot(snapshot disbursement.BatchSnapshot) openapi.BatchSnapshot {
	results := make([]openapi.DisbursementResult, 0, len(snapshot.Results))
	for _, result := range snapshot.Results {
		mappedResult := openapi.DisbursementResult{
			Amount:         result.Worker.Amount().String(),
			Currency:       openapi.Currency(result.Worker.Amount().Currency()),
			DisbursementID: string(result.DisbursementID),
			Status:         openapi.DisbursementStatus(result.Status),
			WorkerID:       string(result.Worker.ID()),
			WorkerName:     result.Worker.Name(),
		}
		if result.ProviderTransactionID != "" {
			providerTransactionID := string(result.ProviderTransactionID)
			mappedResult.ProviderTxnID = &providerTransactionID
		}
		if result.ErrorCode != "" {
			errorCode := openapi.ProviderErrorCode(result.ErrorCode)
			mappedResult.Error = &errorCode
		}
		if result.ErrorMessage != "" {
			errorMessage := result.ErrorMessage
			mappedResult.ErrorMessage = &errorMessage
		}
		results = append(results, mappedResult)
	}

	return openapi.BatchSnapshot{
		BatchID: string(snapshot.BatchID),
		Status:  openapi.BatchStatus(snapshot.Status),
		Results: results,
	}
}
