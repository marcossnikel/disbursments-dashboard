import LoaderCircle from "lucide-react/dist/esm/icons/loader-circle.mjs";
import RotateCcw from "lucide-react/dist/esm/icons/rotate-ccw.mjs";

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

type DemoResetDialogProps = {
  open: boolean;
  isResetting: boolean;
  errorMessage?: string;
  onOpenChange: (open: boolean) => void;
  onConfirm: () => void;
};

export function DemoResetDialog({
  open,
  isResetting,
  errorMessage,
  onOpenChange,
  onConfirm,
}: DemoResetDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="rounded-2xl sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Reset demo data?</DialogTitle>
          <DialogDescription>
            This clears every in-memory batch and restores all seeded workers as
            pending disbursements. It is a demo utility, not a payment
            reconciliation.
          </DialogDescription>
        </DialogHeader>

        {errorMessage ? (
          <Alert variant="destructive">
            <AlertTitle>The demo was not reset</AlertTitle>
            <AlertDescription>{errorMessage}</AlertDescription>
          </Alert>
        ) : null}

        <DialogFooter>
          <DialogClose asChild>
            <Button variant="outline" disabled={isResetting}>
              Cancel
            </Button>
          </DialogClose>
          <Button onClick={onConfirm} disabled={isResetting}>
            {isResetting ? (
              <LoaderCircle aria-hidden="true" className="animate-spin" />
            ) : (
              <RotateCcw aria-hidden="true" />
            )}
            {isResetting ? "Resetting…" : "Reset and restore workers"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
