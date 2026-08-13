import {
  CheckCircle2,
  CircleAlert,
  Info,
  LoaderCircle,
  RotateCcw,
  RefreshCw,
  TriangleAlert,
} from "lucide-react";

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
    description: "The provider confirmed that this payment was not completed.",
    className: "bg-status-danger-soft text-status-danger",
    icon: CircleAlert,
  },
  outcome_unknown: {
    label: "Payment outcome unknown",
    description:
      "The provider timed out before confirming the result. Retry is disabled to prevent paying this worker twice.",
    className: "bg-status-info-soft text-status-info",
    icon: TriangleAlert,
  },
} as const;

export function BatchProgress({
  batch,
  refreshFailed,
  isRefreshing,
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
    <Card className="mb-6 overflow-hidden border-0 bg-secondary text-secondary-foreground shadow-2xl shadow-black/15">
      <CardContent className="p-0">
        <div className="flex flex-col gap-5 border-b border-white/10 px-5 py-5 sm:px-6">
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
            <div className="mb-2 flex items-center justify-between text-xs text-white/55">
              <span>{completedCount} terminal results</span>
              <span>{completionPercentage}%</span>
            </div>
            <Progress
              value={completionPercentage}
              className="h-1.5 bg-white/10"
            />
          </div>

          <dl className="grid grid-cols-2 gap-2 sm:grid-cols-4">
            <SummaryCount label="Pending" value={counts.pending} />
            <SummaryCount label="Succeeded" value={counts.success} />
            <SummaryCount label="Failed" value={counts.failed} />
            <SummaryCount
              label="Outcome unknown"
              value={counts.outcome_unknown}
            />
          </dl>
        </div>

        {refreshFailed ? (
          <div className="border-b border-white/10 px-5 py-4 sm:px-6">
            <Alert className="border-status-warning/20 bg-status-warning/10 text-white">
              <RefreshCw
                aria-hidden="true"
                className={cn(isRefreshing && "animate-spin")}
              />
              <AlertTitle>The latest refresh failed</AlertTitle>
              <AlertDescription className="flex flex-col gap-2 text-white/65 sm:flex-row sm:items-center sm:justify-between">
                <span>
                  The results below are the last confirmed snapshot. Processing
                  may still continue.
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

        <div className="divide-y divide-white/10">
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
                    <p className="font-medium text-white">
                      {result.worker_name}
                    </p>
                    <Badge className={cn("rounded-full", details.className)}>
                      <StatusIcon
                        aria-hidden="true"
                        className={cn(
                          "size-3",
                          result.status === "pending" && "animate-spin",
                        )}
                      />
                      {details.label}
                    </Badge>
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <Button
                          variant="ghost"
                          size="icon-xs"
                          className="text-white/45 hover:bg-white/10 hover:text-white"
                          aria-label={`About ${details.label}`}
                        >
                          <Info aria-hidden="true" />
                        </Button>
                      </TooltipTrigger>
                      <TooltipContent>{details.description}</TooltipContent>
                    </Tooltip>
                  </div>
                  <div className="flex flex-wrap gap-x-3 gap-y-1 font-mono text-[0.7rem] text-white/40">
                    <span>Worker {result.worker_id}</span>
                    <span>Disbursement {result.disbursement_id}</span>
                  </div>
                  {result.provider_transaction_id ? (
                    <p className="mt-2 font-mono text-xs text-white/65">
                      Provider {result.provider_transaction_id}
                    </p>
                  ) : null}
                  {result.error_message ? (
                    <p className="mt-2 text-sm text-white/65">
                      {result.error_message}
                    </p>
                  ) : null}
                </div>
                <div className="flex items-center justify-between gap-3 sm:flex-col sm:items-end">
                  <p className="font-semibold text-white tabular-nums">
                    {formatMoney(result.amount, result.currency)}
                  </p>
                  <p className="text-xs text-white/40">{result.currency}</p>
                  {result.status === "failed" ? (
                    <Button
                      variant="outline"
                      size="sm"
                      className="mt-1 border-white/15 bg-white/5 text-white hover:bg-white/10 hover:text-white"
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

function SummaryCount({ label, value }: { label: string; value: number }) {
  return (
    <div className="rounded-xl border border-white/10 bg-white/5 px-3 py-3">
      <dt className="text-[0.7rem] text-white/45">{label}</dt>
      <dd className="mt-1 text-xl font-semibold text-white tabular-nums">
        {value}
      </dd>
    </div>
  );
}

function countStatuses(batch: BatchSnapshot) {
  const counts = { pending: 0, success: 0, failed: 0, outcome_unknown: 0 };
  for (const result of batch.results) {
    counts[result.status]++;
  }
  return counts;
}
