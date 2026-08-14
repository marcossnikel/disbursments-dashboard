import { useState } from "react";

import type { Worker } from "@/features/disbursements/queries";

type RetryPreparationOptions = {
  refreshWorkers: () => Promise<readonly Worker[]>;
  onReady: (workerID: string) => void;
};

export function useRetryPreparation({
  refreshWorkers,
  onReady,
}: RetryPreparationOptions) {
  const [retryingWorkerID, setRetryingWorkerID] = useState<string | null>(null);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  async function prepareRetry(workerID: string) {
    setRetryingWorkerID(workerID);
    setErrorMessage(null);

    try {
      const refreshedWorkers = await refreshWorkers();
      const workerIsAvailable = refreshedWorkers.some(
        (worker) => worker.id === workerID,
      );
      if (!workerIsAvailable) {
        setErrorMessage(
          "This worker is no longer available for a new batch. Review the original payment before taking another action.",
        );
        return;
      }
      onReady(workerID);
    } catch {
      setErrorMessage(
        "We couldn't refresh this worker. No retry was started; check the connection and try again.",
      );
    } finally {
      setRetryingWorkerID(null);
    }
  }

  return { errorMessage, prepareRetry, retryingWorkerID };
}
