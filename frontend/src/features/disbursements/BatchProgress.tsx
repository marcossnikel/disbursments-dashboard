import CheckCircle2 from "lucide-react/dist/esm/icons/check-circle-2.mjs";
import CircleAlert from "lucide-react/dist/esm/icons/circle-alert.mjs";
import LoaderCircle from "lucide-react/dist/esm/icons/loader-circle.mjs";
import RefreshCw from "lucide-react/dist/esm/icons/refresh-cw.mjs";
import RotateCcw from "lucide-react/dist/esm/icons/rotate-ccw.mjs";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { BatchIDCopyButton } from "@/features/disbursements/BatchIDCopyButton";
import { formatMoney } from "@/features/disbursements/formatMoney";
import type { BatchSnapshot } from "@/features/disbursements/queries";
import { cn } from "@/lib/utils";

type BatchProgressProps = {
  batch: BatchSnapshot;
  refreshFailed: boolean;
  isRefreshing: boolean;
  lastUpdatedAt: number;
  onRefresh: () => void;
  retryingWorkerID: string | null;
  onPrepareRetry: (workerID: string) => void;
};
const statusDetails = {
  pending: {
    label: "Pending",
    description: "The provider call is still in progress. No action is needed.",
    className: "bg-status-warning-soft text-status-warning",
    icon: LoaderCircle,
  },
  success: {
    label: "Success",
    description:
      "The provider confirmed the payment and returned a transaction ID.",
    className: "bg-status-success-soft text-status-success",
    icon: CheckCircle2,
  },
  failed: {
    label: "Failed",
    description:
      "The provider returned a terminal error and no transaction ID. You can prepare a new confirmed batch.",
    className: "bg-status-danger-soft text-status-danger",
    icon: CircleAlert,
  },
} as const;

