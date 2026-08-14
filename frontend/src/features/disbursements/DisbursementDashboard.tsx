import { AlertCircle, RefreshCw, RotateCcw } from "lucide-react";

import { ApiError } from "@/api/client";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { BatchConfirmationDialog } from "@/features/disbursements/BatchConfirmationDialog";
import { BatchProgress } from "@/features/disbursements/BatchProgress";
import { DemoResetDialog } from "@/features/disbursements/DemoResetDialog";
import { useDisbursementWorkflow } from "@/features/disbursements/useDisbursementWorkflow";
import { WorkerSelection } from "@/features/disbursements/WorkerSelection";

export function DisbursementDashboard() {
  const workflow = useDisbursementWorkflow();

  if (workflow.workersLoading) {
    return <WorkersLoadingState />;
  }

  if (workflow.workersLoadFailed) {
    return <WorkersLoadError onRetry={() => void workflow.refreshWorkers()} />;
  }

  return (
    <>
      {workflow.batch ? (
        <BatchProgress
          batch={workflow.batch}
          refreshFailed={workflow.batchRefreshFailed}
          isRefreshing={workflow.isBatchRefreshing}
          lastUpdatedAt={workflow.lastBatchUpdatedAt}
          onRefresh={() => void workflow.refreshBatch()}
          retryingWorkerID={workflow.retryingWorkerID}
          onPrepareRetry={(workerID) => void workflow.prepareRetry(workerID)}
        />
      ) : null}

      <BatchTrackingFeedback
        activeBatchID={workflow.activeBatchID}
        isLoading={workflow.isBatchLoading}
        error={workflow.batchError}
        hasBatch={workflow.batch !== undefined}
        onStopTracking={workflow.stopTrackingBatch}
        onRetry={() => void workflow.refreshBatch()}
      />

      {workflow.retryPreparationError ? (
        <Alert className="mb-6 border-status-warning/20 bg-status-warning-soft">
          <AlertCircle aria-hidden="true" />
          <AlertTitle>Retry was not prepared</AlertTitle>
          <AlertDescription>{workflow.retryPreparationError}</AlertDescription>
        </Alert>
      ) : null}

      {workflow.workers.length === 0 ? (
        <NoPendingDisbursements
          resetDisabled={workflow.batch?.status === "processing"}
          onReset={workflow.openResetDialog}
        />
      ) : (
        <WorkerSelection
          workers={workflow.workers}
          selectedWorkerIDs={workflow.selectedWorkerIDs}
          onToggleWorker={workflow.toggleWorker}
          onToggleAllWorkers={workflow.toggleAllWorkers}
          onReviewBatch={workflow.openConfirmation}
        />
      )}

      <BatchConfirmationDialog
        open={workflow.isConfirmationOpen}
        workers={workflow.selectedWorkers}
        onOpenChange={workflow.changeConfirmationOpen}
        onConfirm={workflow.confirmBatch}
        isSubmitting={workflow.isSubmittingBatch}
        errorMessage={workflow.submissionErrorMessage}
        requestID={workflow.submissionError?.requestID}
        unavailableWorkers={workflow.unavailableWorkers}
        onViewPaymentDetails={workflow.viewConflictingPayment}
        onContinueWithAvailableWorkers={workflow.continueWithAvailableWorkers}
      />
      <DemoResetDialog
        open={workflow.isResetDialogOpen}
        isResetting={workflow.isResettingDemo}
        errorMessage={workflow.resetDemoErrorMessage}
        onOpenChange={workflow.changeResetDialogOpen}
        onConfirm={workflow.confirmDemoReset}
      />
    </>
  );
}

