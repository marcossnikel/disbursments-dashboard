package httpserver

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/marcosnikel/cadana-disbursement-tool/backend/internal/disbursement"
	"github.com/marcosnikel/cadana-disbursement-tool/backend/internal/openapi"
)

type Config struct {
	AllowedOrigin string
}

type Server struct {
	processor     *disbursement.Processor
	logger        *slog.Logger
	allowedOrigin string
	mux           *http.ServeMux
}

var ErrInvalidServer = errors.New("invalid HTTP server")

type requestIDContextKey struct{}

type responseRecorder struct {
	http.ResponseWriter
	statusCode int
}

func New(processor *disbursement.Processor, logger *slog.Logger, config Config) (*Server, error) {
	if processor == nil {
		return nil, fmt.Errorf("%w: processor is required", ErrInvalidServer)
	}
	if logger == nil {
		return nil, fmt.Errorf("%w: logger is required", ErrInvalidServer)
	}
	if strings.TrimSpace(config.AllowedOrigin) == "" {
		return nil, fmt.Errorf("%w: allowed origin is required", ErrInvalidServer)
	}

	server := &Server{
		processor:     processor,
		logger:        logger,
		allowedOrigin: config.AllowedOrigin,
		mux:           http.NewServeMux(),
	}
	server.mux.HandleFunc("GET /workers", server.listWorkers)
	server.mux.HandleFunc("POST /disbursements", server.submitDisbursements)
	server.mux.HandleFunc("GET /disbursements/{batch_id}", server.getDisbursementBatch)
	server.mux.HandleFunc("POST /demo/reset", server.resetDemo)

	return server, nil
}

func (s *Server) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	requestStartedAt := time.Now()
	requestID := "req-" + strings.ToLower(rand.Text())
	request = request.WithContext(context.WithValue(request.Context(), requestIDContextKey{}, requestID))
	recordedResponse := &responseRecorder{ResponseWriter: responseWriter}
	defer s.logRequest(request, recordedResponse, requestStartedAt)

	recordedResponse.Header().Set("X-Request-ID", requestID)

	if request.Header.Get("Origin") == s.allowedOrigin {
		recordedResponse.Header().Set("Access-Control-Allow-Origin", s.allowedOrigin)
		recordedResponse.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		recordedResponse.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		recordedResponse.Header().Add("Vary", "Origin")
		if request.Method == http.MethodOptions {
			recordedResponse.WriteHeader(http.StatusNoContent)
			return
		}
	}

	s.mux.ServeHTTP(recordedResponse, request)
}

func (s *Server) listWorkers(responseWriter http.ResponseWriter, _ *http.Request) {
	workers := s.processor.AvailableWorkers()
	response := make([]openapi.Worker, 0, len(workers))
	for _, worker := range workers {
		response = append(response, openapi.Worker{
			Id:       string(worker.ID()),
			Name:     worker.Name(),
			Amount:   worker.Amount().String(),
			Currency: openapi.Currency(worker.Amount().Currency()),
		})
	}

	s.writeJSON(responseWriter, http.StatusOK, response)
}

func (s *Server) submitDisbursements(responseWriter http.ResponseWriter, request *http.Request) {
	var requestBody openapi.SubmitBatchRequest
	if err := decodeJSONBody(responseWriter, request, &requestBody); err != nil {
		s.writeError(
			responseWriter,
			request,
			http.StatusBadRequest,
			openapi.ErrorCodeInvalidRequest,
			"The request body is invalid: "+err.Error(),
			nil,
		)
		return
	}

	workerIDs := make([]disbursement.WorkerID, 0, len(requestBody.WorkerIds))
	for _, workerID := range requestBody.WorkerIds {
		workerIDs = append(workerIDs, disbursement.WorkerID(workerID))
	}
	submission, err := s.processor.Submit(disbursement.BatchID(requestBody.BatchId), workerIDs)
	if err != nil {
		s.writeSubmissionError(responseWriter, request, err)
		return
	}

	statusCode := http.StatusAccepted
	if !submission.Created {
		statusCode = http.StatusOK
	}
	s.logger.Info(
		"disbursement submission handled",
		"request_id", requestIDFrom(request),
		"batch_id", submission.BatchID,
		"created", submission.Created,
	)
	s.writeJSON(responseWriter, statusCode, openapi.SubmitBatchResponse{
		BatchId: string(submission.BatchID),
	})
}

func (s *Server) logRequest(
	request *http.Request,
	response *responseRecorder,
	startedAt time.Time,
) {
	statusCode := response.statusCode
	if statusCode == 0 {
		statusCode = http.StatusOK
	}
	level := slog.LevelInfo
	if statusCode >= http.StatusInternalServerError {
		level = slog.LevelError
	} else if statusCode >= http.StatusBadRequest {
		level = slog.LevelWarn
	}

	s.logger.Log(
		request.Context(),
		level,
		"HTTP request completed",
		"request_id", requestIDFrom(request),
		"method", request.Method,
		"path", request.URL.Path,
		"status", statusCode,
		"duration_ms", time.Since(startedAt).Milliseconds(),
	)
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	if r.statusCode != 0 {
		return
	}
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *responseRecorder) Write(body []byte) (int, error) {
	if r.statusCode == 0 {
		r.WriteHeader(http.StatusOK)
	}
	return r.ResponseWriter.Write(body)
}

