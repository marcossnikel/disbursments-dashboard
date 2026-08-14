import CircleDollarSign from "lucide-react/dist/esm/icons/circle-dollar-sign.mjs";
import UsersRound from "lucide-react/dist/esm/icons/users-round.mjs";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { formatMoney, type Worker } from "@/features/disbursements/formatMoney";

type WorkerSelectionProps = {
  workers: readonly Worker[];
  selectedWorkerIDs: ReadonlySet<string>;
  onToggleWorker: (workerID: string) => void;
  onToggleAllWorkers: () => void;
  onReviewBatch: () => void;
};

export function WorkerSelection({
  workers,
  selectedWorkerIDs,
  onToggleWorker,
  onToggleAllWorkers,
  onReviewBatch,
}: WorkerSelectionProps) {
  const selectedCount = selectedWorkerIDs.size;
  const allWorkersSelected = selectedCount === workers.length;
  const someWorkersSelected = selectedCount > 0 && !allWorkersSelected;
  const buttonLabel = `Disburse ${selectedCount} ${selectedCount === 1 ? "worker" : "workers"}`;

  return (
    <Card className="overflow-hidden border-black/5 shadow-xl shadow-black/5">
      <CardHeader className="gap-3 border-b bg-white px-5 py-5 sm:flex-row sm:items-center sm:justify-between sm:px-6">
        <div>
          <div className="mb-2 flex items-center gap-2 text-sm font-medium text-[#b93613]">
            <UsersRound aria-hidden="true" className="size-4" />
            Pending disbursements
          </div>
          <CardTitle className="text-xl tracking-tight">
            Choose workers for this batch
          </CardTitle>
        </div>
        <div className="flex flex-wrap items-center gap-4">
          <label
            htmlFor="select-all-workers"
            className="flex cursor-pointer items-center gap-2 text-sm font-medium"
          >
            <Checkbox
              id="select-all-workers"
              checked={
                allWorkersSelected
                  ? true
                  : someWorkersSelected
                    ? "indeterminate"
                    : false
              }
              onCheckedChange={onToggleAllWorkers}
              aria-label="Select all workers"
            />
            {allWorkersSelected ? "Clear all" : "Select all"}
          </label>
          <Badge variant="secondary" className="w-fit rounded-full px-3 py-1">
            {workers.length} available
          </Badge>
        </div>
      </CardHeader>

      <CardContent className="p-0">
        <div className="overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow className="bg-muted/70 hover:bg-muted/70">
                <TableHead className="w-12 pl-5 sm:pl-6">
                  <span className="sr-only">Select</span>
                </TableHead>
                <TableHead>Worker</TableHead>
                <TableHead className="hidden sm:table-cell">ID</TableHead>
                <TableHead>Currency</TableHead>
                <TableHead className="pr-5 text-right sm:pr-6">
                  Amount
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {workers.map((worker) => {
                const isSelected = selectedWorkerIDs.has(worker.id);
                return (
                  <TableRow
                    key={worker.id}
                    data-state={isSelected ? "selected" : undefined}
                  >
                    <TableCell className="pl-5 sm:pl-6">
                      <Checkbox
                        id={`worker-${worker.id}`}
                        checked={isSelected}
                        onCheckedChange={() => onToggleWorker(worker.id)}
                        aria-label={`Select ${worker.name}`}
                      />
                    </TableCell>
                    <TableCell>
                      <label
                        htmlFor={`worker-${worker.id}`}
                        className="block cursor-pointer"
                      >
                        <span className="block font-medium">{worker.name}</span>
                        <span className="block text-xs text-muted-foreground sm:hidden">
                          {worker.id}
                        </span>
                      </label>
                    </TableCell>
                    <TableCell className="hidden font-mono text-xs text-muted-foreground sm:table-cell">
                      {worker.id}
                    </TableCell>
                    <TableCell>
                      <Badge
                        variant="outline"
                        className="rounded-full font-mono text-[0.7rem]"
                      >
                        {worker.currency}
                      </Badge>
                    </TableCell>
                    <TableCell className="pr-5 text-right font-semibold tabular-nums sm:pr-6">
                      {formatMoney(worker.amount, worker.currency)}
                    </TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        </div>

        <div className="flex flex-col gap-4 border-t bg-muted/35 px-5 py-4 sm:flex-row sm:items-center sm:justify-between sm:px-6">
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <CircleDollarSign
              aria-hidden="true"
              className="size-4 text-[#b93613]"
            />
            {selectedCount === 0
              ? "Select at least one worker to continue."
              : `${selectedCount} ${selectedCount === 1 ? "worker" : "workers"} selected`}
          </div>
          <Button
            size="lg"
            className="min-w-44 rounded-xl px-5 shadow-lg shadow-primary/20"
            disabled={selectedCount === 0}
            onClick={onReviewBatch}
          >
            {buttonLabel}
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