export function BatchProgress({
  batch,
  refreshFailed,
  isRefreshing,
  lastUpdatedAt,
  onRefresh,
  retryingWorkerID,
  onPrepareRetry,
}: BatchProgressProps) {
  const counts = countStatuses(batch);
  const completedCount = batch.results.length - counts.pending;
  const completionPercentage =
    batch.results.length === 0
      ? 0
      : Math.round((completedCount / batch.results.length) * 100);

  return (
    <Card className="mb-6 overflow-hidden border-black/5 bg-white shadow-xl shadow-black/5">
      <CardContent className="p-0">
        <div className="flex flex-col gap-5 border-b bg-accent/35 px-5 py-5 sm:px-6">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
            <div>
              <div className="mb-2 flex items-center gap-2 text-sm font-medium text-primary">
                {batch.status === "processing" ? (
                  <LoaderCircle
                    aria-hidden="true"
                    className="size-4 animate-spin"
                  />
                ) : (
                  <CheckCircle2 aria-hidden="true" className="size-4" />
                )}
                {batch.status === "processing"
                  ? "Batch processing"
                  : "Batch complete"}
              </div>
              <h2 className="text-2xl font-semibold tracking-tight">
                Live payment results
              </h2>
            </div>
            <BatchIDCopyButton batchID={batch.batch_id} />
          </div>

          <div>
            <div className="mb-2 flex items-center justify-between text-xs text-muted-foreground">
              <span>{completedCount} terminal results</span>
              <span>{completionPercentage}%</span>
            </div>
            <Progress
              value={completionPercentage}
              aria-label="Batch completion"
              className="h-1.5 bg-primary/10"
            />
          </div>

          <dl className="grid grid-cols-3 gap-2">
            <SummaryCount label="Pending" value={counts.pending} />
            <SummaryCount label="Succeeded" value={counts.success} />
            <SummaryCount label="Failed" value={counts.failed} />
          </dl>
        </div>

        {refreshFailed ? (
          <div className="border-b px-5 py-4 sm:px-6">
            <Alert className="border-status-warning/20 bg-status-warning-soft">
              <RefreshCw
                aria-hidden="true"
                className={cn(isRefreshing && "animate-spin")}
              />
              <AlertTitle>The latest refresh failed</AlertTitle>
              <AlertDescription className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                <span>
                  The results below were last confirmed at{" "}
                  <time dateTime={new Date(lastUpdatedAt).toISOString()}>
                    {formatUpdateTime(lastUpdatedAt)}
                  </time>
                  . Processing may still continue.
                </span>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={onRefresh}
                  disabled={isRefreshing}
                >
                  Refresh now
                </Button>
              </AlertDescription>
            </Alert>
          </div>
        ) : null}

        <div className="divide-y">
          {batch.results.map((result) => {
            const details = statusDetails[result.status];
            const StatusIcon = details.icon;
            return (
              <div
                key={result.disbursement_id}
                className="grid gap-4 px-5 py-4 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center sm:px-6"
              >
                <div className="min-w-0">
                  <div className="mb-1 flex flex-wrap items-center gap-2">
                    <p className="font-medium">{result.worker_name}</p>
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <Badge
                          tabIndex={0}
                          aria-label={`${details.label}. ${details.description}`}
                          className={cn(
                            "cursor-help rounded-full outline-none focus-visible:ring-2 focus-visible:ring-ring/50",
                            details.className,
                          )}
                        >
                          <StatusIcon
                            aria-hidden="true"
                            className={cn(
                              "size-3",
                              result.status === "pending" && "animate-spin",
                            )}
                          />
                          {details.label}
                        </Badge>
                      </TooltipTrigger>
                      <TooltipContent className="max-w-72">
                        {details.description}
                      </TooltipContent>
                    </Tooltip>
                  </div>
                  <div className="flex flex-wrap gap-x-3 gap-y-1 font-mono text-[0.7rem] text-muted-foreground">
                    <span>Worker {result.worker_id}</span>
                    <span>Disbursement {result.disbursement_id}</span>
                  </div>
                  {result.provider_txn_id ? (
                    <p className="mt-2 font-mono text-xs text-muted-foreground">
                      Provider {result.provider_txn_id}
                    </p>
                  ) : null}
                  {result.error_message ? (
                    <p className="mt-2 text-sm text-muted-foreground">
                      {result.error_message}
                    </p>
                  ) : null}
                </div>
                <div className="flex items-center justify-between gap-3 sm:flex-col sm:items-end">
                  <p className="font-semibold tabular-nums">
                    {formatMoney(result.amount, result.currency)}
                  </p>
                  <p className="text-xs text-muted-foreground">
                    {result.currency}
                  </p>
                  {result.status === "failed" ? (
                    <Button
                      variant="outline"
                      size="sm"
                      className="mt-1"
                      aria-label={`Prepare retry for ${result.worker_name}`}
                      disabled={retryingWorkerID !== null}
                      onClick={() => onPrepareRetry(result.worker_id)}
                    >
                      <RotateCcw
                        aria-hidden="true"
                        className={cn(
                          retryingWorkerID === result.worker_id &&
                            "animate-spin",
                        )}
                      />
                      {retryingWorkerID === result.worker_id
                        ? "Preparing…"
                        : "Prepare retry"}
                    </Button>
                  ) : null}
                </div>
              </div>
            );
          })}
        </div>
      </CardContent>
    </Card>
  );
}

function formatUpdateTime(timestamp: number): string {
  return new Intl.DateTimeFormat(undefined, {
    hour: "numeric",
    minute: "2-digit",
    second: "2-digit",
  }).format(timestamp);
}

function SummaryCount({ label, value }: { label: string; value: number }) {
  return (
    <div className="rounded-xl border bg-white/80 px-3 py-3">
      <dt className="text-[0.7rem] text-muted-foreground">{label}</dt>
      <dd className="mt-1 text-xl font-semibold tabular-nums">{value}</dd>
    </div>
  );
}

function countStatuses(batch: BatchSnapshot) {
  const counts = { pending: 0, success: 0, failed: 0 };
  for (const result of batch.results) {
    counts[result.status]++;
  }
  return counts;
}
