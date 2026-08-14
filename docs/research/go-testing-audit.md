# Go testing audit

This audit compares the handwritten backend tests with first-party Go guidance. It focuses on readability, table-driven coverage, failure messages, helpers, cleanup, and parallel execution. Generated OpenAPI code is out of scope.

**Status:** The apply-now recommendations below are implemented in the current test suite. They remain documented to make the boundary between table-driven checks and standalone lifecycle scenarios explicit.

## Executive conclusion

The suite should use more table-driven tests, but converting every test would be less idiomatic, not more. The Go test guidance recommends tables when many cases share the same testing logic and recommends separate test functions when cases require different setup or assertions. The concurrency, idempotency, asynchronous lifecycle, and demo-reset tests are narrative scenarios with meaningful ordering; keeping them as standalone tests makes the payment invariants easier to follow. [Go Test Comments: table-driven tests versus multiple functions](https://go.dev/wiki/TestComments#table-driven-tests-vs-multiple-test-functions)

The current suite already gets several fundamentals right: exact-money acceptance and rejection cases use tables, malformed JSON cases use named subtests, shared helpers call `t.Helper`, HTTP server cleanup uses `t.Cleanup`, failure messages generally put `got` before `want`, and independent top-level tests use `t.Parallel`. These choices match the official testing documentation and Go review guidance. [testing package](https://pkg.go.dev/testing) [Go Code Review Comments: useful test failures](https://go.dev/wiki/CodeReviewComments#useful-test-failures)

## Apply now

### 1. Table-drive worker construction validation

Add `TestNewWorkerRejectsInvalidInputs` in `worker_test.go` with one fresh case per invalid input:

- blank worker ID;
- whitespace-only worker ID;
- blank name;
- whitespace-only name;
- zero-value money.

Every case calls `NewWorker` and checks `errors.Is(err, ErrInvalidWorker)`, so a table removes copy-and-paste without adding conditional branches. Keep a separate success test because it checks returned fields rather than error semantics. This follows the Go recommendation to separate valid-output and error-output tables when their assertions differ. [Go Test Comments: table-driven tests versus multiple functions](https://go.dev/wiki/TestComments#table-driven-tests-vs-multiple-test-functions)

### 2. Table-drive processor configuration validation

Add `TestNewProcessorRejectsInvalidConfiguration` with cases for:

- no workers;
- nil provider;
- zero provider timeout;
- negative provider timeout;
- duplicate worker IDs.

Each case should construct only the relevant input and assert `errors.Is(err, ErrInvalidProcessor)`. A `mutate` function per row would make the table harder to read than explicit `workers` and `config` fields; prefer complete inputs even when they repeat a little. Each table entry should be a complete test case with clear inputs and expected results. [Go Wiki: TableDrivenTests](https://go.dev/wiki/TableDrivenTests)

### 3. Consolidate malformed submissions into one table

Replace the single-purpose unknown-worker test with `TestProcessorRejectsInvalidSubmissions`, using a fresh processor and counting provider in each subtest. Cover:

- nil context;
- empty and whitespace-only batch IDs;
- no worker IDs;
- blank worker ID;
- duplicate worker IDs;
- unknown worker ID.

All cases share the same invariant: `Submit` returns `ErrInvalidSubmission`, no provider work starts, and no batch is persisted. Keep these checks uniform across the table instead of adding case-specific callbacks. If a row needs different logic, it belongs in a separate test. Named subtests isolate `t.Fatal` to the failing case and make individual cases runnable with `go test -run`. [Using Subtests and Sub-benchmarks](https://go.dev/blog/subtests) [Go Test Comments: table-driven tests versus multiple functions](https://go.dev/wiki/TestComments#table-driven-tests-vs-multiple-test-functions)

### 4. Table-drive provider result classification through the public behavior

Add `TestProcessorRecordsProviderOutcomes` with a small result-provider stub and cases for:

- success with a provider transaction ID;
- typed provider decline;
- context deadline or cancellation;
- generic provider error;
- nil error with an empty provider transaction ID.

For each case, submit one worker, wait for completion, and compare the result status, error code, and transaction ID. Also assert the obligation is unavailable after success and available after failure. This exercises the same orchestration and result-mapping path for every row while avoiding a direct test of the unexported `classifyProviderError` helper.

Do not fold the existing two-worker partial-failure test into this table. Its purpose is to prove concurrent overlap and independence inside one batch, not merely to classify one result.

### 5. Add a table at the HTTP-to-domain validation boundary

Keep `TestServerRejectsInvalidBatchJSON` for transport-level decoding failures. Add a separate `TestServerRejectsInvalidBatchSubmissions` table for valid JSON that violates domain rules: missing batch ID, no workers, blank worker ID, duplicate worker IDs, and unknown worker ID. Every row should expect HTTP 400, `invalid_request`, a matching request ID in the header and body, and no provider work.

Separating malformed JSON from valid-but-invalid submissions gives each table one assertion policy. Go explicitly recommends separate tables when normal/error classes or checking logic differ. [Go Test Comments: table-driven tests versus multiple functions](https://go.dev/wiki/TestComments#table-driven-tests-vs-multiple-test-functions)

### 6. Improve failure messages while touching those tests

Use function-oriented messages with the input, actual value, and expected value where practical:

```text
Submit(batch ID "batch-empty") error = nil, want ErrInvalidSubmission
NewProcessor() error = nil, want ErrInvalidProcessor
AvailableWorkers() length = 0, want 1
```

Reserve `t.Fatal` for failed setup or when the current subtest cannot continue. Use `t.Error` when later independent comparisons remain useful. The suite already mostly follows this pattern; the main cleanup is replacing generic labels such as `result status`, `worker count`, or `available worker count` with the API operation that produced the value. [Go Test Comments: got before want and identify the function](https://go.dev/wiki/TestComments#got-before-want) [Go Test Comments: keep going](https://go.dev/wiki/TestComments#keep-going)

### 7. Keep helpers small and domain-specific

The existing `testWorkers`, `newTestServer`, request helpers, JSON decoder, and completion polling helpers correctly call `t.Helper`. Keep that pattern and avoid a generic assertion layer. One focused `newTestProcessor(t, provider, workerCount, timeout)` helper could remove repeated constructor boilerplate, but it should only construct the system under test; result assertions should stay visible in each test. The Go guidance recommends marking setup helpers while warning that helper-based assertion libraries can hide the conditions behind a failure. [Go Test Comments: mark test helpers](https://go.dev/wiki/TestComments#mark-test-helpers) [testing.T.Helper](https://pkg.go.dev/testing#T.Helper)

## Keep as standalone scenario tests

These tests tell a stateful story and should not be converted into tables:

- `TestProcessorCreatesProviderWorkOnceForSimultaneousReplays`: proves one atomic reservation under 20 simultaneous submissions and exactly one provider call per worker.
- `TestProcessorExposesConcurrentPendingWorkAndIndependentResults`: proves both calls overlap, the pending snapshot is visible, one result can fail without blocking the other, and worker availability changes correctly.
- `TestProcessorMakesTimedOutWorkerAvailableForANewAttempt`: proves a sequence across two batch IDs.
- `TestProcessorRejectsAnEntireBatchWhenOneWorkerIsPending`: proves all-or-nothing reservation while another batch remains in flight.
- `TestProcessorRejectsChangedWorkersForAnExistingBatchID`: proves the semantic distinction between exact replay and conflicting reuse.
- `TestProcessorResetRestoresWorkersAndClearsCompletedBatches` and `TestServerResetsCompletedDemoStateButRejectsAResetDuringProcessing`: prove ordered before/during/after transitions.
- `TestServerExposesTheAsynchronousBatchLifecycle`: is an end-to-end contract scenario spanning accepted, pending, completed, replay, and conflict states.
- `TestServerExplainsEveryUnavailableWorkerWithoutStartingTheBatch`: requires prior successful state before exercising the rejection response.
- `TestServerListsAvailableWorkersWithARequestID`: checks one coherent response/logging contract rather than repeated inputs.
- `TestProviderHonorsCancellationBeforePayment`: has one deterministic behavior; a one-row table would only add ceremony.

The official Go guidance is explicit that tables become difficult to understand when rows require different conditional logic. A descriptive standalone test is the clearer tool for these flows. [Go Test Comments: table-driven tests versus multiple functions](https://go.dev/wiki/TestComments#table-driven-tests-vs-multiple-test-functions)

## Existing table tests to retain

- `TestParseMoneyPreservesExactMinorUnits` is a good slice-based table: each amount has the same construction and exact-value assertions.
- `TestParseMoneyRejectsValuesOutsideTheSupportedContract` is a valid map-based table. Go notes that unspecified map iteration order can expose accidental dependencies between cases; these rows are independent and run as parallel subtests. [Go Wiki: map-backed table tests](https://go.dev/wiki/TableDrivenTests#using-a-map-to-store-test-cases)
- `TestServerRejectsInvalidBatchJSON` correctly shares setup and checks the same HTTP status/error code for each malformed body.

Use descriptive case names whenever the raw input is not self-explanatory. Subtest names appear in logs and in `go test -run` selectors, so they should remain human-readable. [Go Test Comments: human-readable subtest names](https://go.dev/wiki/TestComments#choose-human-readable-subtest-names)

## Deliberate non-changes

- Do not introduce an assertion library. The Go test guidance prefers ordinary control flow and useful failure messages. [Go Test Comments: assert libraries](https://go.dev/wiki/TestComments#assert-libraries)
- Do not force tables around multi-step concurrency tests.
- Do not duplicate all ten demo rows in the test unless the exact seed list becomes a contractual fixture. The current test appropriately verifies count, representative exact money, and supported-currency coverage without freezing every display name.
- Do not add `t.Parallel` mechanically to stateful subtests that share a processor, provider gate, log buffer, or HTTP server. Parallel subtests pause until their parent returns and require precise resource isolation. [Using Subtests and Sub-benchmarks: control of parallelism](https://go.dev/blog/subtests#TOC_7.)

## Recommended verification

After applying the test refactor:

```bash
cd backend
gofmt -w internal
go vet ./...
go test ./...
go test -race ./...
```

The race run remains essential because the highest-value tests exercise simultaneous replay, concurrent provider calls, snapshots, and reset exclusion. The race detector only finds races in executed paths, so preserving those scenario tests matters more than maximizing the percentage of tests expressed as tables. [Data Race Detector](https://go.dev/doc/articles/race_detector)
