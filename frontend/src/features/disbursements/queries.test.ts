import { describe, expect, it } from "vitest";

import {
  nextBatchRefreshInterval,
  nextHistoryRefreshInterval,
  type BatchSnapshot,
} from "@/features/disbursements/queries";

describe("batch polling", () => {
  it("polls processing and not-yet-loaded batches", () => {
    expect(nextBatchRefreshInterval(undefined, false)).toBe(400);
    expect(nextBatchRefreshInterval("processing", false)).toBe(400);
  });

  it("waits for operator action after an error or terminal result", () => {
    expect(nextBatchRefreshInterval(undefined, true)).toBe(false);
    expect(nextBatchRefreshInterval("processing", true)).toBe(false);
    expect(nextBatchRefreshInterval("completed", false)).toBe(false);
  });
});

describe("history polling", () => {
  const completedBatch = {
    batch_id: "batch-completed",
    created_at: "2026-08-15T12:00:00Z",
    status: "completed",
    results: [],
  } satisfies BatchSnapshot;

  it("polls while any historical batch is still processing", () => {
    expect(
      nextHistoryRefreshInterval(
        [{ ...completedBatch, status: "processing" }, completedBatch],
        false,
      ),
    ).toBe(400);
  });

  it("stops after every batch completes or a refresh fails", () => {
    expect(nextHistoryRefreshInterval(undefined, false)).toBe(false);
    expect(nextHistoryRefreshInterval([completedBatch], false)).toBe(false);
    expect(nextHistoryRefreshInterval([completedBatch], true)).toBe(false);
  });
});
