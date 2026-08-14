import { useQuery } from "@tanstack/react-query";
import AlertCircle from "lucide-react/dist/esm/icons/alert-circle.mjs";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { BatchConfirmationDialog } from "@/features/disbursements/BatchConfirmationDialog";
import { BatchProgress } from "@/features/disbursements/BatchProgress";
import { DemoResetDialog } from "@/features/disbursements/DemoResetDialog";
import {
  BatchTrackingFeedback,
  NoPendingDisbursements,
  WorkersLoadError,
  WorkersLoadingState,
} from "@/features/disbursements/DisbursementDashboardStates";
import { workersQueryOptions } from "@/features/disbursements/queries";
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
  const batchSubmission = useBatchSubmission({
    selectedWorkers: selection.selectedWorkers,
    onAccepted: (batchID) => {
      batchTracking.trackBatch(batchID);
      selection.clearSelection();
    },
  });
  const retryPreparation = useRetryPreparation({
    refreshWorkers,
    onReady: (workerID) => {
      selection.replaceSelection([workerID]);
      batchSubmission.openConfirmation();
    },
  });
  const demoReset = useDemoReset({
    onReset: () => {
      batchTracking.clearBatchHistory();
      selection.clearSelection();
    },
  });
  const batchQuery = batchTracking.batchQuery;

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
    batchSubmission.changeConfirmationOpen(false);
  }

  if (workersQuery.isPending) {
    return <WorkersLoadingState />;
  }

  if (workersQuery.isError) {
    return <WorkersLoadError onRetry={() => void workersQuery.refetch()} />;
  }

  return (
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
