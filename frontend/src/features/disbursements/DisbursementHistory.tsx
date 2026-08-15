import History from "lucide-react/dist/esm/icons/history.mjs";
import RefreshCw from "lucide-react/dist/esm/icons/refresh-cw.mjs";
import Search from "lucide-react/dist/esm/icons/search.mjs";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import {
  formatMinorUnits,
  totalsByCurrency,
} from "@/features/disbursements/formatMoney";
import type { BatchSnapshot } from "@/features/disbursements/queries";
import { cn } from "@/lib/utils";

type DisbursementHistoryProps = {
  detailBatchID: string | null;
  batches: readonly BatchSnapshot[] | undefined;
  isLoading: boolean;
  isRefreshing: boolean;
  onRefresh: () => void;
  onToggleBatch: (batchID: string) => void;
};

export function DisbursementHistory({
  detailBatchID,
  batches,
  isLoading,
  isRefreshing,
  onRefresh,
  onToggleBatch,
}: DisbursementHistoryProps) {
  const [searchQuery, setSearchQuery] = useState("");

  if (isLoading) {
    return <HistoryLoadingState />;
  }

  if (!batches || batches.length === 0) {
    return (
      <Card className="border-dashed bg-white/75 py-14 text-center">
        <CardContent>
          <History
            aria-hidden="true"
            className="mx-auto size-6 text-muted-foreground"
          />
          <p className="mt-3 text-lg font-semibold">
            No disbursement history yet
          </p>
          <p className="mt-2 text-sm text-muted-foreground">
            Processing and completed batches will appear here.
          </p>
        </CardContent>
      </Card>
    );
  }

  const normalizedSearchQuery = searchQuery.trim().toLocaleLowerCase();
  const filteredBatches =
    normalizedSearchQuery === ""
      ? batches
      : batches.filter((batch) =>
          batch.batch_id.toLocaleLowerCase().includes(normalizedSearchQuery),
        );

  return (
    <section aria-labelledby="history-heading">
      <div className="mb-4 flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h2 id="history-heading" className="text-xl font-semibold">
            Disbursement history
          </h2>
          <p className="mt-1 text-sm text-muted-foreground">
            Newest first. Pending batches refresh automatically.
          </p>
        </div>
        <Button
          variant="outline"
          size="sm"
          onClick={onRefresh}
          disabled={isRefreshing}
        >
          <RefreshCw
            aria-hidden="true"
            className={cn(isRefreshing && "animate-spin")}
          />
          Refresh
        </Button>
      </div>

      <label className="relative mb-4 block max-w-md">
        <span className="sr-only">Search history by batch ID</span>
        <Search
          aria-hidden="true"
          className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground"
        />
        <input
          type="search"
          value={searchQuery}
          onChange={(event) => setSearchQuery(event.target.value)}
          placeholder="Search by batch ID"
          className="h-10 w-full rounded-lg border bg-white pr-3 pl-9 text-sm shadow-sm outline-none transition focus:border-ring focus:ring-3 focus:ring-ring/20"
        />
      </label>

      {filteredBatches.length === 0 ? (
        <Card className="border-dashed bg-white/75 py-10 text-center">
          <CardContent>
            <p className="font-semibold">No batches match this ID</p>
            <p className="mt-2 text-sm text-muted-foreground">
              Check the batch ID or clear the search to see every batch.
            </p>
            <Button
              variant="outline"
              size="sm"
              className="mt-4"
              onClick={() => setSearchQuery("")}
            >
              Clear search
            </Button>
          </CardContent>
        </Card>
      ) : (
        <Card className="overflow-hidden border-black/5 shadow-lg shadow-black/5">
          <CardContent className="p-0">
            <div>
              <table className="w-full text-left">
                <thead className="hidden border-b bg-muted/35 text-xs uppercase tracking-wider text-muted-foreground sm:table-header-group">
                  <tr>
                    <th scope="col" className="px-5 py-4 font-medium">
                      Batch
                    </th>
                    <th scope="col" className="px-5 py-4 font-medium">
                      Total
                    </th>
                    <th scope="col" className="px-5 py-4 font-medium">
                      Results
                    </th>
                    <th scope="col" className="px-5 py-4">
                      <span className="sr-only">Actions</span>
                    </th>
                  </tr>
                </thead>
                <tbody className="block divide-y sm:table-row-group">
                  {filteredBatches.map((batch) => (
                    <HistoryBatchRow
                      key={batch.batch_id}
                      batch={batch}
                      isActive={detailBatchID === batch.batch_id}
                      onToggle={() => onToggleBatch(batch.batch_id)}
                    />
                  ))}
                </tbody>
              </table>
            </div>
          </CardContent>
        </Card>
      )}
    </section>
  );
}

