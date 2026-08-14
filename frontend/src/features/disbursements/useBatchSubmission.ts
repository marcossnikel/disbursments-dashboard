import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { ApiError } from "@/api/client";
import {
  createBatchID,
  submitBatch,
  workersQueryKey,
  type Worker,
} from "@/features/disbursements/queries";

type BatchSubmissionOptions = {
  selectedWorkers: readonly Worker[];
  onAccepted: (batchID: string) => void;
};

export function useBatchSubmission({
  selectedWorkers,
  onAccepted,
}: BatchSubmissionOptions) {
  const queryClient = useQueryClient();
  const [isConfirmationOpen, setConfirmationOpen] = useState(false);
  const mutation = useMutation({
    mutationFn: submitBatch,
    onSuccess: (submission) => {
      onAccepted(submission.batch_id);
      setConfirmationOpen(false);
      void queryClient.invalidateQueries({ queryKey: workersQueryKey });
    },
  });
  const submissionError =
    mutation.error instanceof ApiError ? mutation.error : undefined;
  const unavailableWorkers =
    submissionError?.details?.code === "workers_unavailable"
      ? submissionError.details.unavailable_workers
      : undefined;

  function confirmBatch() {
    mutation.mutate({
      batchID: createBatchID(),
      workerIDs: selectedWorkers.map((worker) => worker.id),
    });
  }

  function changeConfirmationOpen(open: boolean) {
    setConfirmationOpen(open);
    if (!open) {
      mutation.reset();
    }
  }

  return {
    changeConfirmationOpen,
    confirmBatch,
    errorMessage: mutation.error?.message,
    isConfirmationOpen,
    isSubmitting: mutation.isPending,
    openConfirmation: () => setConfirmationOpen(true),
    requestID: submissionError?.requestID,
    unavailableWorkerIDs: new Set(
      unavailableWorkers?.map((worker) => worker.worker_id) ?? [],
    ),
    unavailableWorkers,
  };
}
