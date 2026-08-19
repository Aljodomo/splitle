# Testing Strategy & Guidelines

Splitle maintains a comprehensive, fast, and hermetic test suite across three tiers:
1. Pure domain & algorithm unit tests.
2. In-memory service & scenario tests.
3. Testcontainers-powered PostgreSQL integration tests.

---

## 🏃 Running Tests

```bash
# Run all tests (unit, service, E2E, and Testcontainers PostgreSQL)
go test -count=1 -v -race ./...

# Run only domain unit tests
go test -v -race ./internal/core/domain/...

# Run only the E2E trip simulation test
go test -v -race ./test/e2e/...

# Run only PostgreSQL integration tests
go test -v -race ./internal/adapters/storage/postgres/...
```

---

## 🧪 Test Suite Breakdown

### 1. Domain & Algorithm Unit Tests
- Location: [`internal/core/domain/`](file:///Users/aljodomo/workspace/telesplit/internal/core/domain/)
- `currency_test.go`: 0-decimal (JPY), 2-decimal (EUR/USD), and 3-decimal (KWD) conversion accuracy, negative amounts, formatting.
- `split_test.go`: Multi-payer splits, non-divisible cent remainder distribution, exact splits, shares splits.
- `simplify_test.go`: Proves $N - K$ minimal transfers for disjoint zero-sum subsets, cyclic debt elimination, transitive debt routing.

### 2. Service-Layer Tests
- Location: [`internal/core/service/expense_service_test.go`](file:///Users/aljodomo/workspace/telesplit/internal/core/service/expense_service_test.go)
- Uses [`MemoryRepository`](file:///Users/aljodomo/workspace/telesplit/internal/adapters/storage/memory/memory_repo.go) for instant execution.
- Tests multi-currency expense creation, direct settlement transactions, soft deletes with balance recalculation.

### 3. End-to-End System Scenario Test
- Location: [`test/e2e/scenario_trip_test.go`](file:///Users/aljodomo/workspace/telesplit/test/e2e/scenario_trip_test.go)
- Simulates a full 4-person international trip lifecycle (Alice, Bob, Charlie, Dave):
  - `SplitEqual` (Flights in EUR)
  - `SplitExact` (Hotel rooms in USD)
  - `SplitShares` (Dinner with opt-outs in JPY)
  - Multi-payer `SplitEqual` (Taxi)
  - Soft-deleting an error transaction
  - Interim partial settlement
  - Generating optimal Bitmask DP settlement transfers
  - Settling all debts until net balance reaches exactly $0.00$.

### 4. PostgreSQL Integration Tests (Testcontainers)
- Location: [`internal/adapters/storage/postgres/postgres_repo_test.go`](file:///Users/aljodomo/workspace/telesplit/internal/adapters/storage/postgres/postgres_repo_test.go)
- Automatically spins up an isolated `postgres:16-alpine` container using `testcontainers-go`.
- Executes `migrations/000001_init_schema.up.sql` on startup.
- Verifies SQL queries, unique constraints, batch inserts, and soft deletes against a real database instance.

---

## 🔗 Related Documents
- [architecture.md](file:///Users/aljodomo/workspace/telesplit/.ai/architecture.md) - Project layers and ports.
- [domain-rules.md](file:///Users/aljodomo/workspace/telesplit/.ai/domain-rules.md) - Financial invariants tested across the suite.
