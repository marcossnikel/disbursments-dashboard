import ArrowRight from "lucide-react/dist/esm/icons/arrow-right.mjs";
import Eye from "lucide-react/dist/esm/icons/eye.mjs";
import ShieldAlert from "lucide-react/dist/esm/icons/shield-alert.mjs";

import type { components } from "@/api/generated/schema";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";

type UnavailableWorker = components["schemas"]["UnavailableWorker"];

type BatchConflictNoticeProps = {
  unavailableWorkers: readonly UnavailableWorker[];
  availableWorkerCount: number;
  requestID?: string;
  onCancel: () => void;
  onViewPaymentDetails: () => void;
  onContinueWithAvailableWorkers: () => void;
};

export function BatchConflictNotice({
  unavailableWorkers,
  availableWorkerCount,
  requestID,
  onCancel,
  onViewPaymentDetails,
  onContinueWithAvailableWorkers,
}: BatchConflictNoticeProps) {
  return (
    <div className="border-t px-6 py-5">
      <Alert className="border-status-warning/20 bg-status-warning-soft text-foreground">
        <ShieldAlert aria-hidden="true" className="text-status-warning" />
        <AlertTitle>No disbursements were started</AlertTitle>
        <AlertDescription className="text-muted-foreground">
          The batch changed before confirmation. We stopped the entire request
          so the selected workers were never silently changed.
        </AlertDescription>
      </Alert>

      <div className="mt-4 space-y-3">
        {unavailableWorkers.map((worker) => (
          <div
            key={worker.worker_id}
            className="rounded-xl border bg-muted/45 p-3"
          >
            <div className="flex flex-wrap items-center justify-between gap-2">
              <p className="text-sm font-semibold">{reasonMessage(worker)}</p>
              <Badge variant="outline" className="rounded-full text-[0.65rem]">
                {reasonLabel(worker.reason)}
              </Badge>
            </div>
            <div className="mt-2 flex flex-wrap gap-x-3 gap-y-1 font-mono text-[0.68rem] text-muted-foreground">
              <span>Worker {worker.worker_id}</span>
              <span>Batch {worker.batch_id}</span>
              <span>Disbursement {worker.disbursement_id}</span>
            </div>
          </div>
        ))}
      </div>

      <p className="mt-3 text-xs text-muted-foreground">
        {availableWorkerCount > 0
          ? "You can return with the available workers selected, then review the revised batch again before anything starts."
          : "No workers from this selection are currently available for another batch."}
      </p>
      {requestID ? (
        <p className="mt-2 font-mono text-xs">Request {requestID}</p>
      ) : null}

      <div className="mt-5 flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
        <Button variant="ghost" onClick={onCancel}>
          Cancel
        </Button>
        <Button variant="outline" onClick={onViewPaymentDetails}>
          <Eye aria-hidden="true" />
          View payment details
        </Button>
        {availableWorkerCount > 0 ? (
          <Button onClick={onContinueWithAvailableWorkers}>
            Continue with {availableWorkerCount} available{" "}
            {availableWorkerCount === 1 ? "worker" : "workers"}
            <ArrowRight aria-hidden="true" />
          </Button>
        ) : null}
      </div>
    </div>
  );
}

function reasonMessage(worker: UnavailableWorker): string {
  switch (worker.reason) {
    case "already_paid":
      return `${worker.worker_name} was already paid.`;
    case "already_pending":
      return `${worker.worker_name} already has a payment in progress.`;
    case "outcome_unknown":
      return `${worker.worker_name}'s provider result is unknown. Another payment could pay this worker twice.`;
  }
}

function reasonLabel(reason: UnavailableWorker["reason"]): string {
  switch (reason) {
    case "already_paid":
      return "Already paid";
    case "already_pending":
      return "Payment pending";
    case "outcome_unknown":
      return "Outcome unknown";
  }
}
