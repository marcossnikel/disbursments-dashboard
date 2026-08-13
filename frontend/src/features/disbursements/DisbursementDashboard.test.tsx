import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { TooltipProvider } from "@/components/ui/tooltip";
import { DisbursementDashboard } from "@/features/disbursements/DisbursementDashboard";

const workers = [
  { id: "w-001", name: "Ada Lovelace", amount: "1500.50", currency: "USD" },
  { id: "w-002", name: "Linus Torvalds", amount: "2300.00", currency: "EUR" },
] as const;

describe("DisbursementDashboard", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("lets an operator review selected workers and exact currency totals", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(JSON.stringify(workers), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      ),
    );
    const user = userEvent.setup();

    renderDashboard();

    expect(screen.getByText("Loading pending disbursements…")).toBeInTheDocument();
    await screen.findByText("Ada Lovelace");

    await user.click(screen.getByRole("checkbox", { name: "Select Ada Lovelace" }));
    await user.click(screen.getByRole("checkbox", { name: "Select Linus Torvalds" }));
    await user.click(screen.getByRole("button", { name: "Disburse 2 workers" }));

    const dialog = screen.getByRole("dialog", { name: "Confirm this batch" });
    expect(within(dialog).getByText("Ada Lovelace")).toBeInTheDocument();
    expect(within(dialog).getByText("Linus Torvalds")).toBeInTheDocument();
    expect(within(dialog).getAllByText("$1,500.50")).toHaveLength(2);
    expect(within(dialog).getAllByText("€2,300.00")).toHaveLength(2);
    expect(
      within(dialog).getByRole("button", { name: "Confirm and disburse" }),
    ).toBeEnabled();
  });

  it("shows pending work immediately and polls until every result is terminal", async () => {
    let batchReadCount = 0;
    vi.stubGlobal(
      "fetch",
      vi.fn(async (request: Request) => {
        const url = new URL(request.url);
        if (request.method === "GET" && url.pathname === "/workers") {
          return jsonResponse(workers);
        }
        if (request.method === "POST" && url.pathname === "/disbursements") {
          const body = (await request.json()) as { batch_id: string };
          return jsonResponse({ batch_id: body.batch_id }, { status: 202 });
        }
        if (request.method === "GET" && url.pathname.startsWith("/disbursements/")) {
          batchReadCount++;
          const batchID = url.pathname.replace("/disbursements/", "");
          return jsonResponse(
            batchReadCount === 1
              ? {
                  batch_id: batchID,
                  status: "processing",
                  results: [
                    {
                      disbursement_id: "disb-001",
                      worker_id: "w-001",
                      worker_name: "Ada Lovelace",
                      amount: "1500.50",
                      currency: "USD",
                      status: "pending",
                    },
                  ],
                }
              : {
                  batch_id: batchID,
                  status: "completed",
                  results: [
                    {
                      disbursement_id: "disb-001",
                      worker_id: "w-001",
                      worker_name: "Ada Lovelace",
                      amount: "1500.50",
                      currency: "USD",
                      status: "success",
                      provider_transaction_id: "ptx-001",
                    },
                  ],
                },
          );
        }
        return new Response(null, { status: 404 });
      }),
    );
    const user = userEvent.setup();

    renderDashboard();
    await screen.findByText("Ada Lovelace");
    await user.click(screen.getByRole("checkbox", { name: "Select Ada Lovelace" }));
    await user.click(screen.getByRole("button", { name: "Disburse 1 worker" }));
    await user.click(screen.getByRole("button", { name: "Confirm and disburse" }));

    expect(await screen.findByText("Pending")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Copy batch ID" })).toBeInTheDocument();
    expect(await screen.findByText("Success", {}, { timeout: 2_000 })).toBeInTheDocument();
    expect(screen.getByText("Provider ptx-001")).toBeInTheDocument();
    expect(screen.getByText("1", { selector: "dd" })).toBeInTheDocument();
  });
});

function renderDashboard() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });

  return render(
    <QueryClientProvider client={queryClient}>
      <TooltipProvider>
        <DisbursementDashboard />
      </TooltipProvider>
    </QueryClientProvider>,
  );
}

function jsonResponse(body: unknown, init?: ResponseInit) {
  return new Response(JSON.stringify(body), {
    status: 200,
    ...init,
    headers: { "Content-Type": "application/json", ...init?.headers },
  });
}
