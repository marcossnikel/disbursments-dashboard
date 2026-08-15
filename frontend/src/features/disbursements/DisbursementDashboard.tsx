import { useQuery } from "@tanstack/react-query";
import AlertCircle from "lucide-react/dist/esm/icons/alert-circle.mjs";
import { useState } from "react";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { BatchConfirmationDialog } from "@/features/disbursements/BatchConfirmationDialog";
import { BatchProgress } from "@/features/disbursements/BatchProgress";
import { DemoResetDialog } from "@/features/disbursements/DemoResetDialog";
import { DisbursementHistory } from "@/features/disbursements/DisbursementHistory";
import {
  BatchTrackingFeedback,
  NoPendingDisbursements,
  WorkersLoadError,
  WorkersLoadingState,
} from "@/features/disbursements/DisbursementDashboardStates";
import {
  historyQueryOptions,
  type BatchSnapshot,
  workersQueryOptions,
} from "@/features/disbursements/queries";
import { useBatchSubmission } from "@/features/disbursements/useBatchSubmission";
import { useBatchTracking } from "@/features/disbursements/useBatchTracking";
import { useDemoReset } from "@/features/disbursements/useDemoReset";
import { useRetryPreparation } from "@/features/disbursements/useRetryPreparation";
import { useWorkerSelection } from "@/features/disbursements/useWorkerSelection";
import { WorkerSelection } from "@/features/disbursements/WorkerSelection";