func (s *Server) getDisbursementBatch(responseWriter http.ResponseWriter, request *http.Request) {
	snapshot, found := s.processor.Batch(disbursement.BatchID(request.PathValue("batch_id")))
	if !found {
		s.writeError(
			responseWriter,
			request,
			http.StatusNotFound,
			openapi.ErrorCodeBatchNotFound,
			"The requested batch was not found.",
			nil,
		)
		return
	}

	s.writeJSON(responseWriter, http.StatusOK, mapBatchSnapshot(snapshot))
}

func (s *Server) resetDemo(responseWriter http.ResponseWriter, request *http.Request) {
	if err := s.processor.ResetDemo(); err != nil {
		if errors.Is(err, disbursement.ErrDemoResetInProgress) {
			s.writeError(
				responseWriter,
				request,
				http.StatusConflict,
				openapi.ErrorCodeDemoResetInProgress,
				"Wait for the active batch to finish before resetting demo data.",
				nil,
			)
			return
		}
		s.logger.Error("reset demo state", "request_id", requestIDFrom(request), "error", err)
		s.writeError(
			responseWriter,
			request,
			http.StatusInternalServerError,
			openapi.ErrorCodeInternalError,
			"The demo data could not be reset.",
			nil,
		)
		return
	}

	responseWriter.WriteHeader(http.StatusNoContent)
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
			nil,
		)
	case errors.As(err, &idempotencyConflict):
		s.writeError(
			responseWriter,
			request,
			http.StatusConflict,
			openapi.ErrorCodeIdempotencyConflict,
			"This batch ID is already associated with a different worker selection. No disbursements were started.",
			nil,
		)
	case errors.As(err, &workersUnavailable):
		details := make([]openapi.UnavailableWorker, 0, len(workersUnavailable.Workers))
		for _, unavailableWorker := range workersUnavailable.Workers {
			details = append(details, openapi.UnavailableWorker{
				BatchId:        string(unavailableWorker.BatchID),
				DisbursementId: string(unavailableWorker.DisbursementID),
				Reason:         openapi.UnavailableReason(unavailableWorker.Reason),
				WorkerId:       string(unavailableWorker.Worker.ID()),
				WorkerName:     unavailableWorker.Worker.Name(),
			})
		}
		s.writeError(
			responseWriter,
			request,
			http.StatusConflict,
			openapi.ErrorCodeWorkersUnavailable,
			"No disbursements were started. One or more workers are no longer available.",
			&details,
		)
	default:
		s.logger.Error(
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
			nil,
		)
	}
}

func (s *Server) writeError(
	responseWriter http.ResponseWriter,
	request *http.Request,
	statusCode int,
	code openapi.ErrorCode,
	message string,
	unavailableWorkers *[]openapi.UnavailableWorker,
) {
	s.writeJSON(responseWriter, statusCode, openapi.ErrorResponse{
		Code:               code,
		Message:            message,
		RequestId:          requestIDFrom(request),
		UnavailableWorkers: unavailableWorkers,
	})
}

func (s *Server) writeJSON(responseWriter http.ResponseWriter, statusCode int, body any) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(statusCode)
	if err := json.NewEncoder(responseWriter).Encode(body); err != nil {
		s.logger.Error("encode JSON response", "error", err)
	}
}

func decodeJSONBody(responseWriter http.ResponseWriter, request *http.Request, destination any) error {
	request.Body = http.MaxBytesReader(responseWriter, request.Body, 64<<10)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain one JSON object")
	}
	return nil
}

func requestIDFrom(request *http.Request) string {
	requestID, _ := request.Context().Value(requestIDContextKey{}).(string)
	return requestID
}

func mapBatchSnapshot(snapshot disbursement.BatchSnapshot) openapi.BatchSnapshot {
	results := make([]openapi.DisbursementResult, 0, len(snapshot.Results))
	for _, result := range snapshot.Results {
		mappedResult := openapi.DisbursementResult{
			Amount:         result.Worker.Amount().String(),
			Currency:       openapi.Currency(result.Worker.Amount().Currency()),
			DisbursementId: string(result.DisbursementID),
			Status:         openapi.DisbursementStatus(result.Status),
			WorkerId:       string(result.Worker.ID()),
			WorkerName:     result.Worker.Name(),
		}
		if result.ProviderTransactionID != "" {
			providerTransactionID := string(result.ProviderTransactionID)
			mappedResult.ProviderTransactionId = &providerTransactionID
		}
		if result.ErrorCode != "" {
			errorCode := openapi.ProviderErrorCode(result.ErrorCode)
			mappedResult.ErrorCode = &errorCode
		}
		if result.ErrorMessage != "" {
			errorMessage := result.ErrorMessage
			mappedResult.ErrorMessage = &errorMessage
		}
		results = append(results, mappedResult)
	}

	return openapi.BatchSnapshot{
		BatchId: string(snapshot.BatchID),
		Status:  openapi.BatchStatus(snapshot.Status),
		Results: results,
	}
}
