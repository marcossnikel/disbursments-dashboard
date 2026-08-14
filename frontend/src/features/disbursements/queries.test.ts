import { describe, expect, it } from "vitest";

import { nextBatchRefreshInterval } from "@/features/disbursements/queries";

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