function WorkersLoadError({ onRetry }: { onRetry: () => void }) {
  return (
    <Alert
      variant="destructive"
      className="border-status-danger/15 bg-status-danger-soft"
    >
      <AlertCircle aria-hidden="true" />
      <AlertTitle>We couldn't load pending disbursements</AlertTitle>
      <AlertDescription className="flex flex-col items-start gap-3 sm:flex-row sm:items-center sm:justify-between">
        <span>
          Check that the API is running, then try again. Your selection has not
          changed.
        </span>
        <Button variant="outline" size="sm" onClick={onRetry}>
          <RefreshCw aria-hidden="true" />
          Try again
        </Button>
      </AlertDescription>
    </Alert>
  );
}

type BatchTrackingFeedbackProps = {
  activeBatchID: string | null;
  isLoading: boolean;
  error: Error | null;
  hasBatch: boolean;
  onStopTracking: () => void;
  onRetry: () => void;
};

function BatchTrackingFeedback({
  activeBatchID,
  isLoading,
  error,
  hasBatch,
  onStopTracking,
  onRetry,
}: BatchTrackingFeedbackProps) {
  if (activeBatchID === null) {
    return null;
  }
  if (isLoading) {
    return <BatchLoadingState />;
  }
  if (!error || hasBatch) {
    return null;
  }

  return (
    <Alert
      variant="destructive"
      className="mb-6 border-status-danger/15 bg-status-danger-soft"
    >
      <AlertCircle aria-hidden="true" />
      <AlertTitle>We couldn't load batch {activeBatchID}</AlertTitle>
      <AlertDescription className="flex flex-col items-start gap-3 sm:flex-row sm:items-center sm:justify-between">
        <span>
          The batch may still be processing. Retrying is safe because it only
          reads status. Stopping tracking clears this view; it does not cancel
          payments.
          {error instanceof ApiError ? (
            <span className="mt-1 block font-mono text-xs">
              Request {error.requestID}
            </span>
          ) : null}
        </span>
        <span className="flex shrink-0 flex-wrap gap-2">
          <Button variant="ghost" size="sm" onClick={onStopTracking}>
            Stop tracking
          </Button>
          <Button variant="outline" size="sm" onClick={onRetry}>
            <RefreshCw aria-hidden="true" />
            Retry status
          </Button>
        </span>
      </AlertDescription>
    </Alert>
  );
}

function NoPendingDisbursements({
  resetDisabled,
  onReset,
}: {
  resetDisabled: boolean;
  onReset: () => void;
}) {
  return (
    <Card className="border-dashed bg-white/75 py-14 text-center">
      <CardContent>
        <p className="text-lg font-semibold">No pending disbursements</p>
        <p className="mt-2 text-sm text-muted-foreground">
          Every available worker obligation has already been processed or
          reserved.
        </p>
        <Button
          variant="outline"
          className="mt-5"
          disabled={resetDisabled}
          onClick={onReset}
        >
          <RotateCcw aria-hidden="true" />
          Reset demo data
        </Button>
        {resetDisabled ? (
          <p className="mt-2 text-xs text-muted-foreground">
            Reset becomes available after the active batch finishes.
          </p>
        ) : null}
      </CardContent>
    </Card>
  );
}

function BatchLoadingState() {
  return (
    <Card className="mb-6 border-0 bg-secondary text-secondary-foreground shadow-xl">
      <CardContent className="p-6">
        <div className="flex items-center gap-3">
          <RefreshCw
            aria-hidden="true"
            className="size-4 animate-spin text-primary"
          />
          <div>
            <p className="font-medium">Opening the accepted batch…</p>
            <p className="text-sm text-white/50">
              Pending results will appear here immediately.
            </p>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}

function WorkersLoadingState() {
  return (
    <Card className="border-black/5 shadow-xl shadow-black/5">
      <CardContent className="p-6">
        <p className="mb-5 text-sm font-medium text-muted-foreground">
          Loading pending disbursements…
        </p>
        <div className="space-y-3" aria-hidden="true">
          {[0, 1, 2, 3].map((row) => (
            <Skeleton key={row} className="h-14 w-full rounded-xl" />
          ))}
        </div>
      </CardContent>
    </Card>
  );
}
