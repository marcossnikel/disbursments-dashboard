package httpserver_test

import (
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/marcosnikel/cadana-disbursement-tool/backend/internal/openapi"
)

func TestServerExposesTheAsynchronousBatchLifecycle(t *testing.T) {
	t.Parallel()

	provider := newGatedProvider("w-002")
	testServer := newTestServer(t, provider, 2, slog.New(slog.DiscardHandler))
	batchRequest := openapi.SubmitBatchRequest{
		BatchID: "batch-http-lifecycle", WorkerIDs: []string{"w-001", "w-002"},
	}
	response := postBatch(t, testServer.Client(), testServer.URL, batchRequest)
	if got, want := response.StatusCode, http.StatusAccepted; got != want {
		response.Body.Close()
		t.Fatalf("POST /disbursements status = %d, want %d", got, want)
	}
	var submission openapi.SubmitBatchResponse
	decodeJSON(t, response, &submission)

	waitForPaymentStarts(t, provider.started, 2)
	processing := getBatch(t, testServer.Client(), testServer.URL, submission.BatchID)
	if got, want := processing.Status, openapi.Processing; got != want {
		t.Errorf("initial batch status = %q, want %q", got, want)
	}
	for _, result := range processing.Results {
		if got, want := result.Status, openapi.InFlight; got != want {
			t.Errorf("initial worker %q status = %q, want %q", result.WorkerID, got, want)
		}
	}
	close(provider.release)

	completed := waitForCompletedBatch(t, testServer.Client(), testServer.URL, submission.BatchID)
	resultsByWorker := make(map[string]openapi.DisbursementResult, len(completed.Results))
	for _, result := range completed.Results {
		resultsByWorker[result.WorkerID] = result
	}
	if got, want := resultsByWorker["w-001"].Status, openapi.Success; got != want {
		t.Errorf("w-001 status = %q, want %q", got, want)
	}
	if resultsByWorker["w-001"].ProviderTxnID == nil {
		t.Error("w-001 provider transaction ID = nil, want a value")
	}
	if got, want := resultsByWorker["w-002"].Status, openapi.Failed; got != want {
		t.Errorf("w-002 status = %q, want %q", got, want)
	}
	if resultError := resultsByWorker["w-002"].Error; resultError == nil {
		t.Error("w-002 error = nil, want provider_declined")
	} else if got, want := *resultError, openapi.ProviderDeclined; got != want {
		t.Errorf("w-002 error = %q, want %q", got, want)
	}

	replay := postBatch(t, testServer.Client(), testServer.URL, batchRequest)
	if got, want := replay.StatusCode, http.StatusOK; got != want {
		replay.Body.Close()
		t.Fatalf("replayed POST status = %d, want %d", got, want)
	}
	decodeJSON(t, replay, &openapi.SubmitBatchResponse{})

	conflictResponse := postBatch(t, testServer.Client(), testServer.URL, openapi.SubmitBatchRequest{
		BatchID: batchRequest.BatchID, WorkerIDs: []string{"w-001"},
	})
	if got, want := conflictResponse.StatusCode, http.StatusConflict; got != want {
		conflictResponse.Body.Close()
		t.Fatalf("conflicting POST status = %d, want %d", got, want)
	}
	var conflict openapi.ErrorResponse
	decodeJSON(t, conflictResponse, &conflict)
	if got, want := conflict.Code, openapi.ErrorCodeIdempotencyConflict; got != want {
		t.Errorf("conflict code = %q, want %q", got, want)
	}
	if got, want := conflict.RequestID, conflictResponse.Header.Get("X-Request-ID"); got != want {
		t.Errorf("conflict request ID = %q, want header value %q", got, want)
	}
}

