package httpserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/marcosnikel/cadana-disbursement-tool/backend/internal/disbursement"
	"github.com/marcosnikel/cadana-disbursement-tool/backend/internal/httpserver"
	"github.com/marcosnikel/cadana-disbursement-tool/backend/internal/openapi"
)

func TestServerListsAvailableWorkersWithARequestID(t *testing.T) {
	t.Parallel()

	workers, err := disbursement.SeedWorkers()
	if err != nil {
		t.Fatalf("SeedWorkers() error = %v", err)
	}
	processor, err := disbursement.NewProcessor(
		context.Background(),
		workers,
		disbursement.ProcessorConfig{
			Provider:        unusedProvider{},
			ProviderTimeout: time.Second,
		},
	)
	if err != nil {
		t.Fatalf("NewProcessor() error = %v", err)
	}
	var logOutput bytes.Buffer
	handler, err := httpserver.New(
		processor,
		slog.New(slog.NewJSONHandler(&logOutput, nil)),
		httpserver.Config{AllowedOrigin: "http://localhost:5173"},
	)
	if err != nil {
		t.Fatalf("httpserver.New() error = %v", err)
	}
	testServer := httptest.NewServer(handler)
	t.Cleanup(testServer.Close)

	response, err := testServer.Client().Get(testServer.URL + "/workers")
	if err != nil {
		t.Fatalf("GET /workers error = %v", err)
	}
	defer response.Body.Close()

	if got, want := response.StatusCode, http.StatusOK; got != want {
		t.Fatalf("GET /workers status = %d, want %d", got, want)
	}
	requestID := response.Header.Get("X-Request-ID")
	if !strings.HasPrefix(requestID, "req-") {
		t.Errorf("X-Request-ID = %q, want req- prefix", requestID)
	}
	if got, want := response.Header.Get("Content-Type"), "application/json"; got != want {
		t.Errorf("Content-Type = %q, want %q", got, want)
	}

	var responseWorkers []openapi.Worker
	if err := json.NewDecoder(response.Body).Decode(&responseWorkers); err != nil {
		t.Fatalf("decode workers: %v", err)
	}
	if got, want := len(responseWorkers), 10; got != want {
		t.Fatalf("worker count = %d, want %d", got, want)
	}
	if got, want := responseWorkers[0].Amount, "1500.50"; got != want {
		t.Errorf("first worker amount = %q, want %q", got, want)
	}
	if got, want := responseWorkers[0].Currency, openapi.USD; got != want {
		t.Errorf("first worker currency = %q, want %q", got, want)
	}

	logLine := logOutput.String()
	for _, expected := range []string{
		`"msg":"HTTP request completed"`,
		`"request_id":"` + requestID + `"`,
		`"method":"GET"`,
		`"path":"/workers"`,
		`"status":200`,
	} {
		if !strings.Contains(logLine, expected) {
			t.Errorf("access log = %q, want to contain %q", logLine, expected)
		}
	}
}

