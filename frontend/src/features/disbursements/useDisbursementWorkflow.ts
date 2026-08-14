import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";

import { ApiError } from "@/api/client";
import {
  batchQueryOptions,
  createBatchID,
  disbursementBatchesQueryKey,
  resetDemo,
  submitBatch,
  workersQueryKey,
  workersQueryOptions,
} from "@/features/disbursements/queries";

export function useDisbursementWorkflow() {
  const queryClient = useQueryClient();
  const workersQuery = useQuery(workersQueryOptions());
  const [selectedWorkerIDs, setSelectedWorkerIDs] = useState<
    ReadonlySet<string>
  >(() => new Set());
  const [isConfirmationOpen, setConfirmationOpen] = useState(false);
  const [isResetDialogOpen, setResetDialogOpen] = useState(false);
  const [activeBatchID, setActiveBatchID] = useState<string | null>(
    readBatchIDFromURL,
  );
  const [retryingWorkerID, setRetryingWorkerID] = useState<string | null>(null);
  const [retryPreparationError, setRetryPreparationError] = useState<
    string | null
  >(null);
  const batchQuery = useQuery(batchQueryOptions(activeBatchID));

  const submitBatchMutation = useMutation({
    mutationFn: submitBatch,
    onSuccess: (submission) => {
      trackBatch(submission.batch_id);
      setConfirmationOpen(false);
      setSelectedWorkerIDs(new Set());
      void queryClient.invalidateQueries({ queryKey: workersQueryKey });
    },
  });

  const resetDemoMutation = useMutation({
    mutationFn: resetDemo,
    onSuccess: async () => {
      stopTrackingBatch();
      setSelectedWorkerIDs(new Set());
      setRetryPreparationError(null);
      setResetDialogOpen(false);
      queryClient.removeQueries({ queryKey: disbursementBatchesQueryKey });
      await queryClient.invalidateQueries({ queryKey: workersQueryKey });
    },
  });

  useEffect(() => {
    if (batchQuery.data?.status === "completed") {
      void queryClient.invalidateQueries({ queryKey: workersQueryKey });
    }
  }, [batchQuery.data?.status, queryClient]);

  const workers = workersQuery.data ?? [];
  const selectedWorkers = workers.filter((worker) =>
    selectedWorkerIDs.has(worker.id),
  );
  const submissionError =
    submitBatchMutation.error instanceof ApiError
      ? submitBatchMutation.error
      : undefined;
  const unavailableWorkers =
    submissionError?.details?.code === "workers_unavailable"
      ? submissionError.details.unavailable_workers
      : undefined;

  function toggleWorker(workerID: string) {
    setSelectedWorkerIDs((currentSelection) => {
      const nextSelection = new Set(currentSelection);
      if (nextSelection.has(workerID)) {
        nextSelection.delete(workerID);
      } else {
        nextSelection.add(workerID);
      }
      return nextSelection;
    });
  }

  function toggleAllWorkers() {
    setSelectedWorkerIDs((currentSelection) =>
      currentSelection.size === workers.length
        ? new Set()
        : new Set(workers.map((worker) => worker.id)),
    );
  }

  function confirmBatch() {
    submitBatchMutation.mutate({
      batchID: createBatchID(),
      workerIDs: selectedWorkers.map((worker) => worker.id),
    });
  }

  function openConfirmation() {
    setConfirmationOpen(true);
  }

  function changeConfirmationOpen(open: boolean) {
    setConfirmationOpen(open);
    if (open) {
      return;
    }
    if (hasUnavailableWorkers(unavailableWorkers)) {
      removeUnavailableWorkersFromSelection();
    }
    submitBatchMutation.reset();
  }

  function continueWithAvailableWorkers() {
    const unavailableWorkerIDs = getUnavailableWorkerIDs(unavailableWorkers);
    setSelectedWorkerIDs(
      new Set(
        selectedWorkers
          .filter((worker) => !unavailableWorkerIDs.has(worker.id))
          .map((worker) => worker.id),
      ),
    );
    setConfirmationOpen(false);
    submitBatchMutation.reset();
    void queryClient.invalidateQueries({ queryKey: workersQueryKey });
  }

  function removeUnavailableWorkersFromSelection() {
    const unavailableWorkerIDs = getUnavailableWorkerIDs(unavailableWorkers);
    setSelectedWorkerIDs(
      (currentSelection) =>
        new Set(
          [...currentSelection].filter(
            (workerID) => !unavailableWorkerIDs.has(workerID),
          ),
        ),
    );
    void queryClient.invalidateQueries({ queryKey: workersQueryKey });
  }

  function viewConflictingPayment() {
    const conflictingBatchID = unavailableWorkers?.[0]?.batch_id;
    if (!conflictingBatchID) {
      return;
    }
    removeUnavailableWorkersFromSelection();
    trackBatch(conflictingBatchID);
    setConfirmationOpen(false);
    submitBatchMutation.reset();
  }

  function trackBatch(batchID: string) {
    setActiveBatchID(batchID);
    writeBatchIDToURL(batchID);
  }

  function stopTrackingBatch() {
    setActiveBatchID(null);
    clearBatchIDFromURL();
  }

  async function prepareRetry(workerID: string) {
    setRetryingWorkerID(workerID);
    setRetryPreparationError(null);

    try {
      const refreshedWorkers = await workersQuery.refetch();
      if (refreshedWorkers.isError) {
        setRetryPreparationError(retryRefreshErrorMessage);
        return;
      }
      const workerIsAvailable = refreshedWorkers.data?.some(
        (worker) => worker.id === workerID,
      );
      if (!workerIsAvailable) {
        setRetryPreparationError(
          "This worker is no longer available for a new batch. Review the original payment before taking another action.",
        );
        return;
      }

      setSelectedWorkerIDs(new Set([workerID]));
      submitBatchMutation.reset();
      setConfirmationOpen(true);
    } catch {
      setRetryPreparationError(retryRefreshErrorMessage);
    } finally {
      setRetryingWorkerID(null);
    }
  }

  function changeResetDialogOpen(open: boolean) {
    setResetDialogOpen(open);
    if (!open) {
      resetDemoMutation.reset();
    }
  }

  function openResetDialog() {
    setResetDialogOpen(true);
  }

  return {
    activeBatchID,
    batch: batchQuery.data,
    batchError: batchQuery.error,
    batchRefreshFailed: batchQuery.isRefetchError,
    changeConfirmationOpen,
    changeResetDialogOpen,
    confirmDemoReset: () => resetDemoMutation.mutate(),
    confirmBatch,
    continueWithAvailableWorkers,
    isConfirmationOpen,
    isBatchLoading: batchQuery.isPending,
    isBatchRefreshing: batchQuery.isFetching,
    isResetDialogOpen,
    isResettingDemo: resetDemoMutation.isPending,
    isSubmittingBatch: submitBatchMutation.isPending,
    lastBatchUpdatedAt: batchQuery.dataUpdatedAt,
    openConfirmation,
    openResetDialog,
    prepareRetry,
    refreshBatch: () => batchQuery.refetch(),
    refreshWorkers: () => workersQuery.refetch(),
    resetDemoErrorMessage: resetDemoMutation.error?.message,
    retryPreparationError,
    retryingWorkerID,
    selectedWorkerIDs,
    selectedWorkers,
    stopTrackingBatch,
    submissionErrorMessage: submitBatchMutation.error?.message,
    submissionError,
    toggleAllWorkers,
    toggleWorker,
    unavailableWorkers,
    viewConflictingPayment,
    workers,
    workersLoadFailed: workersQuery.isError,
    workersLoading: workersQuery.isPending,
  };
}

const retryRefreshErrorMessage =
  "We couldn't refresh this worker. No retry was started; check the connection and try again.";

function hasUnavailableWorkers(
  workers: readonly { worker_id: string }[] | undefined,
): workers is readonly { worker_id: string }[] {
  return workers !== undefined && workers.length > 0;
}

function getUnavailableWorkerIDs(
  workers: readonly { worker_id: string }[] | undefined,
) {
  return new Set(workers?.map((worker) => worker.worker_id) ?? []);
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