func TestServerCancelsOnlyPaymentsThatHaveNotReachedTheProvider(t *testing.T) {
	t.Parallel()

	provider := newGatedProvider("")
	testServer := newTestServerWithConcurrency(
		t,
		provider,
		3,
		slog.New(slog.DiscardHandler),
		2,
	)
	const batchID = "batch-http-cancel"
	response := postBatch(t, testServer.Client(), testServer.URL, openapi.SubmitBatchRequest{
		BatchID: batchID, WorkerIDs: []string{"w-001", "w-002", "w-003"},
	})
	decodeJSON(t, response, &openapi.SubmitBatchResponse{})
	startedRequests := waitForPaymentStarts(t, provider.started, 2)
	startedWorkers := map[string]bool{}
	for _, request := range startedRequests {
		startedWorkers[string(request.WorkerID)] = true
	}

	cancelResponse := postBatchCancellation(t, testServer.Client(), testServer.URL, batchID)
	if got, want := cancelResponse.StatusCode, http.StatusOK; got != want {
		cancelResponse.Body.Close()
		t.Fatalf("POST cancellation status = %d, want %d", got, want)
	}
	var cancellation openapi.CancelBatchResponse
	decodeJSON(t, cancelResponse, &cancellation)
	if got, want := cancellation.CanceledCount, 1; got != want {
		t.Fatalf("canceled count = %d, want %d", got, want)
	}
	if got, want := cancellation.Batch.Status, openapi.Processing; got != want {
		t.Errorf("batch status after cancellation = %q, want %q", got, want)
	}
	canceledWorkerID := ""
	for _, result := range cancellation.Batch.Results {
		if result.Status == openapi.Canceled {
			canceledWorkerID = result.WorkerID
		}
	}
	if canceledWorkerID == "" {
		t.Fatal("canceled worker ID is empty")
	}
	if startedWorkers[canceledWorkerID] {
		t.Errorf("canceled worker %q had reached the provider", canceledWorkerID)
	}

	repeatedResponse := postBatchCancellation(t, testServer.Client(), testServer.URL, batchID)
	var repeated openapi.CancelBatchResponse
	decodeJSON(t, repeatedResponse, &repeated)
	if got := repeated.CanceledCount; got != 0 {
		t.Errorf("repeated canceled count = %d, want 0", got)
	}

	close(provider.release)
	completed := waitForCompletedBatch(t, testServer.Client(), testServer.URL, batchID)
	for _, result := range completed.Results {
		if result.WorkerID == canceledWorkerID && result.Status != openapi.Canceled {
			t.Errorf("canceled worker final status = %q, want %q", result.Status, openapi.Canceled)
		}
	}
	select {
	case request := <-provider.started:
		t.Errorf("canceled payment reached provider for worker %q", request.WorkerID)
	default:
	}

	missingResponse := postBatchCancellation(t, testServer.Client(), testServer.URL, "batch-missing")
	if got, want := missingResponse.StatusCode, http.StatusNotFound; got != want {
		missingResponse.Body.Close()
		t.Fatalf("missing cancellation status = %d, want %d", got, want)
	}
	var missingError openapi.ErrorResponse
	decodeJSON(t, missingResponse, &missingError)
	if got, want := missingError.Code, openapi.ErrorCodeBatchNotFound; got != want {
		t.Errorf("missing cancellation code = %q, want %q", got, want)
	}
}

func TestServerExplainsEveryUnavailableWorkerWithoutStartingTheBatch(t *testing.T) {
	t.Parallel()

	provider := newGatedProvider("")
	testServer := newTestServer(t, provider, 2, slog.New(slog.DiscardHandler))
	firstResponse := postBatch(t, testServer.Client(), testServer.URL, openapi.SubmitBatchRequest{
		BatchID: "batch-paid", WorkerIDs: []string{"w-001"},
	})
	decodeJSON(t, firstResponse, &openapi.SubmitBatchResponse{})
	waitForPaymentStarts(t, provider.started, 1)
	close(provider.release)
	waitForCompletedBatch(t, testServer.Client(), testServer.URL, "batch-paid")

	blockedResponse := postBatch(t, testServer.Client(), testServer.URL, openapi.SubmitBatchRequest{
		BatchID: "batch-not-started", WorkerIDs: []string{"w-001", "w-002"},
	})
	if got, want := blockedResponse.StatusCode, http.StatusConflict; got != want {
		blockedResponse.Body.Close()
		t.Fatalf("blocked POST status = %d, want %d", got, want)
	}
	var blockedError openapi.ErrorResponse
	decodeJSON(t, blockedResponse, &blockedError)
	if got, want := blockedError.Code, openapi.ErrorCodeWorkersUnavailable; got != want {
		t.Errorf("blocked error code = %q, want %q", got, want)
	}
	if !strings.HasPrefix(blockedError.Message, "No disbursements were started.") {
		t.Errorf("blocked message = %q, want no-work explanation", blockedError.Message)
	}
	if blockedError.UnavailableWorkers == nil || len(*blockedError.UnavailableWorkers) != 1 {
		t.Fatalf("unavailable workers = %v, want one detail", blockedError.UnavailableWorkers)
	}
	if got, want := (*blockedError.UnavailableWorkers)[0].Reason, openapi.AlreadyPaid; got != want {
		t.Errorf("unavailable reason = %q, want %q", got, want)
	}
	if got, want := (*blockedError.UnavailableWorkers)[0].WorkerName, "Maya Thompson"; got != want {
		t.Errorf("unavailable worker name = %q, want %q", got, want)
	}

	missingResponse, err := testServer.Client().Get(testServer.URL + "/disbursements/batch-not-started")
	if err != nil {
		t.Fatalf("GET rejected batch error = %v", err)
	}
	if got, want := missingResponse.StatusCode, http.StatusNotFound; got != want {
		missingResponse.Body.Close()
		t.Fatalf("GET rejected batch status = %d, want %d", got, want)
	}
	var missingError openapi.ErrorResponse
	decodeJSON(t, missingResponse, &missingError)
	if got, want := missingError.Code, openapi.ErrorCodeBatchNotFound; got != want {
		t.Errorf("missing batch code = %q, want %q", got, want)
	}
}

