import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";

import {
  batchQueryOptions,
  type BatchSnapshot,
  disbursementBatchesQueryKey,
  disbursementHistoryQueryKey,
  workersQueryKey,
} from "@/features/disbursements/queries";

export function useBatchTracking() {
  const queryClient = useQueryClient();
  const [activeBatchID, setActiveBatchID] = useState<string | null>(
    readBatchIDFromURL,
  );
  const batchQuery = useQuery(batchQueryOptions(activeBatchID));

  useEffect(() => {
    if (batchQuery.data?.status === "completed") {
      void queryClient.invalidateQueries({ queryKey: workersQueryKey });
    }
  }, [batchQuery.data?.status, queryClient]);

  useEffect(() => {
    const batch = batchQuery.data;
    if (!batch) {
      return;
    }

    const createdAt =
      batch.created_at ||
      (batchQuery.dataUpdatedAt > 0
        ? new Date(batchQuery.dataUpdatedAt).toISOString()
        : null);
    if (!createdAt) {
      return;
    }

    queryClient.setQueryData<BatchSnapshot[]>(
      disbursementHistoryQueryKey,
      (history) =>
        upsertBatchSnapshot(history, { ...batch, created_at: createdAt }),
    );
  }, [batchQuery.data, batchQuery.dataUpdatedAt, queryClient]);

  function trackBatch(batchID: string) {
    setActiveBatchID(batchID);
    writeBatchIDToURL(batchID);
  }

  function stopTrackingBatch() {
    setActiveBatchID(null);
    clearBatchIDFromURL();
  }

  function clearBatchHistory() {
    stopTrackingBatch();
    queryClient.removeQueries({ queryKey: disbursementBatchesQueryKey });
    queryClient.removeQueries({ queryKey: disbursementHistoryQueryKey });
  }

  return {
    activeBatchID,
    batchQuery,
    clearBatchHistory,
    stopTrackingBatch,
    trackBatch,
  };
}

function upsertBatchSnapshot(
  history: readonly BatchSnapshot[] | undefined,
  snapshot: BatchSnapshot,
): BatchSnapshot[] {
  const existingIndex = history?.findIndex(
    (batch) => batch.batch_id === snapshot.batch_id,
  );
  if (existingIndex !== undefined && existingIndex >= 0 && history) {
    return history.map((batch, index) =>
      index === existingIndex ? snapshot : batch,
    );
  }

  return [snapshot, ...(history ?? [])];
}

function readBatchIDFromURL(): string | null {
  return new URLSearchParams(window.location.search).get("batch");
}

function writeBatchIDToURL(batchID: string) {
  const url = new URL(window.location.href);
  url.searchParams.set("batch", batchID);
  window.history.replaceState(null, "", url);
}

function clearBatchIDFromURL() {
  const url = new URL(window.location.href);
  url.searchParams.delete("batch");
  window.history.replaceState(null, "", url);
}
