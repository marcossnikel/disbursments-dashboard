import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { AlertCircle, RefreshCw } from "lucide-react";
import { useState } from "react";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { ApiError } from "@/api/client";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { BatchConfirmationDialog } from "@/features/disbursements/BatchConfirmationDialog";
import { BatchProgress } from "@/features/disbursements/BatchProgress";
import { WorkerSelection } from "@/features/disbursements/WorkerSelection";
import {
  batchQueryOptions,
  createBatchID,
  submitBatch,
  workersQueryKey,
  workersQueryOptions,
} from "@/features/disbursements/queries";

export function DisbursementDashboard() {
  const workersQuery = useQuery(workersQueryOptions());
  const queryClient = useQueryClient();
  const [selectedWorkerIDs, setSelectedWorkerIDs] = useState<ReadonlySet<string>>(
    () => new Set(),
  );
  const [isConfirmationOpen, setConfirmationOpen] = useState(false);
  const [activeBatchID, setActiveBatchID] = useState<string | null>(null);
  const batchQuery = useQuery(batchQueryOptions(activeBatchID));
  const submitBatchMutation = useMutation({
    mutationFn: submitBatch,
    onSuccess: (submission) => {
      setActiveBatchID(submission.batch_id);
      setConfirmationOpen(false);
      setSelectedWorkerIDs(new Set());
      void queryClient.invalidateQueries({ queryKey: workersQueryKey });
    },
  });

  if (workersQuery.isPending) {
    return <WorkersLoadingState />;
  }

  if (workersQuery.isError) {
    return (
      <Alert variant="destructive" className="border-status-danger/15 bg-status-danger-soft">
        <AlertCircle aria-hidden="true" />
        <AlertTitle>We couldn't load pending disbursements</AlertTitle>
        <AlertDescription className="flex flex-col items-start gap-3 sm:flex-row sm:items-center sm:justify-between">
          <span>Check that the API is running, then try again. Your selection has not changed.</span>
          <Button variant="outline" size="sm" onClick={() => void workersQuery.refetch()}>
            <RefreshCw aria-hidden="true" />
            Try again
          </Button>
        </AlertDescription>
      </Alert>
    );
  }

  const workers = workersQuery.data;
  const selectedWorkers = workers.filter((worker) => selectedWorkerIDs.has(worker.id));

  if (workers.length === 0) {
    return (
      <Card className="border-dashed bg-white/75 py-16 text-center">
        <CardContent>
          <p className="text-lg font-semibold">No pending disbursements</p>
          <p className="mt-2 text-sm text-muted-foreground">
            Every available worker obligation has already been processed or reserved.
          </p>
        </CardContent>
      </Card>
    );
  }

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

  function confirmBatch() {
    submitBatchMutation.mutate({
      batchID: createBatchID(),
      workerIDs: selectedWorkers.map((worker) => worker.id),
    });
  }

  const submissionError =
    submitBatchMutation.error instanceof ApiError ? submitBatchMutation.error : undefined;

  return (
    <>
      {batchQuery.data ? (
        <BatchProgress
          batch={batchQuery.data}
          refreshFailed={batchQuery.isRefetchError}
          isRefreshing={batchQuery.isFetching}
          onRefresh={() => void batchQuery.refetch()}
        />
      ) : null}
      {activeBatchID !== null && batchQuery.isPending ? <BatchLoadingState /> : null}
      {activeBatchID !== null && batchQuery.isError && !batchQuery.data ? (
        <Alert variant="destructive" className="mb-6 border-status-danger/15 bg-status-danger-soft">
          <AlertCircle aria-hidden="true" />
          <AlertTitle>We couldn't load batch {activeBatchID}</AlertTitle>
          <AlertDescription className="flex flex-col items-start gap-3 sm:flex-row sm:items-center sm:justify-between">
            <span>The batch may still be processing. Retry without creating another batch.</span>
            <Button variant="outline" size="sm" onClick={() => void batchQuery.refetch()}>
              <RefreshCw aria-hidden="true" />
              Retry status
            </Button>
          </AlertDescription>
        </Alert>
      ) : null}
      <WorkerSelection
        workers={workers}
        selectedWorkerIDs={selectedWorkerIDs}
        onToggleWorker={toggleWorker}
        onReviewBatch={() => setConfirmationOpen(true)}
      />
      <BatchConfirmationDialog
        open={isConfirmationOpen}
        workers={selectedWorkers}
        onOpenChange={(open) => {
          setConfirmationOpen(open);
          if (!open) {
            submitBatchMutation.reset();
          }
        }}
        onConfirm={confirmBatch}
        isSubmitting={submitBatchMutation.isPending}
        errorMessage={submitBatchMutation.error?.message}
        requestID={submissionError?.requestID}
      />
    </>
  );
}

function BatchLoadingState() {
  return (
    <Card className="mb-6 border-0 bg-secondary text-secondary-foreground shadow-xl">
      <CardContent className="p-6">
        <div className="flex items-center gap-3">
          <RefreshCw aria-hidden="true" className="size-4 animate-spin text-primary" />
          <div>
            <p className="font-medium">Opening the accepted batch…</p>
            <p className="text-sm text-white/50">Pending results will appear here immediately.</p>
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
