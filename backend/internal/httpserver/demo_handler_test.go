package httpserver_test

import (
	"log/slog"
	"net/http"
	"testing"

	"github.com/marcosnikel/cadana-disbursement-tool/backend/internal/openapi"
)

func TestServerResetsCompletedDemoStateButRejectsAResetDuringProcessing(t *testing.T) {
	t.Parallel()

	provider := newGatedProvider("")
	testServer := newTestServer(t, provider, 2, slog.New(slog.DiscardHandler))
	batchResponse := postBatch(t, testServer.Client(), testServer.URL, openapi.SubmitBatchRequest{
		BatchID: "batch-reset-demo", WorkerIDs: []string{"w-001"},
	})
	decodeJSON(t, batchResponse, &openapi.SubmitBatchResponse{})
	waitForPaymentStarts(t, provider.started, 1)

	blockedReset := postDemoReset(t, testServer.Client(), testServer.URL)
	if got, want := blockedReset.StatusCode, http.StatusConflict; got != want {
		blockedReset.Body.Close()
		t.Fatalf("POST /demo/reset while processing status = %d, want %d", got, want)
	}
	var blockedError openapi.ErrorResponse
	decodeJSON(t, blockedReset, &blockedError)
	if got, want := blockedError.Code, openapi.ErrorCodeDemoResetInProgress; got != want {
		t.Errorf("blocked reset code = %q, want %q", got, want)
	}

	close(provider.release)
	waitForCompletedBatch(t, testServer.Client(), testServer.URL, "batch-reset-demo")
	completedReset := postDemoReset(t, testServer.Client(), testServer.URL)
	if got, want := completedReset.StatusCode, http.StatusNoContent; got != want {
		completedReset.Body.Close()
		t.Fatalf("POST /demo/reset status = %d, want %d", got, want)
	}
	completedReset.Body.Close()

	workersResponse, err := testServer.Client().Get(testServer.URL + "/workers")
	if err != nil {
		t.Fatalf("GET /workers after reset error = %v", err)
	}
	var restoredWorkers []openapi.Worker
	decodeJSON(t, workersResponse, &restoredWorkers)
	if got, want := len(restoredWorkers), 2; got != want {
		t.Errorf("restored worker count = %d, want %d", got, want)
	}

	missingBatch, err := testServer.Client().Get(testServer.URL + "/disbursements/batch-reset-demo")
	if err != nil {
		t.Fatalf("GET reset batch error = %v", err)
	}
	if got, want := missingBatch.StatusCode, http.StatusNotFound; got != want {
		missingBatch.Body.Close()
		t.Fatalf("GET reset batch status = %d, want %d", got, want)
	}
	missingBatch.Body.Close()

	historyResponse, err := testServer.Client().Get(testServer.URL + "/disbursements")
	if err != nil {
		t.Fatalf("GET /disbursements after reset error = %v", err)
	}
	var history []openapi.BatchSnapshot
	decodeJSON(t, historyResponse, &history)
	if got := len(history); got != 0 {
		t.Errorf("history length after reset = %d, want 0", got)
	}
}
