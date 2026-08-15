import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { TooltipProvider } from "@/components/ui/tooltip";
import { DisbursementDashboard } from "@/features/disbursements/DisbursementDashboard";

const workers = [
  { id: "w-001", name: "Maya Thompson", amount: "1500.50", currency: "USD" },
  { id: "w-002", name: "Daniel Kim", amount: "2300.00", currency: "EUR" },
] as const;

describe("DisbursementDashboard", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    window.history.replaceState(null, "", "/");
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

    expect(
      screen.getByText("Loading pending disbursements…"),
    ).toBeInTheDocument();
    await screen.findByText("Maya Thompson");

    await user.click(
      screen.getByRole("checkbox", { name: "Select all workers" }),
    );
    expect(
      screen.getByRole("checkbox", { name: "Select Maya Thompson" }),
    ).toBeChecked();
    expect(
      screen.getByRole("checkbox", { name: "Select Daniel Kim" }),
    ).toBeChecked();
    await user.click(
      screen.getByRole("button", { name: "Disburse 2 workers" }),
    );

    const dialog = screen.getByRole("dialog", { name: "Confirm this batch" });
    expect(within(dialog).getByText("2 workers selected")).toBeInTheDocument();
    expect(within(dialog).getByText("Maya Thompson")).toBeInTheDocument();
    expect(within(dialog).getByText("Daniel Kim")).toBeInTheDocument();
    const batchTotals = within(dialog).getByLabelText("Batch totals");
    expect(within(batchTotals).getByText("$1,500.50")).toBeInTheDocument();
    expect(within(batchTotals).getByText("€2,300.00")).toBeInTheDocument();
    const recipients = within(dialog).getByLabelText("Recipients");
    expect(within(recipients).getByText("$1,500.50")).toBeInTheDocument();
    expect(within(recipients).getByText("€2,300.00")).toBeInTheDocument();
    expect(
      within(dialog).getByRole("button", { name: "Confirm and disburse" }),
    ).toBeEnabled();
  });

  it("shows pending work immediately and polls until every result is terminal", async () => {
    let batchReadCount = 0;
    let historyReadCount = 0;
    let workerReadCount = 0;
    let submittedBatchID = "";
    vi.stubGlobal(
      "fetch",
      vi.fn(async (request: Request) => {
        const url = new URL(request.url);
        if (request.method === "GET" && url.pathname === "/workers") {
          workerReadCount++;
          return jsonResponse(workers);
        }
        if (request.method === "POST" && url.pathname === "/disbursements") {
          const body = (await request.json()) as { batch_id: string };
          submittedBatchID = body.batch_id;
          return jsonResponse({ batch_id: body.batch_id }, { status: 202 });
        }
        if (request.method === "GET" && url.pathname === "/disbursements") {
          historyReadCount++;
          return jsonResponse({}, { status: 405 });
        }
        if (
          request.method === "GET" &&
          url.pathname.startsWith("/disbursements/")
        ) {
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
                      worker_name: "Maya Thompson",
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
                      worker_name: "Maya Thompson",
                      amount: "1500.50",
                      currency: "USD",
                      status: "success",
                      provider_txn_id: "ptx-001",
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
    await screen.findByText("Maya Thompson");
    await user.click(screen.getByRole("tab", { name: "History" }));
    expect(
      await screen.findByText("No disbursement history yet"),
    ).toBeInTheDocument();
    expect(historyReadCount).toBe(1);
    await user.click(screen.getByRole("tab", { name: "New disbursement" }));
    await user.click(
      screen.getByRole("checkbox", { name: "Select Maya Thompson" }),
    );
    await user.click(screen.getByRole("button", { name: "Disburse 1 worker" }));
    await user.click(
      screen.getByRole("button", { name: "Confirm and disburse" }),
    );

    expect(
      screen.getByRole("tab", { name: "New disbursement" }),
    ).toHaveAttribute("aria-selected", "true");
    expect(historyReadCount).toBeGreaterThanOrEqual(1);
    expect(
      await screen.findByLabelText(
        "Pending. The provider call is still in progress. No action is needed.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "About Pending" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Copy batch ID" }),
    ).toBeInTheDocument();
    const liveResults = screen
      .getByRole("heading", { name: "Live payment results" })
      .closest('[data-slot="card"]');
    if (!(liveResults instanceof HTMLElement)) {
      throw new Error("Live payment results card was not rendered");
    }
    expect(
      await within(liveResults).findByText("Success", {}, { timeout: 2_000 }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "About Success" }),
    ).not.toBeInTheDocument();
    await user.hover(within(liveResults).getByText("Success"));
    expect(
      await screen.findByText(
        "The provider confirmed the payment and returned a transaction ID.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("Provider ptx-001")).toBeInTheDocument();
    expect(screen.getByText("1", { selector: "dd" })).toBeInTheDocument();

    await user.click(screen.getByRole("tab", { name: "History" }));
    const history = await screen.findByRole("region", {
      name: "Disbursement history",
    });
    expect(within(history).getByText(submittedBatchID)).toBeInTheDocument();
    expect(within(history).getByText("$1,500.50")).toBeInTheDocument();
    expect(
      within(history).getByText("1 succeeded, 0 failed"),
    ).toBeInTheDocument();
    expect(
      within(history).queryByText("Maya Thompson"),
    ).not.toBeInTheDocument();

    const search = within(history).getByRole("searchbox", {
      name: "Search history by batch ID",
    });
    await user.type(search, submittedBatchID.slice(-6));
    expect(within(history).getByText(submittedBatchID)).toBeInTheDocument();
    await user.clear(search);
    await user.type(search, "batch-that-does-not-exist");
    expect(
      within(history).getByText("No batches match this ID"),
    ).toBeInTheDocument();
    expect(
      within(history).queryByText(submittedBatchID),
    ).not.toBeInTheDocument();
    await user.click(
      within(history).getByRole("button", { name: "Clear search" }),
    );

    await user.click(
      within(history).getByRole("button", { name: "Show details" }),
    );
    expect(await screen.findByText("Maya Thompson")).toBeInTheDocument();
    const hideDetails = within(history).getByRole("button", {
      name: "Hide details",
    });
    expect(hideDetails).toHaveAttribute("aria-expanded", "true");
    await user.click(hideDetails);
    expect(
      screen.queryByRole("heading", { name: "Live payment results" }),
    ).not.toBeInTheDocument();
    expect(
      within(history).getByRole("button", { name: "Show details" }),
    ).toHaveAttribute("aria-expanded", "false");
    await waitFor(() => expect(workerReadCount).toBeGreaterThanOrEqual(3));
  });

  it("shows the same neutral empty state when history is empty or unavailable", async () => {
    let historyResponseStatus = 200;
    vi.stubGlobal(
      "fetch",
      vi.fn(async (request: Request) => {
        const url = new URL(request.url);
        if (url.pathname === "/workers") {
          return jsonResponse(workers);
        }
        if (url.pathname === "/disbursements") {
          return historyResponseStatus === 200
            ? jsonResponse([])
            : jsonResponse({}, { status: historyResponseStatus });
        }
        return new Response(null, { status: 404 });
      }),
    );
    const user = userEvent.setup();

    const firstDashboard = renderDashboard();
    await screen.findByText("Maya Thompson");
    await user.click(screen.getByRole("tab", { name: "History" }));

    expect(
      await screen.findByText("No disbursement history yet"),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("We couldn't load disbursement history"),
    ).not.toBeInTheDocument();

    firstDashboard.unmount();
    historyResponseStatus = 500;
    renderDashboard();
    await screen.findByText("Maya Thompson");
    await user.click(screen.getByRole("tab", { name: "History" }));

    expect(
      await screen.findByText("No disbursement history yet"),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("We couldn't load disbursement history"),
    ).not.toBeInTheDocument();
  });

  it("restores an accepted batch from the URL after a refresh", async () => {
    window.history.replaceState(null, "", "/?batch=batch-restored");
    vi.stubGlobal(
      "fetch",
      vi.fn(async (request: Request) => {
        const url = new URL(request.url);
        if (url.pathname === "/workers") {
          return jsonResponse([]);
        }
        if (url.pathname === "/disbursements/batch-restored") {
          return jsonResponse({
            batch_id: "batch-restored",
            status: "completed",
            results: [
              {
                disbursement_id: "disb-restored",
                worker_id: "w-001",
                worker_name: "Maya Thompson",
                amount: "1500.50",
                currency: "USD",
                status: "success",
                provider_txn_id: "ptx-restored",
              },
            ],
          });
        }
        return new Response(null, { status: 404 });
      }),
    );
    const user = userEvent.setup();

    renderDashboard();

    expect(await screen.findByText("Batch complete")).toBeInTheDocument();
    expect(
      screen.getByRole("tab", { name: "New disbursement" }),
    ).toHaveAttribute("aria-selected", "true");
    expect(screen.getByText("Provider ptx-restored")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /Confirm and disburse/ }),
    ).not.toBeInTheDocument();

    await user.click(screen.getByRole("tab", { name: "History" }));
    const history = await screen.findByRole("region", {
      name: "Disbursement history",
    });
    expect(within(history).getByText("batch-restored")).toBeInTheDocument();
  });

  it("can stop tracking an unavailable batch without implying cancellation", async () => {
    window.history.replaceState(null, "", "/?batch=batch-missing");
    vi.stubGlobal(
      "fetch",
      vi.fn(async (request: Request) => {
        const url = new URL(request.url);
        if (url.pathname === "/workers") {
          return jsonResponse(workers);
        }
        if (url.pathname === "/disbursements/batch-missing") {
          return jsonResponse(
            {
              code: "batch_not_found",
              message: "The requested batch was not found.",
              request_id: "req-missing",
            },
            { status: 404 },
          );
        }
        return new Response(null, { status: 404 });
      }),
    );
    const user = userEvent.setup();

    renderDashboard();
    expect(
      await screen.findByText("We couldn't load batch batch-missing"),
    ).toBeInTheDocument();
    expect(screen.getByText(/does not cancel payments/i)).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Stop tracking" }));

    expect(
      screen.queryByText("We couldn't load batch batch-missing"),
    ).not.toBeInTheDocument();
    expect(window.location.search).toBe("");
  });

  it("explains an unavailable worker and requires a fresh confirmation for the remainder", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async (request: Request) => {
        const url = new URL(request.url);
        if (request.method === "GET" && url.pathname === "/workers") {
          return jsonResponse(workers);
        }
        if (request.method === "POST" && url.pathname === "/disbursements") {
          return jsonResponse(
            {
              code: "workers_unavailable",
              message:
                "No disbursements were started. One or more workers are no longer available.",
              request_id: "req-conflict",
              unavailable_workers: [
                {
                  worker_id: "w-001",
                  worker_name: "Maya Thompson",
                  reason: "already_paid",
                  batch_id: "batch-paid",
                  disbursement_id: "disb-paid",
                },
              ],
            },
            { status: 409, headers: { "X-Request-ID": "req-conflict" } },
          );
        }
        return new Response(null, { status: 404 });
      }),
    );
    const user = userEvent.setup();

    renderDashboard();
    await screen.findByText("Maya Thompson");
    await user.click(
      screen.getByRole("checkbox", { name: "Select Maya Thompson" }),
    );
    await user.click(
      screen.getByRole("checkbox", { name: "Select Daniel Kim" }),
    );
    await user.click(
      screen.getByRole("button", { name: "Disburse 2 workers" }),
    );
    await user.click(
      screen.getByRole("button", { name: "Confirm and disburse" }),
    );

    expect(
      await screen.findByText("No disbursements were started"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Maya Thompson was already paid."),
    ).toBeInTheDocument();
    expect(screen.getByText("Already paid")).toBeInTheDocument();
    expect(screen.queryByText("already_paid")).not.toBeInTheDocument();
    expect(screen.getByText("Batch batch-paid")).toBeInTheDocument();
    expect(screen.getByText("Request req-conflict")).toBeInTheDocument();

    await user.click(
      screen.getByRole("button", { name: "Continue with 1 available worker" }),
    );
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Disburse 1 worker" }),
    ).toBeEnabled();

    await user.click(screen.getByRole("button", { name: "Disburse 1 worker" }));
    const revisedDialog = screen.getByRole("dialog", {
      name: "Confirm this batch",
    });
    expect(within(revisedDialog).getByText("Daniel Kim")).toBeInTheDocument();
    expect(
      within(revisedDialog).queryByText("Maya Thompson"),
    ).not.toBeInTheDocument();
  });

  it("refreshes availability and clears a stale selection when viewing its conflicting payment", async () => {
    let workerReadCount = 0;
    vi.stubGlobal(
      "fetch",
      vi.fn(async (request: Request) => {
        const url = new URL(request.url);
        if (request.method === "GET" && url.pathname === "/workers") {
          workerReadCount++;
          return jsonResponse(workerReadCount === 1 ? workers : [workers[1]]);
        }
        if (request.method === "POST" && url.pathname === "/disbursements") {
          return jsonResponse(
            {
              code: "workers_unavailable",
              message: "No disbursements were started.",
              request_id: "req-stale",
              unavailable_workers: [
                {
                  worker_id: "w-001",
                  worker_name: "Maya Thompson",
                  reason: "already_paid",
                  batch_id: "batch-paid",
                  disbursement_id: "disb-paid",
                },
              ],
            },
            { status: 409 },
          );
        }
        if (
          request.method === "GET" &&
          url.pathname === "/disbursements/batch-paid"
        ) {
          return jsonResponse({
            batch_id: "batch-paid",
            status: "completed",
            results: [
              {
                disbursement_id: "disb-paid",
                worker_id: "w-001",
                worker_name: "Maya Thompson",
                amount: "1500.50",
                currency: "USD",
                status: "success",
                provider_txn_id: "ptx-paid",
              },
            ],
          });
        }
        return new Response(null, { status: 404 });
      }),
    );
    const user = userEvent.setup();

    renderDashboard();
    await screen.findByText("Maya Thompson");
    await user.click(
      screen.getByRole("checkbox", { name: "Select Maya Thompson" }),
    );
    await user.click(screen.getByRole("button", { name: "Disburse 1 worker" }));
    await user.click(
      screen.getByRole("button", { name: "Confirm and disburse" }),
    );
    await user.click(
      await screen.findByRole("button", { name: "View payment details" }),
    );

    expect(await screen.findByText("Batch complete")).toBeInTheDocument();
    await user.click(
      await screen.findByRole("tab", { name: "New disbursement" }),
    );
    expect(
      screen.queryByRole("checkbox", { name: "Select Maya Thompson" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("checkbox", { name: "Select Daniel Kim" }),
    ).not.toBeChecked();
    expect(
      screen.getByRole("button", { name: "Disburse 0 workers" }),
    ).toBeDisabled();
  });

  it("prepares a fresh confirmation only for a confirmed failure", async () => {
    window.history.replaceState(null, "", "/?batch=batch-with-failures");
    vi.stubGlobal(
      "fetch",
      vi.fn(async (request: Request) => {
        const url = new URL(request.url);
        if (url.pathname === "/workers") {
          return jsonResponse([workers[0]]);
        }
        if (url.pathname === "/disbursements/batch-with-failures") {
          return jsonResponse({
            batch_id: "batch-with-failures",
            status: "completed",
            results: [
              {
                disbursement_id: "disb-failed",
                worker_id: "w-001",
                worker_name: "Maya Thompson",
                amount: "1500.50",
                currency: "USD",
                status: "failed",
                error_message: "Provider declined this disbursement.",
              },
              {
                disbursement_id: "disb-success",
                worker_id: "w-002",
                worker_name: "Daniel Kim",
                amount: "2300.00",
                currency: "EUR",
                status: "success",
                provider_txn_id: "ptx-success",
              },
            ],
          });
        }
        return new Response(null, { status: 404 });
      }),
    );
    const user = userEvent.setup();

    renderDashboard();

    expect(await screen.findByText("Daniel Kim")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", {
        name: "Prepare retry for Daniel Kim",
      }),
    ).not.toBeInTheDocument();

    await user.click(
      screen.getByRole("button", { name: "Prepare retry for Maya Thompson" }),
    );

    const retryDialog = await screen.findByRole("dialog", {
      name: "Confirm this batch",
    });
    expect(within(retryDialog).getByText("Maya Thompson")).toBeInTheDocument();
    expect(
      within(retryDialog).queryByText("Daniel Kim"),
    ).not.toBeInTheDocument();
    expect(
      within(retryDialog).getByRole("button", { name: "Confirm and disburse" }),
    ).toBeEnabled();
  });

  it("confirms a demo reset and restores workers when no payments remain", async () => {
    window.history.replaceState(null, "", "/?batch=batch-finished");
    let demoWasReset = false;
    vi.stubGlobal(
      "fetch",
      vi.fn(async (request: Request) => {
        const url = new URL(request.url);
        if (request.method === "GET" && url.pathname === "/workers") {
          return jsonResponse(demoWasReset ? [workers[0]] : []);
        }
        if (
          request.method === "GET" &&
          url.pathname === "/disbursements/batch-finished"
        ) {
          return jsonResponse({
            batch_id: "batch-finished",
            status: "completed",
            results: [],
          });
        }
        if (request.method === "POST" && url.pathname === "/demo/reset") {
          demoWasReset = true;
          return new Response(null, { status: 204 });
        }
        return new Response(null, { status: 404 });
      }),
    );
    const user = userEvent.setup();

    renderDashboard();

    await user.click(
      await screen.findByRole("tab", { name: "New disbursement" }),
    );
    expect(
      await screen.findByText("No pending disbursements"),
    ).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Reset demo data" }));

    const resetDialog = screen.getByRole("dialog", {
      name: "Reset demo data?",
    });
    expect(
      within(resetDialog).getByText(/clears every in-memory batch/i),
    ).toBeInTheDocument();
    await user.click(
      within(resetDialog).getByRole("button", {
        name: "Reset and restore workers",
      }),
    );

    expect(await screen.findByText("Maya Thompson")).toBeInTheDocument();
    expect(screen.queryByText("Live payment results")).not.toBeInTheDocument();
    expect(window.location.search).toBe("");
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
