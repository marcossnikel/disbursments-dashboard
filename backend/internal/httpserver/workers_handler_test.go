package httpserver_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/marcosnikel/cadana-disbursement-tool/backend/internal/openapi"
)

func TestServerListsAvailableWorkersWithARequestID(t *testing.T) {
	t.Parallel()

	var logOutput bytes.Buffer
	testServer := newTestServer(
		t,
		unusedProvider{},
		10,
		slog.New(slog.NewJSONHandler(&logOutput, nil)),
	)
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

	var workers []openapi.Worker
	if err := json.NewDecoder(response.Body).Decode(&workers); err != nil {
		t.Fatalf("decode workers: %v", err)
	}
	if got, want := len(workers), 10; got != want {
		t.Fatalf("worker count = %d, want %d", got, want)
	}
	if got, want := workers[0].Amount, "1500.50"; got != want {
		t.Errorf("first worker amount = %q, want %q", got, want)
	}
	if got, want := workers[0].Currency, openapi.USD; got != want {
		t.Errorf("first worker currency = %q, want %q", got, want)
	}

	for _, expected := range []string{
		`"msg":"HTTP request completed"`,
		`"request_id":"` + requestID + `"`,
		`"method":"GET"`,
		`"path":"/workers"`,
		`"status":200`,
	} {
		if !strings.Contains(logOutput.String(), expected) {
			t.Errorf("access log = %q, want to contain %q", logOutput.String(), expected)
		}
	}
}
