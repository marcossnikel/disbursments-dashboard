import ArrowRight from "lucide-react/dist/esm/icons/arrow-right.mjs";
import LoaderCircle from "lucide-react/dist/esm/icons/loader-circle.mjs";
import ShieldCheck from "lucide-react/dist/esm/icons/shield-check.mjs";
import TriangleAlert from "lucide-react/dist/esm/icons/triangle-alert.mjs";

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
  const hasAvailabilityConflict =
    unavailableWorkers !== undefined && unavailableWorkers.length > 0;
  const availableWorkerCount =
    workers.length - (unavailableWorkers?.length ?? 0);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex max-h-[min(42rem,calc(100vh-2rem))] flex-col gap-0 overflow-hidden rounded-2xl p-0 sm:max-w-lg">
        <DialogHeader className="shrink-0 border-b px-6 py-5">
          <div className="mb-1 flex items-center gap-2 text-[#b93613]">
            <ShieldCheck aria-hidden="true" className="size-4" />
            <span className="text-xs font-semibold tracking-[0.12em] uppercase">
              Final review
            </span>
          </div>
          <DialogTitle className="text-xl">Confirm this batch</DialogTitle>
          <DialogDescription>
            Review every worker and currency total before starting provider
            payments.
          </DialogDescription>
        </DialogHeader>

        <div
          className={
            hasAvailabilityConflict
              ? "hidden"
              : "min-h-0 flex-1 space-y-5 overflow-y-auto px-6 py-5"
          }
        >
          <div>
            <p className="font-semibold">
              {workers.length} {workers.length === 1 ? "worker" : "workers"}{" "}
              selected
            </p>
            <p className="mt-1 text-sm text-muted-foreground">
              Totals stay separated by currency. No payment starts until you
              confirm.
            </p>
          </div>

          <div className="grid gap-3 sm:grid-cols-2" aria-label="Batch totals">
            {[...totals].map(([currency, minorUnits]) => (
              <div
                key={currency}
                className="rounded-xl border border-primary/15 bg-accent/50 p-4"
              >
                <p className="text-xs font-semibold tracking-[0.1em] text-accent-foreground uppercase">
                  {currency} total
                </p>
                <p className="mt-2 text-xl font-semibold tabular-nums">
                  {formatMinorUnits(minorUnits, currency)}
                </p>
              </div>
            ))}
          </div>

          <div>
            <p className="mb-2 text-xs font-semibold tracking-[0.1em] text-muted-foreground uppercase">
              Recipients
            </p>
            <div
              aria-label="Recipients"
              className="max-h-48 divide-y overflow-y-auto rounded-xl border"
            >
              {workers.map((worker) => (
                <div
                  key={worker.id}
                  className="flex items-center justify-between gap-4 px-4 py-3"
                >
                  <div className="min-w-0">
                    <p className="truncate text-sm font-medium">
                      {worker.name}
                    </p>
                    <p className="font-mono text-xs text-muted-foreground">
                      {worker.id}
                    </p>
                  </div>
                  <div className="shrink-0 text-right">
                    <p className="text-sm font-semibold tabular-nums">
                      {formatMoney(worker.amount, worker.currency)}
                    </p>
                    <p className="text-xs text-muted-foreground">
                      {worker.currency}
                    </p>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>

        {hasAvailabilityConflict ? (
          <div className="min-h-0 flex-1 overflow-y-auto">
            <BatchConflictNotice
              unavailableWorkers={unavailableWorkers}
              availableWorkerCount={availableWorkerCount}
              requestID={requestID}
              onCancel={() => onOpenChange(false)}
              onViewPaymentDetails={onViewPaymentDetails}
              onContinueWithAvailableWorkers={onContinueWithAvailableWorkers}
            />
          </div>
        ) : errorMessage ? (
          <div className="border-t px-6 py-4">
            <Alert
              variant="destructive"
              className="border-status-danger/15 bg-status-danger-soft"
            >
              <TriangleAlert aria-hidden="true" />
              <AlertTitle>The batch wasn't started</AlertTitle>
              <AlertDescription>
                {errorMessage}
                {requestID ? (
                  <span className="mt-1 block font-mono text-xs">
                    Request {requestID}
                  </span>
                ) : null}
              </AlertDescription>
            </Alert>
          </div>
        ) : null}

        {!hasAvailabilityConflict ? (
          <DialogFooter className="m-0 shrink-0 rounded-none border-t bg-muted/35 px-6 py-4">
            <DialogClose asChild>
              <Button variant="outline" disabled={isSubmitting}>
                Go back
              </Button>
            </DialogClose>
            <Button
              onClick={onConfirm}
              className="gap-2"
              disabled={isSubmitting}
            >
              {isSubmitting ? (
                <>
                  <LoaderCircle
                    aria-hidden="true"
                    className="size-4 animate-spin"
                  />
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
