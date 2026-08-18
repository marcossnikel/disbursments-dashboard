import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import {
  cancelBatch,
  disbursementBatchesQueryKey,
  workersQueryKey,
} from "@/features/disbursements/queries";

export function useBatchCancellation(batchID: string | null) {
  const queryClient = useQueryClient();
  const [isOpen, setOpen] = useState(false);
  const [feedback, setFeedback] = useState<{
    batchID: string;
    message: string;
  } | null>(null);
  const mutation = useMutation({
    mutationFn: cancelBatch,
    onSuccess: (cancellation) => {
      queryClient.setQueryData(
        [...disbursementBatchesQueryKey, cancellation.batch.batch_id],
        cancellation.batch,
      );
      void queryClient.invalidateQueries({ queryKey: workersQueryKey });
      setFeedback({
        batchID: cancellation.batch.batch_id,
        message: cancellationFeedback(cancellation.canceled_count),
      });
      setOpen(false);
    },
  });

  function confirmCancellation() {
    if (batchID !== null) {
      mutation.mutate(batchID);
    }
  }

  function changeOpen(open: boolean) {
    setOpen(open);
    if (!open) {
      mutation.reset();
    }
  }

  return {
    changeOpen,
    confirmCancellation,
    errorMessage: mutation.error?.message,
    feedbackMessage: feedback?.batchID === batchID ? feedback.message : null,
    isCanceling: mutation.isPending,
    isOpen,
    open: () => {
      mutation.reset();
      setFeedback(null);
      setOpen(true);
    },
  };
}

function cancellationFeedback(canceledCount: number): string {
  if (canceledCount === 0) {
    return "No payments were canceled because every remaining payment had already reached the provider.";
  }
  const paymentLabel = canceledCount === 1 ? "payment" : "payments";
  return `${canceledCount} queued ${paymentLabel} canceled. Any in-flight provider calls will continue to a final result.`;
}
