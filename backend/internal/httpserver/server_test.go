package httpserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/marcosnikel/cadana-disbursement-tool/backend/internal/demodata"
	"github.com/marcosnikel/cadana-disbursement-tool/backend/internal/disbursement"
	"github.com/marcosnikel/cadana-disbursement-tool/backend/internal/httpserver"
	"github.com/marcosnikel/cadana-disbursement-tool/backend/internal/openapi"
)

type unusedProvider struct{}

func (unusedProvider) Pay(context.Context, disbursement.PaymentRequest) (disbursement.PaymentResult, error) {
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

func (p *gatedProvider) Pay(ctx context.Context, request disbursement.PaymentRequest) (disbursement.PaymentResult, error) {
	p.started <- request
	select {
	case <-p.release:
	case <-ctx.Done():
		return disbursement.PaymentResult{}, ctx.Err()
	}
	if request.WorkerID == p.failingWorker {
		return disbursement.PaymentResult{}, &disbursement.ProviderFailure{
			Code: disbursement.ProviderDeclined, Message: "the provider declined this payment",
		}
	}
	return disbursement.PaymentResult{
		ProviderTransactionID: disbursement.ProviderTransactionID("ptx-" + request.WorkerID),
	}, nil
}

func TestNewServerRejectsInvalidConfiguration(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.DiscardHandler)
	processor := newTestProcessor(t, unusedProvider{}, 1, logger)
	testCases := []struct {
		name      string
		processor *disbursement.Processor
		logger    *slog.Logger
		config    httpserver.Config
	}{
		{
			name:   "missing processor",
			logger: logger,
			config: httpserver.Config{AllowedOrigin: "http://localhost:5173"},
		},
		{
			name:      "missing logger",
			processor: processor,
			config:    httpserver.Config{AllowedOrigin: "http://localhost:5173"},
		},
		{
			name:      "missing allowed origin",
			processor: processor,
			logger:    logger,
		},
		{
			name:      "blank allowed origin",
			processor: processor,
			logger:    logger,
			config:    httpserver.Config{AllowedOrigin: "  "},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := httpserver.New(testCase.processor, testCase.logger, testCase.config)
			if !errors.Is(err, httpserver.ErrInvalidServer) {
				t.Fatalf("httpserver.New() error = %v, want ErrInvalidServer", err)
			}
		})
	}
}

func newTestProcessor(
	t *testing.T,
	provider disbursement.PaymentProvider,
	workerCount int,
	logger *slog.Logger,
) *disbursement.Processor {
	t.Helper()

	workers, err := demodata.Workers()
	if err != nil {
		t.Fatalf("Workers() error = %v", err)
	}
	processor, err := disbursement.NewProcessor(workers[:workerCount], disbursement.ProcessorConfig{
		Provider: provider, ProviderTimeout: time.Second, Logger: logger,
	})
	if err != nil {
		t.Fatalf("NewProcessor() error = %v", err)
	}
	return processor
}

func newTestServer(
	t *testing.T,
	provider disbursement.PaymentProvider,
	workerCount int,
	logger *slog.Logger,
) *httptest.Server {
	t.Helper()

	processor := newTestProcessor(t, provider, workerCount, logger)
	handler, err := httpserver.New(
		processor,
		logger,
		httpserver.Config{AllowedOrigin: "http://localhost:5173"},
	)
	if err != nil {
		t.Fatalf("httpserver.New() error = %v", err)
	}
	testServer := httptest.NewServer(handler)
	t.Cleanup(testServer.Close)
	return testServer
}

func postBatch(t *testing.T, client *http.Client, serverURL string, request openapi.SubmitBatchRequest) *http.Response {
	t.Helper()
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(request); err != nil {
		t.Fatalf("encode batch request: %v", err)
	}
	return postRawBatch(t, client, serverURL, body.String())
}

func postRawBatch(t *testing.T, client *http.Client, serverURL, body string) *http.Response {
	t.Helper()
	request, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		serverURL+"/disbursements",
		strings.NewReader(body),
	)
	if err != nil {
		t.Fatalf("create POST /disbursements request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("POST /disbursements error = %v", err)
	}
	return response
}

func postDemoReset(t *testing.T, client *http.Client, serverURL string) *http.Response {
	t.Helper()
	request, err := http.NewRequestWithContext(context.Background(), http.MethodPost, serverURL+"/demo/reset", nil)
	if err != nil {
		t.Fatalf("create POST /demo/reset request: %v", err)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("POST /demo/reset error = %v", err)
	}
	return response
}

func getBatch(t *testing.T, client *http.Client, serverURL, batchID string) openapi.BatchSnapshot {
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

func waitForCompletedBatch(t *testing.T, client *http.Client, serverURL, batchID string) openapi.BatchSnapshot {
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

func waitForPaymentStarts(
	t *testing.T,
	started <-chan disbursement.PaymentRequest,
	count int,
) []disbursement.PaymentRequest {
	t.Helper()

	requests := make([]disbursement.PaymentRequest, 0, count)
	for range count {
		select {
		case request := <-started:
			requests = append(requests, request)
		case <-time.After(time.Second):
			t.Fatalf("provider calls started = %d, want %d", len(requests), count)
		}
	}
	return requests
}

func decodeJSON(t *testing.T, response *http.Response, destination any) {
	t.Helper()
	defer response.Body.Close()
	if err := json.NewDecoder(response.Body).Decode(destination); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}
