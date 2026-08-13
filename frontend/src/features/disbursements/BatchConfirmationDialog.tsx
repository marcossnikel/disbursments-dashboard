import { ArrowRight, LoaderCircle, ShieldCheck, TriangleAlert } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  formatMinorUnits,
  formatMoney,
  totalsByCurrency,
  type Worker,
} from "@/features/disbursements/formatMoney";
import type { components } from "@/api/generated/schema";
import { BatchConflictNotice } from "@/features/disbursements/BatchConflictNotice";

type UnavailableWorker = components["schemas"]["UnavailableWorker"];

type BatchConfirmationDialogProps = {
  open: boolean;
  workers: readonly Worker[];
  onOpenChange: (open: boolean) => void;
  onConfirm: () => void;
  isSubmitting: boolean;
  errorMessage?: string;
  requestID?: string;
  unavailableWorkers?: readonly UnavailableWorker[];
  onViewPaymentDetails: () => void;
  onContinueWithAvailableWorkers: () => void;
};

export function BatchConfirmationDialog({
  open,
  workers,
  onOpenChange,
  onConfirm,
  isSubmitting,
  errorMessage,
  requestID,
  unavailableWorkers,
  onViewPaymentDetails,
  onContinueWithAvailableWorkers,
}: BatchConfirmationDialogProps) {
  const totals = totalsByCurrency(workers);
  const hasAvailabilityConflict = unavailableWorkers !== undefined && unavailableWorkers.length > 0;
  const availableWorkerCount = workers.length - (unavailableWorkers?.length ?? 0);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[min(42rem,calc(100vh-2rem))] gap-0 overflow-hidden p-0 sm:max-w-xl">
        <DialogHeader className="border-b px-6 py-5">
          <div className="mb-1 flex items-center gap-2 text-primary">
            <ShieldCheck aria-hidden="true" className="size-4" />
            <span className="text-xs font-semibold tracking-[0.12em] uppercase">Final review</span>
          </div>
          <DialogTitle className="text-xl">
            Confirm this batch
          </DialogTitle>
          <DialogDescription>
            Review every worker and currency total before starting provider payments.
          </DialogDescription>
        </DialogHeader>

        <div className="max-h-72 overflow-y-auto px-6 py-2">
          {workers.map((worker) => (
            <div
              key={worker.id}
              className="flex items-center justify-between gap-4 border-b py-3 last:border-0"
            >
              <div className="min-w-0">
                <p className="truncate text-sm font-medium">{worker.name}</p>
                <p className="font-mono text-xs text-muted-foreground">{worker.id}</p>
              </div>
              <div className="text-right">
                <p className="text-sm font-semibold tabular-nums">
                  {formatMoney(worker.amount, worker.currency)}
                </p>
                <p className="text-xs text-muted-foreground">{worker.currency}</p>
              </div>
            </div>
          ))}
        </div>

        <div className="border-t bg-secondary px-6 py-4 text-secondary-foreground">
          <p className="mb-2 text-xs font-medium tracking-[0.12em] text-white/50 uppercase">
            Batch totals
          </p>
          <div className="flex flex-wrap gap-2">
            {[...totals].map(([currency, minorUnits]) => (
              <Badge key={currency} className="rounded-full bg-white/10 px-3 py-1 text-white">
                {formatMinorUnits(minorUnits, currency)}
              </Badge>
            ))}
          </div>
        </div>

        {hasAvailabilityConflict ? (
          <BatchConflictNotice
            unavailableWorkers={unavailableWorkers}
            availableWorkerCount={availableWorkerCount}
            requestID={requestID}
            onCancel={() => onOpenChange(false)}
            onViewPaymentDetails={onViewPaymentDetails}
            onContinueWithAvailableWorkers={onContinueWithAvailableWorkers}
          />
        ) : errorMessage ? (
          <div className="border-t px-6 py-4">
            <Alert variant="destructive" className="border-status-danger/15 bg-status-danger-soft">
              <TriangleAlert aria-hidden="true" />
              <AlertTitle>The batch wasn't started</AlertTitle>
              <AlertDescription>
                {errorMessage}
                {requestID ? (
                  <span className="mt-1 block font-mono text-xs">Request {requestID}</span>
                ) : null}
              </AlertDescription>
            </Alert>
          </div>
        ) : null}

        {!hasAvailabilityConflict ? (
          <DialogFooter className="m-0 rounded-none px-6 py-4">
            <DialogClose asChild>
              <Button variant="outline" disabled={isSubmitting}>
                Go back
              </Button>
            </DialogClose>
            <Button onClick={onConfirm} className="gap-2" disabled={isSubmitting}>
              {isSubmitting ? (
                <>
                  <LoaderCircle aria-hidden="true" className="size-4 animate-spin" />
                  Starting batch…
                </>
              ) : (
                <>
                  Confirm and disburse
                  <ArrowRight aria-hidden="true" className="size-4" />
                </>
              )}
            </Button>
          </DialogFooter>
        ) : null}
      </DialogContent>
    </Dialog>
  );
}
