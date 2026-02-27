## Test Types Comparison

| Feature          | Unit       | Integration | Component    | E2E  |
| ---------------- | ---------- | ----------- | ------------ | ---- |
| Docker database  | No         | Yes         | Yes          | Yes  |
| External systems | No         | No          | No (stubbed) | Yes  |
| Business cases   | Depends    | No          | Yes          | Yes  |
| Tested API       | Go package | Go package  | HTTP/gRPC    | HTTP |
| Speed            | Fast       | Fast        | Medium       | Slow |
| Intro cost       | $          | $$          | $$$          | $    |
| Maintenance cost | $          | $$          | $$           | $$$$ |

### Integration vs Component

- **Integration tests** — "unit tests for adapters." Verify a single struct (e.g. a repository) works correctly with real infrastructure. No business logic.
- **Component tests** — verify complete business flows through the public API (HTTP/gRPC). Spin up the entire service; stub only external dependencies.

### When to use each

- **Unit** (ms): logic in isolation, run constantly
- **Integration** (seconds): adapters with real databases
- **Component** (tens of seconds): business flows end-to-end within the service

For simpler projects, component tests alone may be sufficient. As complexity grows, push edge cases down to unit/integration tests and keep component tests focused on happy paths.

---

## Component Test Setup

**Production** — service calls real external APIs:

```
Tests → Service → Redis (Docker) + PostgreSQL (Docker) + Receipts API + Spreadsheets API
```

**Component tests** — external APIs are replaced with stubs:

```
Tests → Service → Redis (Docker) + PostgreSQL (Docker) + ReceiptsStub + SpreadsheetsStub
```

Three requirements:

1. Run the service as a function (not a separate process)
2. Stub external dependencies behind interfaces
3. Use real infrastructure in Docker

---

## Stubs vs Mocks

**Avoid generated mocks** — they are fragile, require updating on every logic change, and test method calls rather than behaviour.

**Use stubs** — simple structs that implement the same interface, faking behaviour and recording inputs for assertions.

```go
type ReceiptsService interface {
    IssueReceipt(ctx context.Context, request IssueReceiptRequest) error
}

type ReceiptsServiceStub struct {
    lock            sync.Mutex
    IssuedReceipts  []IssueReceiptRequest
}

func (s *ReceiptsServiceStub) IssueReceipt(ctx context.Context, request IssueReceiptRequest) error {
    s.lock.Lock()
    defer s.lock.Unlock()
    s.IssuedReceipts = append(s.IssuedReceipts, request)
    return nil
}
```

- Written once; no per-test setup
- `IssuedReceipts` is inspected directly in assertions
- `sync.Mutex` makes it safe for parallel tests

---

## Parallel Test Safety

Parallel tests share state, which causes races and flaky results.

| Shared state          | Problem                                        | Solution                                                 |
| --------------------- | ---------------------------------------------- | -------------------------------------------------------- |
| Stub in-memory fields | Concurrent writes corrupt the slice            | Mutex on all field access                                |
| Database tables       | Unique constraint violations, wrong row counts | Unique IDs per test; assert on specific rows, not counts |
| Redis keys            | Tests overwrite each other's data              | Unique key prefixes per test                             |

---

## What to Test in Component Tests

Focus on **happy paths** per feature. Push edge cases to unit/integration tests.

Example happy paths for a ticket application:

- A receipt is issued for a confirmed ticket
- `tickets-to-print` sheet updated when a ticket is confirmed
- `tickets-to-refund` sheet updated when a ticket is cancelled

Write component tests **before** enabling the feature in production (TDD-style works well here).

---

## Summary

| Concept        | Detail                                                                   |
| -------------- | ------------------------------------------------------------------------ |
| Component test | Full service, Docker infra, stubbed externals, tested via HTTP/gRPC      |
| Stub           | Interface implementation that records inputs; no external calls          |
| Mock           | Avoid — tests method calls, not behaviour; fragile                       |
| Thread safety  | Protect stub state with `sync.Mutex`; use unique IDs for infra isolation |
| Scope          | Happy paths in component tests; edge cases in unit/integration tests     |
