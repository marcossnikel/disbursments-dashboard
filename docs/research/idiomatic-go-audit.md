# Idiomatic Go audit

This audit compares the current backend worktree against first-party Go guidance and the documentation of the libraries used by the backend. It covers handwritten files under `backend/cmd` and `backend/internal`; generated OpenAPI code is reviewed through its generator configuration rather than edited directly.

## Executive conclusion

The backend is already structurally sound Go. Its three meaningful boundaries are clear: `disbursement` owns the business rules, `httpserver` adapts HTTP, and `mockpayment` implements the outbound provider port. The recent split of large files into responsibility-named files improves navigation without manufacturing extra packages. Go explicitly permits one package to span several files and recommends `internal` for non-public supporting packages, so adding Java-style `controller`, `service`, and `repository` layers would make this take-home less idiomatic rather than more professional. [Organizing a Go module](https://go.dev/doc/modules/layout) [Package names](https://go.dev/blog/package-names)

No correctness-breaking Go violation was found. The apply-now improvements are therefore targeted: make generated Go initialisms idiomatic, pass context explicitly instead of storing it in `Processor`, use the Go 1.25 task API with a clearly enforced shutdown ordering, improve a few semantic file/function boundaries, add concise package/API documentation, and add focused tests around money boundaries and strict HTTP decoding.

## Implementation outcome

The apply-now recommendations were implemented on August 14, 2026:

- Generated Go names now preserve `ID` and `IDs` while JSON field names remain unchanged.
- `Submit` receives its context explicitly; the HTTP adapter visibly detaches request cancellation for bounded asynchronous work.
- Payment jobs use `WaitGroup.Go`, and shutdown waits only after HTTP handlers have stopped successfully.
- Demo fixtures live in `internal/demodata`; the reviewer-only reset is isolated in `disbursement/demo.go`.
- Batch submission, obligation state, provider-result processing, middleware, response helpers, and HTTP handlers have responsibility-named files without new architectural layers.
- Exported contracts and package responsibilities have concise documentation.
- Money boundary tests and strict JSON boundary tests were added, and large test files were split by behavior.
- `gofmt`, `go vet`, unit tests, race tests, backend and frontend builds, and generated-contract verification all pass after the refactor.

## Verification snapshot

The following checks passed against the audited worktree:

```text
gofmt -l cmd internal     no output
go vet ./...              passed
go test ./...             passed
go test -race ./...       passed
```

Current statement coverage from `go test -cover ./...` is 86.1% for `disbursement`, 80.0% for `httpserver`, and 70.0% for `mockpayment`. Coverage is supporting evidence, not a target by itself. The race detector is especially relevant here, but it only detects races in paths actually executed; the official guidance recommends exercising realistic concurrent paths, which the simultaneous replay test does. [Data Race Detector](https://go.dev/doc/articles/race_detector)

## Apply-now improvements

### 1. Normalize generated Go initialisms

**Current:** handwritten domain identifiers correctly use `BatchID`, `WorkerID`, and `DisbursementID`, while generated OpenAPI fields use `BatchId`, `WorkerId`, `RequestId`, and `ProviderTxnId`.

**Change:** configure `backend/oapi-codegen.yaml` with:

```yaml
output-options:
  skip-prune: true
  name-normalizer: ToCamelCaseWithInitialisms
```

Then regenerate and update handwritten references to the generated fields. Keep JSON names such as `batch_id` and `provider_txn_id` unchanged because this is only a Go identifier change.

**Why:** Go's initialism convention is `ID`, never `Id`. Generated code is exempt from the handwritten-code rule, so this is not a correctness defect, but the project's pinned generator directly supports the idiomatic normalizer at low cost. [Go Code Review Comments: initialisms](https://go.dev/wiki/CodeReviewComments#initialisms) [oapi-codegen v2.8 name normalizers](https://pkg.go.dev/github.com/oapi-codegen/oapi-codegen/v2@v2.8.0/pkg/codegen#NameNormalizerFunction)

### 2. Make the asynchronous context lifetime explicit

**Current:** `Processor` stores `applicationContext context.Context`, and `NewProcessor` accepts that context.

**Change:** remove the context field and accept context as the first argument to `Submit`:

```go
func (p *Processor) Submit(ctx context.Context, batchID BatchID, workerIDs []WorkerID) (Submission, error)
```

The HTTP handler should pass `request.Context()`. Because a submitted payment intentionally outlives the HTTP response, detach request cancellation once, explicitly, with `context.WithoutCancel(ctx)`, then apply the existing provider timeout inside each job. This preserves contextual values while making the exceptional async lifetime visible at the call site. `WithoutCancel` removes the parent's cancellation and deadline, so the provider timeout remains mandatory. [Contexts and structs](https://go.dev/blog/context-and-structs) [context.WithoutCancel](https://pkg.go.dev/context#WithoutCancel)

Use `InfoContext`, `WarnContext`, or `Log` whenever that job/request context is available. The `slog` documentation recommends passing a context to an output method when one is available. This also makes future request/trace correlation possible without changing domain types. [slog contexts](https://pkg.go.dev/log/slog#hdr-Contexts)

### 3. Use `WaitGroup.Go` and make its lifecycle contract true

**Current:** `prepareSubmission` calls `jobs.Add`, `Submit` starts goroutines, and `processPayment` defers `jobs.Done`.

**Change:** since the module declares Go 1.25, use `p.jobs.Go(func() { ... })`. It couples task registration, goroutine startup, and completion, eliminating the manual `Add`/`Done` protocol. [sync.WaitGroup](https://pkg.go.dev/sync#WaitGroup) [sync.WaitGroup.Go](https://pkg.go.dev/sync#WaitGroup.Go)

The important constraint is not syntactic: a `Go` call made while the counter is zero must happen before `Wait`. Therefore `Processor.Wait` must only run after the HTTP server has successfully stopped accepting and executing submission handlers. If `http.Server.Shutdown` times out, do not pretend a graceful processor drain is established; return the shutdown error (or explicitly force-close before waiting under a separately proven lifecycle). `Shutdown` normally waits for active connections to become idle. [sync.WaitGroup.Go](https://pkg.go.dev/sync#WaitGroup.Go) [http.Server.Shutdown](https://pkg.go.dev/net/http#Server.Shutdown)

Add a short comment to `Wait` documenting that the caller must stop submissions first. The goroutine itself remains bounded by `providerTimeout`, and `Wait` makes its process lifetime observable. Go's review guidance specifically asks code to make goroutine exit conditions clear. [Go Code Review Comments: goroutine lifetimes](https://go.dev/wiki/CodeReviewComments#goroutine-lifetimes)

### 4. Finish the responsibility split without adding packages

Keep the current packages. Within `disbursement`, move submission-only declarations currently left in `batch.go` (`Submission`, unavailable-worker types, and submission error types) into `submission.go`. Keep batch snapshot/storage declarations and `Batch` in `batch.go`.

Refactor `processPayment` into orchestration plus one state-transition helper:

```text
processPayment          provider timeout + orchestration + logging
recordPaymentResult     one locked state transition, returns a snapshot
classifyProviderError   pure error-to-domain-failure translation
```

This makes the mutex boundary visible without scattering the behavior across tiny functions. Go does not prescribe a line limit; its review guidance recommends changing boundaries based on semantics and warns against both oversized functions and repetitive tiny functions. [Go Code Review Comments: line length and function boundaries](https://go.dev/wiki/CodeReviewComments#line-length)

Keep `httpserver` as one package with `server.go`, `middleware.go`, `response.go`, and responsibility-named `*_handler.go` files. File names already supply navigation; a `handlers` subpackage would force exports and wiring without creating an independently useful abstraction. Go explicitly supports splitting one package across files. [Organizing a Go module](https://go.dev/doc/modules/layout)

Move `SeedWorkers` out of `disbursement/worker.go` into a small `internal/demodata` package. Hard-coded names and amounts are composition/demo fixtures, not worker or payment rules. A focused `demodata.Workers()` (or `demodata.SeedWorkers()`) makes the dependency direction honest: demo data depends on the domain constructors, while the domain no longer knows any sample people. Main and HTTP tests can use `demodata`; processor tests should construct only the one or two workers required by each test through a local helper. `demodata` is a concrete capability name, unlike the generic package names that Go's package guidance warns against. [Package names](https://go.dev/blog/package-names)

`ResetDemo` is also a non-domain feature, but its implementation must remain owned by `Processor`: it atomically changes the mutex-protected obligation and idempotency state, and moving that mutation to another package would either expose internals or require a substantially more complex replaceable-processor wrapper. Keep the method for this reviewer-facing demo, move it from `obligations.go` to `disbursement/demo.go`, and document that it is an explicitly non-production convenience which is rejected while work is pending. This is a deliberate, visible trade-off rather than pretending that paid obligations normally become payable again. If the reset endpoint is ever removed, delete the method and file together.

### 5. Make `mockpayment.Pay` read as a policy

`generateLatency` is already a good extraction. Complete the separation with a small `waitForLatency(ctx)` helper so `Pay` reads as: wait, decide failure, create transaction ID. Make the stateless empty `Provider` use a value receiver; Go recommends value receivers for small immutable values. [Go Code Review Comments: receiver type](https://go.dev/wiki/CodeReviewComments#receiver-type)

Keep `math/rand/v2` for simulated latency/failure and `crypto/rand.Text` for identifiers. The top-level `math/rand/v2` functions are safe for concurrent simulation but are not security-grade, while `crypto/rand.Text` provides collision-resistant random text. [math/rand/v2](https://pkg.go.dev/math/rand/v2) [crypto/rand.Text](https://pkg.go.dev/crypto/rand#Text)

Rename the structured log key `provider_txn_id` to `provider_transaction_id`; the API wire field remains unchanged. The longer log key is clearer operational vocabulary and avoids the already-rejected `txn` abbreviation.

### 6. Document the exported contract, not the implementation

Add concise package comments for `disbursement`, `httpserver`, `mockpayment`, and `demodata`, and doc comments for exported domain types and methods whose behavior is not obvious, especially `Submit`, `Batch`, `Wait`, the typed errors, and provider failure types. `Submit` should document atomic validation/reservation, replay behavior, and asynchronous completion; `Wait` should document its shutdown precondition.

Do not narrate getters or every line. Comments should explain invariants and lifecycle decisions. The Go review guidance calls for package comments and doc comments on exported names, with complete sentences beginning with the declaration name. [Go Code Review Comments: doc comments](https://go.dev/wiki/CodeReviewComments#doc-comments) [Go Code Review Comments: package comments](https://go.dev/wiki/CodeReviewComments#package-comments)

Separate `doc.go` files are not justified for these small packages. The official convention requires a package comment, not a particular filename. Put the short package comment in the package's primary file (`processor.go`, `server.go`, or `provider.go`) so documentation does not create several nearly empty files. Reserve `doc.go` for a genuinely substantial package overview or a generated-only package that has no suitable handwritten file.

### 7. Add focused boundary tests

Add table-driven money cases for `0.01`, `0.10`, `1.00`, a normal payroll amount, and the largest accepted two-decimal `int64` value. The current representation is correct: a signed integer stores exact minor units, and `strconv.ParseInt` rejects overflow at 64 bits. These tests make the central payment invariant easier to defend than a more elaborate decimal dependency would. [Go numeric types](https://go.dev/ref/spec#Numeric_types) [strconv.ParseInt](https://pkg.go.dev/strconv#ParseInt)

Add HTTP tests for an unknown JSON field, two concatenated JSON objects, and a body above the configured limit. The implementation already uses both `DisallowUnknownFields` and `MaxBytesReader`; tests should preserve those intentional input-boundary decisions. Extract `64 << 10` to a named `maximumRequestBodyBytes` constant. [json.Decoder.DisallowUnknownFields](https://pkg.go.dev/encoding/json#Decoder.DisallowUnknownFields) [http.MaxBytesReader](https://pkg.go.dev/net/http#MaxBytesReader)

Keep the existing simultaneous replay, partial failure, timeout/retry, reset, HTTP lifecycle, and `-race` tests. Their failure messages already follow the useful `got, want` convention. [Go Code Review Comments: useful test failures](https://go.dev/wiki/CodeReviewComments#useful-test-failures)

Split the two approximately 500-line test files by the same behaviors as production, while keeping the same test packages and shared helpers. This is justified by semantic navigation, not a line-count rule:

```text
disbursement/
  submission_test.go   replay, conflicts, unavailable and unknown workers
  processing_test.go   concurrent results, failures and timeout/retry
  demo_test.go         reset behavior
  processor_test.go    shared processor/provider test fixtures

httpserver/
  workers_test.go
  disbursements_test.go
  demo_test.go
  server_test.go       shared server/HTTP helpers and middleware behavior
```

Go does not impose a file-length limit, but these tests already have stable behavioral seams. The split reduces scrolling and makes production/test ownership obvious without adding a package or abstraction. [Go Code Review Comments: function and line boundaries](https://go.dev/wiki/CodeReviewComments#line-length)

## Already idiomatic; keep these decisions

### Exact money

`Money` has unexported fields, validates currency at construction, parses canonical decimal strings directly into `int64` minor units, and never uses `float64`. That is simpler and more readable for fixed two-decimal USD/EUR than `math/big.Rat` or a third-party decimal package. The integer is exact within its documented range, and `ParseInt` provides explicit overflow detection. Keep the representation.

For a small readability improvement, change `containsOnlyDigits` to range directly over the string's runes instead of ranging over `len(value)` and indexing. The behavior remains deliberately ASCII-only. Keep the explanatory decimal-removal comment in `parseMinorUnits`.

### Concurrency and idempotency

The processor performs replay detection, worker validation, availability checks, reservation, and pending-batch persistence while holding one write lock, then starts provider calls after releasing it. This is the correct critical section: concurrent submissions cannot reserve the same obligation or create duplicate provider work. Read-only snapshots use `RLock`, and returned results are value copies. `sync.RWMutex` explicitly permits concurrent readers or one writer. [sync.RWMutex](https://pkg.go.dev/sync#RWMutex)

Do not replace the mutex with channels merely to look more "Go-like." The current shared-state invariant is compact, covered by a simultaneous replay test, and passes the race detector.

### Error semantics

Sentinel error strings are lowercase, callers use `errors.Is` for categories and `errors.As` for typed details, and `%w` preserves intentional domain classifications. This follows Go's error-chain model. Keep domain errors free of HTTP status codes; the HTTP adapter should continue translating them. [Working with Errors in Go 1.13](https://go.dev/blog/go1.13-errors) [Go Code Review Comments: error strings](https://go.dev/wiki/CodeReviewComments#error-strings)

### Interfaces

`PaymentProvider` belongs in `disbursement`, the package that consumes the capability, while `mockpayment` returns a concrete implementation. This is exactly the Go interface-location guidance. Do not create global `ports`, `interfaces`, or `types` packages. [Go Code Review Comments: interfaces](https://go.dev/wiki/CodeReviewComments#interfaces) [Package names](https://go.dev/blog/package-names)

### HTTP boundaries

The server sets explicit read-header, read, write, idle, provider, and shutdown timeouts. JSON bodies are size-limited, unknown fields are rejected, and trailing JSON objects are rejected. Keep these controls. [http.Server timeouts](https://pkg.go.dev/net/http#Server) [http.MaxBytesReader](https://pkg.go.dev/net/http#MaxBytesReader)

The custom response recorder does not preserve optional interfaces such as `http.Flusher`, but this API does not stream, hijack, or push. Do not add generic wrapper machinery until an endpoint needs those capabilities; the standard documentation notes that wrappers may not support `Flusher`, so this should simply remain an explicit non-streaming constraint. [http.Flusher](https://pkg.go.dev/net/http#Flusher)

## Deliberate non-changes

- **No DDD folder tree.** The domain vocabulary is useful; generic enterprise layers are not. Packages should represent cohesive capabilities with good client-facing names, not architectural ceremony.
- **No queue, database, repository, event bus, or dependency-injection framework.** None is required to prove concurrent in-process execution, idempotency within the exercise's process lifetime, or partial failure isolation.
- **No `utils`, `common`, or `helpers` package.** Go's package guidance specifically warns that such packages provide no useful abstraction boundary. [Package names](https://go.dev/blog/package-names)
- **No money library.** Fixed two-decimal currencies plus explicit `int64` bounds are fully represented by the current value object.
- **No retry loop inside the provider call.** Automatic retries would change payment semantics and require provider-side idempotency guarantees that the exercise does not provide.
- **No conversion of the asynchronous API to a synchronous method.** Go generally prefers synchronous functions, but this take-home explicitly requires live pending states and concurrent background processing. The correct response is to make the exceptional goroutine lifetime bounded and documented, not to erase the requirement. [Go Code Review Comments: synchronous functions](https://go.dev/wiki/CodeReviewComments#synchronous-functions)
- **No manual edits to `internal/openapi/generated.go`.** Change the source specification or generator configuration and regenerate.

## Recommended application order

1. Add the generator initialism normalizer and regenerate.
2. Refactor context/task lifecycle and update processor/HTTP tests.
3. Move demo seed data out of the domain; isolate `ResetDemo` in a clearly named domain-owned file.
4. Move submission declarations and isolate the locked payment-result transition.
5. Simplify the mock provider latency wait and logging vocabulary.
6. Add package/contract documentation, split the large test files by behavior, and add focused boundary tests.
7. Run `gofmt`, `go vet ./...`, `go test ./...`, `go test -race ./...`, and the repository's full generated-code verification.