func TestServerExposesTheAsynchronousBatchLifecycle(t *testing.T) {
	t.Parallel()

	workers, err := disbursement.SeedWorkers()
	if err != nil {
		t.Fatalf("SeedWorkers() error = %v", err)
	}
	provider := newGatedProvider("w-002")
	processor, err := disbursement.NewProcessor(
		context.Background(),
		workers[:2],
		disbursement.ProcessorConfig{
			Provider:        provider,
			ProviderTimeout: time.Second,
		},
	)
	if err != nil {
		t.Fatalf("NewProcessor() error = %v", err)
	}
	handler, err := httpserver.New(
		processor,
		slog.New(slog.DiscardHandler),
		httpserver.Config{AllowedOrigin: "http://localhost:5173"},
	)
	if err != nil {
		t.Fatalf("httpserver.New() error = %v", err)
	}
	testServer := httptest.NewServer(handler)
	t.Cleanup(testServer.Close)

	batchRequest := openapi.SubmitBatchRequest{
		BatchId:   "batch-http-lifecycle",
		WorkerIds: []string{"w-001", "w-002"},
	}
	response := postBatch(t, testServer.Client(), testServer.URL, batchRequest)
	if got, want := response.StatusCode, http.StatusAccepted; got != want {
		response.Body.Close()
		t.Fatalf("POST /disbursements status = %d, want %d", got, want)
	}
	var submission openapi.SubmitBatchResponse
	decodeJSON(t, response, &submission)
	if got, want := submission.BatchId, batchRequest.BatchId; got != want {
		t.Errorf("submitted batch ID = %q, want %q", got, want)
	}

	pending := getBatch(t, testServer.Client(), testServer.URL, submission.BatchId)
	if got, want := pending.Status, openapi.Processing; got != want {
		t.Errorf("initial batch status = %q, want %q", got, want)
	}
	for _, result := range pending.Results {
		if got, want := result.Status, openapi.DisbursementStatusPending; got != want {
			t.Errorf("initial worker %q status = %q, want %q", result.WorkerId, got, want)
		}
	}

	for range 2 {
		select {
		case <-provider.started:
		case <-time.After(time.Second):
			t.Fatal("provider calls did not start")
		}
	}
	close(provider.release)

	completed := waitForCompletedBatch(t, testServer.Client(), testServer.URL, submission.BatchId)
	resultsByWorker := make(map[string]openapi.DisbursementResult, len(completed.Results))
	for _, result := range completed.Results {
		resultsByWorker[result.WorkerId] = result
	}
	if got, want := resultsByWorker["w-001"].Status, openapi.DisbursementStatusSuccess; got != want {
		t.Errorf("w-001 status = %q, want %q", got, want)
	}
	if transactionID := resultsByWorker["w-001"].ProviderTransactionId; transactionID == nil {
		t.Error("w-001 provider transaction ID = nil, want a value")
	}
	if got, want := resultsByWorker["w-002"].Status, openapi.DisbursementStatusFailed; got != want {
		t.Errorf("w-002 status = %q, want %q", got, want)
	}

	replayResponse := postBatch(t, testServer.Client(), testServer.URL, batchRequest)
	if got, want := replayResponse.StatusCode, http.StatusOK; got != want {
		replayResponse.Body.Close()
		t.Fatalf("replayed POST status = %d, want %d", got, want)
	}
	decodeJSON(t, replayResponse, &openapi.SubmitBatchResponse{})

	conflictResponse := postBatch(t, testServer.Client(), testServer.URL, openapi.SubmitBatchRequest{
		BatchId:   batchRequest.BatchId,
		WorkerIds: []string{"w-001"},
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
	if got, want := conflict.RequestId, conflictResponse.Header.Get("X-Request-ID"); got != want {
		t.Errorf("conflict request ID = %q, want header value %q", got, want)
	}
}

func TestServerExplainsEveryUnavailableWorkerWithoutStartingTheBatch(t *testing.T) {
	t.Parallel()

	workers, err := disbursement.SeedWorkers()
	if err != nil {
		t.Fatalf("SeedWorkers() error = %v", err)
	}
	provider := newGatedProvider("")
	processor, err := disbursement.NewProcessor(
		context.Background(),
		workers[:2],
		disbursement.ProcessorConfig{
			Provider:        provider,
			ProviderTimeout: time.Second,
		},
	)
	if err != nil {
		t.Fatalf("NewProcessor() error = %v", err)
	}
	handler, err := httpserver.New(
		processor,
		slog.New(slog.DiscardHandler),
		httpserver.Config{AllowedOrigin: "http://localhost:5173"},
	)
	if err != nil {
		t.Fatalf("httpserver.New() error = %v", err)
	}
	testServer := httptest.NewServer(handler)
	t.Cleanup(testServer.Close)

	firstResponse := postBatch(t, testServer.Client(), testServer.URL, openapi.SubmitBatchRequest{
		BatchId:   "batch-paid",
		WorkerIds: []string{"w-001"},
	})
	decodeJSON(t, firstResponse, &openapi.SubmitBatchResponse{})
	select {
	case <-provider.started:
	case <-time.After(time.Second):
		t.Fatal("first provider payment did not start")
	}
	close(provider.release)
	waitForCompletedBatch(t, testServer.Client(), testServer.URL, "batch-paid")

	blockedResponse := postBatch(t, testServer.Client(), testServer.URL, openapi.SubmitBatchRequest{
		BatchId:   "batch-not-started",
		WorkerIds: []string{"w-001", "w-002"},
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
	if blockedError.UnavailableWorkers == nil {
		t.Fatal("unavailable workers = nil, want details")
	}
	if got, want := len(*blockedError.UnavailableWorkers), 1; got != want {
		t.Fatalf("unavailable worker count = %d, want %d", got, want)
	}
	detail := (*blockedError.UnavailableWorkers)[0]
	if got, want := detail.WorkerName, "Ada Lovelace"; got != want {
		t.Errorf("unavailable worker name = %q, want %q", got, want)
	}
	if got, want := detail.Reason, openapi.UnavailableReasonAlreadyPaid; got != want {
		t.Errorf("unavailable reason = %q, want %q", got, want)
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

type unusedProvider struct{}

func (unusedProvider) Pay(
	context.Context,
	disbursement.PaymentRequest,
) (disbursement.PaymentResult, error) {
	return disbursement.PaymentResult{}, io.ErrUnexpectedEOF
}

type gatedProvider struct {
	started       chan disbursement.PaymentRequest
	release       chan struct{}
	failingWorker disbursement.WorkerID
}

func newGatedProvider(failingWorker disbursement.WorkerID) *gatedProvider {
	return &gatedProvider{
		started:       make(chan disbursement.PaymentRequest, 2),
		release:       make(chan struct{}),
		failingWorker: failingWorker,
	}
}

func (p *gatedProvider) Pay(
	ctx context.Context,
	request disbursement.PaymentRequest,
) (disbursement.PaymentResult, error) {
	p.started <- request
	select {
	case <-p.release:
	case <-ctx.Done():
		return disbursement.PaymentResult{}, ctx.Err()
	}

	if request.WorkerID == p.failingWorker {
		return disbursement.PaymentResult{}, &disbursement.ProviderFailure{
			Code:    disbursement.ProviderDeclined,
			Message: "the provider declined this payment",
		}
	}
	return disbursement.PaymentResult{
		ProviderTransactionID: disbursement.ProviderTransactionID("ptx-" + request.WorkerID),
	}, nil
}

func postBatch(
	t *testing.T,
	client *http.Client,
	serverURL string,
	request openapi.SubmitBatchRequest,
) *http.Response {
	t.Helper()

	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(request); err != nil {
		t.Fatalf("encode batch request: %v", err)
	}
	response, err := client.Post(serverURL+"/disbursements", "application/json", &body)
	if err != nil {
		t.Fatalf("POST /disbursements error = %v", err)
	}
	return response
}

func getBatch(
	t *testing.T,
	client *http.Client,
	serverURL string,
	batchID string,
) openapi.BatchSnapshot {
	t.Helper()

	response, err := client.Get(serverURL + "/disbursements/" + batchID)
	if err != nil {
		t.Fatalf("GET batch error = %v", err)
	}
	if got, want := response.StatusCode, http.StatusOK; got != want {
		response.Body.Close()
		t.Fatalf("GET batch status = %d, want %d", got, want)
	}
	var snapshot openapi.BatchSnapshot
	decodeJSON(t, response, &snapshot)
	return snapshot
}

func waitForCompletedBatch(
	t *testing.T,
	client *http.Client,
	serverURL string,
	batchID string,
) openapi.BatchSnapshot {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		snapshot := getBatch(t, client, serverURL, batchID)
		if snapshot.Status == openapi.Completed {
			return snapshot
		}
		time.Sleep(time.Millisecond)
	}

	t.Fatalf("batch %q did not complete before the deadline", batchID)
	return openapi.BatchSnapshot{}
}

func decodeJSON(t *testing.T, response *http.Response, destination any) {
	t.Helper()
	defer response.Body.Close()

	if err := json.NewDecoder(response.Body).Decode(destination); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}
