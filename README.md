# Cadana Disbursement Console

An internal operations dashboard for selecting pending worker payments, starting an asynchronous batch, and following every result without losing the identifiers needed to investigate it.

## Run locally

Prerequisites: Go 1.25+, Node.js 22.12+, and pnpm 11.

Install frontend dependencies once:

```bash
cd frontend && pnpm install
```

From the repository root, run each app in its own terminal:

```bash
make backend
```

```bash
make frontend
```

Open [http://localhost:5173](http://localhost:5173). The API defaults to `http://localhost:8080`; `API_ADDRESS`, `FRONTEND_ORIGIN`, and `VITE_API_URL` can override the local defaults.

## Verify

```bash
make verify
```

That command runs Go tests with the race detector, Go vet, frontend interaction and money tests, ESLint, Prettier, TypeScript, production builds, and a generated-contract drift check. `make test` is the shorter test-only command.

## Payment safety model

The payment attempt and the worker's payment obligation are related, but they are not the same lifecycle:

| Attempt result | Obligation after the result    | Operator action                          |
| -------------- | ------------------------------ | ---------------------------------------- |
| `pending`      | Reserved by the accepted batch | Wait and keep polling                    |
| `success`      | Paid                           | Never retry                              |
| `failed`       | Available again                | Prepare a new batch and confirm it again |

A batch is reserved atomically: if any selected worker is unavailable, no provider call begins. The conflict response includes the worker, prior batch, disbursement, reason, and request IDs so the UI can explain exactly what happened rather than silently dropping a payment.

## Key design decisions

1. React and TypeScript keep the live-review surface in a framework I can explain fluently, while Go owns the payment rules and process lifecycle.
2. `api/openapi.yaml` is the transport contract, and generated Go and TypeScript types prevent the two applications from inventing different payloads.
3. Money enters the API as a canonical decimal string and becomes a checked `int64` count of minor units plus currency in Go; the frontend uses `bigint`, so neither side performs monetary arithmetic with floating point.
4. The processor locks only long enough to validate and reserve the full batch, then calls the provider concurrently outside the lock so partial failures remain independent.
5. A replay of the same batch ID and canonical worker set returns the existing batch, while the same ID with different workers returns `409` and starts nothing.
6. The exercise treats every simulated provider error as a terminal failure and releases the obligation for a newly confirmed retry; in production, an ambiguous network timeout would require provider idempotency and reconciliation before retrying.
7. TanStack Query is the single owner of server state and polling, while local React state holds only the current selection and dialog feedback.
8. Structured JSON access logs carry request ID, status, and duration, while payment lifecycle logs carry batch, disbursement, worker, provider transaction, status, and error identifiers.

## Trade-off

- State is deliberately process-local to keep this exercise readable. A production implementation would durably persist idempotency records and payment obligations before introducing queues, multiple instances, or automatic recovery.

## Repository map

```text
api/openapi.yaml                 shared HTTP contract
backend/cmd/api                  composition, signals, and server lifecycle
backend/internal/demodata        reviewer-facing worker fixtures
backend/internal/disbursement    money, obligations, batches, and concurrency
backend/internal/httpserver      HTTP handlers, middleware, and wire translation
backend/internal/mockpayment     flaky 50–200 ms provider
frontend/src/api                 generated types and typed client
frontend/src/features/disbursements
                                 dashboard behavior and feature components
frontend/src/components/ui       shared shadcn primitives
```

The backend logs one JSON object per lifecycle event to stdout. A successful result includes its provider transaction ID; client-visible failures also include an `X-Request-ID` for correlation.
