package disbursement_test

import (
	"context"
	"testing"

	"github.com/marcosnikel/cadana-disbursement-tool/backend/internal/disbursement"
)

func TestProcessorListsBatchSnapshotsNewestFirst(t *testing.T) {
	t.Parallel()

	provider := newCountingProvider()
	processor := newTestProcessor(t, provider, 2)

	if _, err := processor.Submit(context.Background(), "batch-older", []disbursement.WorkerID{"w-001"}); err != nil {
		t.Fatalf("first Submit() error = %v", err)
	}
	waitForPaymentStarts(t, provider.started, 1)
	if _, err := processor.Submit(context.Background(), "batch-newer", []disbursement.WorkerID{"w-002"}); err != nil {
		t.Fatalf("second Submit() error = %v", err)
	}
	waitForPaymentStarts(t, provider.started, 1)

	snapshots := processor.Batches()
	if got, want := len(snapshots), 2; got != want {
		t.Fatalf("Batches() length = %d, want %d", got, want)
	}
	if got, want := snapshots[0].BatchID, disbursement.BatchID("batch-newer"); got != want {
		t.Errorf("first batch ID = %q, want %q", got, want)
	}
	if got, want := snapshots[1].BatchID, disbursement.BatchID("batch-older"); got != want {
		t.Errorf("second batch ID = %q, want %q", got, want)
	}
	for _, snapshot := range snapshots {
		if snapshot.CreatedAt.IsZero() {
			t.Errorf("batch %q creation time is zero", snapshot.BatchID)
		}
		if got, want := snapshot.Status, disbursement.BatchProcessing; got != want {
			t.Errorf("batch %q status = %q, want %q", snapshot.BatchID, got, want)
		}
		if got, want := len(snapshot.Results), 1; got != want {
			t.Errorf("batch %q result count = %d, want %d", snapshot.BatchID, got, want)
		}
	}

	close(provider.release)
	waitForCompletedBatch(t, processor, "batch-older")
	waitForCompletedBatch(t, processor, "batch-newer")
}
