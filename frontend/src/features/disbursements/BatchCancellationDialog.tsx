import CircleX from "lucide-react/dist/esm/icons/circle-x.mjs";
import LoaderCircle from "lucide-react/dist/esm/icons/loader-circle.mjs";

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

type BatchCancellationDialogProps = {
  open: boolean;
  pendingCount: number;
  inFlightCount: number;
  isCanceling: boolean;
  errorMessage?: string;
  onOpenChange: (open: boolean) => void;
  onConfirm: () => void;
};

export function BatchCancellationDialog({
  open,
  pendingCount,
  inFlightCount,
  isCanceling,
  errorMessage,
  onOpenChange,
  onConfirm,
}: BatchCancellationDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="rounded-2xl sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Cancel queued payments?</DialogTitle>
          <DialogDescription>
            This will cancel {pendingCount} queued {paymentLabel(pendingCount)}{" "}
            that {pendingCount === 1 ? "has" : "have"} not reached the provider.
            {inFlightCount > 0
              ? ` ${inFlightCount} in-flight ${paymentLabel(inFlightCount)} will continue and record a final result.`
              : " No provider calls are currently in flight."}
          </DialogDescription>
        </DialogHeader>

        {errorMessage ? (
          <Alert variant="destructive">
            <AlertTitle>The batch was not canceled</AlertTitle>
            <AlertDescription>{errorMessage}</AlertDescription>
          </Alert>
        ) : null}

        <DialogFooter>
          <DialogClose asChild>
            <Button variant="outline" disabled={isCanceling}>
              Keep processing
            </Button>
          </DialogClose>
          <Button
            variant="destructive"
            onClick={onConfirm}
            disabled={isCanceling || pendingCount === 0}
          >
            {isCanceling ? (
              <LoaderCircle aria-hidden="true" className="animate-spin" />
            ) : (
              <CircleX aria-hidden="true" />
            )}
            {isCanceling ? "Canceling…" : "Cancel queued payments"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function paymentLabel(count: number): string {
  return count === 1 ? "payment" : "payments";
}