export function DisbursementDashboard() {
  const workersQuery = useQuery(workersQueryOptions());
  const workers = workersQuery.data ?? [];
  const selection = useWorkerSelection(workers);
  const batchTracking = useBatchTracking();
  const [activeTab, setActiveTab] = useState<"new" | "history">("new");
  const [historyDetailBatchID, setHistoryDetailBatchID] = useState<
    string | null
  >(null);
  const [acceptedBatch, setAcceptedBatch] = useState<{
    batchID: string;
    createdAt: string;
  } | null>(null);
  const historyQuery = useQuery(historyQueryOptions(activeTab === "history"));
  const batchSubmission = useBatchSubmission({
    selectedWorkers: selection.selectedWorkers,
    onAccepted: (batchID) => {
      setAcceptedBatch({ batchID, createdAt: new Date().toISOString() });
      batchTracking.trackBatch(batchID);
      selection.clearSelection();
    },
  });
  const retryPreparation = useRetryPreparation({
    refreshWorkers,
    onReady: (workerID) => {
      selection.replaceSelection([workerID]);
      setActiveTab("new");
      batchSubmission.openConfirmation();
    },
  });
  const demoReset = useDemoReset({
    onReset: () => {
      setActiveTab("new");
      setHistoryDetailBatchID(null);
      setAcceptedBatch(null);
      batchTracking.clearBatchHistory();
      selection.clearSelection();
    },
  });
  const batchQuery = batchTracking.batchQuery;
  const historyBatches = mergeTrackedBatchIntoHistory(
    historyQuery.data,
    batchQuery.data,
    acceptedBatch,
    batchQuery.dataUpdatedAt,
  );

  async function refreshWorkers() {
    const refreshedWorkers = await workersQuery.refetch();
    if (refreshedWorkers.isError) {
      throw refreshedWorkers.error;
    }
    return refreshedWorkers.data ?? [];
  }

  function changeConfirmationOpen(open: boolean) {
    if (!open && batchSubmission.unavailableWorkerIDs.size > 0) {
      selection.removeWorkers(batchSubmission.unavailableWorkerIDs);
      void workersQuery.refetch();
    }
    batchSubmission.changeConfirmationOpen(open);
  }

  function continueWithAvailableWorkers() {
    selection.removeWorkers(batchSubmission.unavailableWorkerIDs);
    batchSubmission.changeConfirmationOpen(false);
    void workersQuery.refetch();
  }

  function viewConflictingPayment() {
    const conflictingBatchID =
      batchSubmission.unavailableWorkers?.[0]?.batch_id;
    if (!conflictingBatchID) {
      return;
    }
    selection.removeWorkers(batchSubmission.unavailableWorkerIDs);
    batchTracking.trackBatch(conflictingBatchID);
    setHistoryDetailBatchID(conflictingBatchID);
    setActiveTab("history");
    batchSubmission.changeConfirmationOpen(false);
  }

  function toggleHistoryBatch(batchID: string) {
    if (historyDetailBatchID === batchID) {
      setHistoryDetailBatchID(null);
      return;
    }
    setHistoryDetailBatchID(batchID);
    batchTracking.trackBatch(batchID);
  }

  if (workersQuery.isPending) {
    return <WorkersLoadingState />;
  }

  if (workersQuery.isError) {
    return <WorkersLoadError onRetry={() => void workersQuery.refetch()} />;
  }

  const trackedBatchDetails = (
    <>
      {batchQuery.data ? (
        <BatchProgress
          batch={batchQuery.data}
          refreshFailed={batchQuery.isRefetchError}
          isRefreshing={batchQuery.isFetching}
          lastUpdatedAt={batchQuery.dataUpdatedAt}
          onRefresh={() => void batchQuery.refetch()}
          retryingWorkerID={retryPreparation.retryingWorkerID}
          onPrepareRetry={(workerID) =>
            void retryPreparation.prepareRetry(workerID)
          }
        />
      ) : null}

      <BatchTrackingFeedback
        activeBatchID={batchTracking.activeBatchID}
        isLoading={batchQuery.isPending}
        error={batchQuery.error}
        hasBatch={batchQuery.data !== undefined}
        onStopTracking={batchTracking.stopTrackingBatch}
        onRetry={() => void batchQuery.refetch()}
      />

      {retryPreparation.errorMessage ? (
        <Alert className="mb-6 border-status-warning/20 bg-status-warning-soft">
          <AlertCircle aria-hidden="true" />
          <AlertTitle>Retry was not prepared</AlertTitle>
          <AlertDescription>{retryPreparation.errorMessage}</AlertDescription>
        </Alert>
      ) : null}
    </>
  );

  return (
    <>
      <Tabs
        value={activeTab}
        onValueChange={(value) => setActiveTab(value as "new" | "history")}
      >
        <TabsList aria-label="Disbursement sections">
          <TabsTrigger value="new">New disbursement</TabsTrigger>
          <TabsTrigger value="history">History</TabsTrigger>
        </TabsList>

        <TabsContent value="new">
          {trackedBatchDetails}

          {workers.length === 0 ? (
            <NoPendingDisbursements
              resetDisabled={batchQuery.data?.status === "processing"}
              onReset={demoReset.open}
            />
          ) : (
            <WorkerSelection
              workers={workers}
              selectedWorkerIDs={selection.selectedWorkerIDs}
              onToggleWorker={selection.toggleWorker}
              onToggleAllWorkers={selection.toggleAllWorkers}
              onReviewBatch={batchSubmission.openConfirmation}
            />
          )}
        </TabsContent>

        <TabsContent value="history">
          <DisbursementHistory
            detailBatchID={historyDetailBatchID}
            batches={historyBatches}
            isLoading={historyQuery.isPending}
            isRefreshing={historyQuery.isFetching}
            onRefresh={() => void historyQuery.refetch()}
            onToggleBatch={toggleHistoryBatch}
          />

          {historyDetailBatchID ? (
            <div className="mt-6">{trackedBatchDetails}</div>
          ) : null}
        </TabsContent>
      </Tabs>

      <BatchConfirmationDialog
        open={batchSubmission.isConfirmationOpen}
        workers={selection.selectedWorkers}
        onOpenChange={changeConfirmationOpen}
        onConfirm={batchSubmission.confirmBatch}
        isSubmitting={batchSubmission.isSubmitting}
        errorMessage={batchSubmission.errorMessage}
        requestID={batchSubmission.requestID}
        unavailableWorkers={batchSubmission.unavailableWorkers}
        onViewPaymentDetails={viewConflictingPayment}
        onContinueWithAvailableWorkers={continueWithAvailableWorkers}
      />
      <DemoResetDialog
        open={demoReset.isOpen}
        isResetting={demoReset.isResetting}
        errorMessage={demoReset.errorMessage}
        onOpenChange={demoReset.changeOpen}
        onConfirm={demoReset.confirmReset}
      />
    </>
  );
}

function mergeTrackedBatchIntoHistory(
  history: readonly BatchSnapshot[] | undefined,
  trackedBatch: BatchSnapshot | undefined,
  acceptedBatch: { batchID: string; createdAt: string } | null,
  trackedBatchUpdatedAt: number,
): readonly BatchSnapshot[] | undefined {
  if (!trackedBatch) {
    return history;
  }

  const createdAt =
    trackedBatch.created_at ||
    (acceptedBatch?.batchID === trackedBatch.batch_id
      ? acceptedBatch.createdAt
      : trackedBatchUpdatedAt > 0
        ? new Date(trackedBatchUpdatedAt).toISOString()
        : null);
  if (!createdAt) {
    return history;
  }

  const latestSnapshot = { ...trackedBatch, created_at: createdAt };
  const existingIndex = history?.findIndex(
    (batch) => batch.batch_id === trackedBatch.batch_id,
  );
  if (existingIndex !== undefined && existingIndex >= 0 && history) {
    return history.map((batch, index) =>
      index === existingIndex ? latestSnapshot : batch,
    );
  }

  if (
    acceptedBatch?.batchID === trackedBatch.batch_id ||
    !history ||
    history.length === 0
  ) {
    return [latestSnapshot, ...(history ?? [])];
  }

  return history;
}