func TestServerRejectsInvalidBatchJSON(t *testing.T) {
	t.Parallel()

	testServer := newTestServer(t, unusedProvider{}, 1, slog.New(slog.DiscardHandler))
	largeBody := `{"batch_id":"batch-large","worker_ids":[` + strings.Repeat(`"w-001",`, 9000) + `"w-001"]}`
	testCases := []struct{ name, body string }{
		{name: "unknown field", body: `{"batch_id":"batch-unknown","worker_ids":["w-001"],"unexpected":true}`},
		{name: "multiple objects", body: `{"batch_id":"batch-one","worker_ids":["w-001"]}{}`},
		{name: "body above limit", body: largeBody},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			response := postRawBatch(t, testServer.Client(), testServer.URL, testCase.body)
			if got, want := response.StatusCode, http.StatusBadRequest; got != want {
				response.Body.Close()
				t.Fatalf("POST status = %d, want %d", got, want)
			}
			var responseError openapi.ErrorResponse
			decodeJSON(t, response, &responseError)
			if got, want := responseError.Code, openapi.ErrorCodeInvalidRequest; got != want {
				t.Errorf("error code = %q, want %q", got, want)
			}
		})
	}
}

func TestServerRejectsInvalidBatchSubmissions(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		request openapi.SubmitBatchRequest
	}{
		{
			name:    "missing batch ID",
			request: openapi.SubmitBatchRequest{WorkerIDs: []string{"w-001"}},
		},
		{
			name:    "missing workers",
			request: openapi.SubmitBatchRequest{BatchID: "batch-invalid"},
		},
		{
			name: "blank worker ID",
			request: openapi.SubmitBatchRequest{
				BatchID: "batch-invalid", WorkerIDs: []string{"  "},
			},
		},
		{
			name: "duplicate worker ID",
			request: openapi.SubmitBatchRequest{
				BatchID: "batch-invalid", WorkerIDs: []string{"w-001", "w-001"},
			},
		},
		{
			name: "unknown worker ID",
			request: openapi.SubmitBatchRequest{
				BatchID: "batch-invalid", WorkerIDs: []string{"w-999"},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			provider := newGatedProvider("")
			testServer := newTestServer(t, provider, 1, slog.New(slog.DiscardHandler))
			response := postBatch(t, testServer.Client(), testServer.URL, testCase.request)
			if got, want := response.StatusCode, http.StatusBadRequest; got != want {
				response.Body.Close()
				t.Fatalf("POST /disbursements status = %d, want %d", got, want)
			}
			var responseError openapi.ErrorResponse
			decodeJSON(t, response, &responseError)
			if got, want := responseError.Code, openapi.ErrorCodeInvalidRequest; got != want {
				t.Errorf("POST /disbursements error code = %q, want %q", got, want)
			}
			if got, want := responseError.RequestID, response.Header.Get("X-Request-ID"); got != want {
				t.Errorf("POST /disbursements request ID = %q, want %q", got, want)
			}
			select {
			case request := <-provider.started:
				t.Errorf("provider received unexpected request for worker %q", request.WorkerID)
			default:
			}
		})
	}
}