function HistoryBatchRow({
  batch,
  isActive,
  onToggle,
}: {
  batch: BatchSnapshot;
  isActive: boolean;
  onToggle: () => void;
}) {
  const counts = countResults(batch);
  const totals = Array.from(totalsByCurrency(batch.results));

  return (
    <tr
      className={cn(
        "block p-5 sm:table-row sm:p-0",
        isActive && "bg-primary/5",
      )}
    >
      <td className="block p-0 align-top sm:table-cell sm:px-5 sm:py-5">
        <p className="break-all font-mono text-sm text-foreground sm:break-normal">
          {batch.batch_id}
        </p>
        <time
          dateTime={batch.created_at}
          className="mt-1 block text-xs text-muted-foreground"
        >
          {formatHistoryTime(batch.created_at)}
        </time>
      </td>
      <td className="mt-4 block p-0 align-top sm:table-cell sm:px-5 sm:py-5">
        <p className="mb-1 text-xs font-medium uppercase tracking-wider text-muted-foreground sm:hidden">
          Total
        </p>
        <div className="space-y-1 font-medium tabular-nums">
          {totals.map(([currency, amount]) => (
            <p key={currency}>{formatMinorUnits(amount, currency)}</p>
          ))}
        </div>
      </td>
      <td className="mt-4 block p-0 align-top text-sm sm:table-cell sm:px-5 sm:py-5">
        <p className="mb-1 text-xs font-medium uppercase tracking-wider text-muted-foreground sm:hidden">
          Results
        </p>
        {counts.pending > 0 ? (
          <p className="font-medium text-status-warning">
            {formatCount(counts.pending, "pending")}
          </p>
        ) : null}
        <p className={cn(counts.pending > 0 && "mt-1")}>
          {formatCount(counts.success, "succeeded")},{" "}
          {formatCount(counts.failed, "failed")}
        </p>
      </td>
      <td className="block p-0 text-right align-top sm:table-cell sm:px-5 sm:py-5">
        <Button
          variant="outline"
          size="sm"
          className="mt-4 w-full sm:mt-0 sm:w-auto"
          onClick={onToggle}
          aria-expanded={isActive}
        >
          {isActive ? "Hide details" : "Show details"}
        </Button>
      </td>
    </tr>
  );
}

function countResults(batch: BatchSnapshot) {
  const counts = { pending: 0, success: 0, failed: 0 };
  for (const result of batch.results) {
    counts[result.status]++;
  }
  return counts;
}

function formatCount(count: number, label: string): string {
  return `${count} ${label}`;
}

function HistoryLoadingState() {
  return (
    <Card className="border-black/5 shadow-xl shadow-black/5">
      <CardContent className="p-6">
        <p className="mb-5 text-sm font-medium text-muted-foreground">
          Loading disbursement history…
        </p>
        <div className="space-y-3" aria-hidden="true">
          {[0, 1, 2].map((row) => (
            <Skeleton key={row} className="h-24 w-full rounded-xl" />
          ))}
        </div>
      </CardContent>
    </Card>
  );
}

function formatHistoryTime(timestamp: string): string {
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(new Date(timestamp));
}
