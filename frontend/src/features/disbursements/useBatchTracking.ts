import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";

import {
  batchQueryOptions,
  disbursementBatchesQueryKey,
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
  }

  return {
    activeBatchID,
    batchQuery,
    clearBatchHistory,
    stopTrackingBatch,
    trackBatch,
  };
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
