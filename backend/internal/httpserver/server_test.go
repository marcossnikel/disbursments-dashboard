package httpserver_test

import (
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

	response, err := testServer.Client().Get(testServer.URL + "/workers")
	if err != nil {
		t.Fatalf("GET /workers error = %v", err)
	}
	defer response.Body.Close()

	if got, want := response.StatusCode, http.StatusOK; got != want {
		t.Fatalf("GET /workers status = %d, want %d", got, want)
	}
	if requestID := response.Header.Get("X-Request-ID"); !strings.HasPrefix(requestID, "req-") {
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
}

type unusedProvider struct{}

func (unusedProvider) Pay(
	context.Context,
	disbursement.PaymentRequest,
) (disbursement.PaymentResult, error) {
	return disbursement.PaymentResult{}, io.ErrUnexpectedEOF
}
