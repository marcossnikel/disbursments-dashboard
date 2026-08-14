import { describe, expect, it } from "vitest";

import {
  formatMoney,
  totalsByCurrency,
} from "@/features/disbursements/formatMoney";

describe("money formatting", () => {
  it("preserves values beyond JavaScript's safe integer range", () => {
    expect(formatMoney("90071992547409.91", "USD")).toBe(
      "$90,071,992,547,409.91",
    );
  });

  it("keeps totals separate by currency", () => {
    const totals = totalsByCurrency([
      { id: "wrk_001", name: "Ada", amount: "10.10", currency: "USD" },
      { id: "wrk_002", name: "Linus", amount: "20.20", currency: "EUR" },
      { id: "wrk_003", name: "Grace", amount: "0.30", currency: "USD" },
    ]);

    expect(totals.get("USD")).toBe(1_040n);
    expect(totals.get("EUR")).toBe(2_020n);
  });
});
