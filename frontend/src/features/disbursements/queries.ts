import { queryOptions } from "@tanstack/react-query";

import type { components } from "@/api/generated/schema";
import { apiClient, apiError } from "@/api/client";

export type Worker = components["schemas"]["Worker"];
export type BatchSnapshot = components["schemas"]["BatchSnapshot"];

export type SubmitBatchInput = {
  batchID: string;
  workerIDs: string[];
};

export const workersQueryKey = ["workers"] as const;

export function workersQueryOptions() {
  return queryOptions({
    queryKey: workersQueryKey,
    queryFn: listWorkers,
  });
}

export function batchQueryOptions(batchID: string | null) {
  return queryOptions({
    queryKey: ["disbursement-batch", batchID],
    queryFn: () => getBatch(batchID ?? ""),
    enabled: batchID !== null,
    refetchInterval: (query) =>
      query.state.data?.status === "completed" ? false : 400,
  });
}

export async function submitBatch({
  batchID,
  workerIDs,
}: SubmitBatchInput): Promise<{ batch_id: string }> {
  const { data, error, response } = await apiClient.POST("/disbursements", {
    body: {
      batch_id: batchID,
      worker_ids: workerIDs,
    },
  });
  if (!response.ok || !data) {
    throw apiError(response, error);
  }
  return data;
}

export function createBatchID(): string {
  return `batch-${globalThis.crypto.randomUUID()}`;
}

async function listWorkers(): Promise<Worker[]> {
  const { data, error, response } = await apiClient.GET("/workers");
  if (!response.ok || !data) {
    throw apiError(response, error);
  }
  return data;
}

async function getBatch(batchID: string): Promise<BatchSnapshot> {
  const { data, error, response } = await apiClient.GET("/disbursements/{batch_id}", {
    params: { path: { batch_id: batchID } },
  });
  if (!response.ok || !data) {
    throw apiError(response, error);
  }
  return data;
}
