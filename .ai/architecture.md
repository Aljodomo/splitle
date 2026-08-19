# Architecture & Component Layout

Splitle is architected using the **Hexagonal (Ports & Adapters / Clean Architecture)** pattern in Go.

---

## 🏛️ Layer Diagram

```
                               ┌────────────────────────┐
                               │  Future HTTP REST API  │
                               │  or gRPC / Web Backend │
                               └───────────┬────────────┘
                                          │ (Inbound calls)
                                          ▼
┌────────────────────────────────────────────────────────────────────────────────────────┐
│                                   CORE APPLICATION                                     │
│                                                                                        │
│   ┌────────────────────────────────────────────────────────────────────────────────┐   │
│   │                              ports.ExpenseService                              │   │
│   │                       (internal/core/service/expense_service.go)               │   │
│   └───────────────────────────────────────┬────────────────────────────────────────┘   │
│                                           │                                            │
│   ┌───────────────────────────────────────▼────────────────────────────────────────┐   │
│   │                                 DOMAIN LAYER                                   │   │
│   │                         (internal/core/domain/)                                │   │
│   │  - expense.go (Entities: Transaction, Entry, Payer, Borrower)                 │   │
│   │  - balance.go (GroupBalance, UserBalance, DebtTransfer)                        │   │
│   │  - analytics.go (GroupAnalytics, UserSpendingSummary, AnalyticsFilter)         │   │
│   │  - currency.go (Arbitrary-Precision FX Math, Symbols, Formatting)              │   │
│   │  - split.go (SplitEqual, SplitExact, SplitShares + Remainder Allocation)       │   │
│   │  - simplify.go (Optimal Bitmask DP Cash Flow Minimization)                     │   │
│   │  - errors.go (Sentinel Domain Errors)                                          │   │
│   │  * Precision: Exact base-10 decimal.Decimal math via shopspring/decimal        │   │
│   └───────────────────────────────────────┬────────────────────────────────────────┘   │
│                                           │                                            │
│   ┌───────────────────────────────────────┴────────────────────────────────────────┐   │
│   │                               PORTS (INTERFACES)                               │   │
│   │                             (internal/core/ports/)                             │   │
│   │  - repository.go (TransactionRepository)                                       │   │
│   │  - fx.go (ExchangeRateProvider)                                                │   │
│   │  - service.go (ExpenseService & RecordExpenseParams)                           │   │
│   └────────────────────────────────────────────────────────────────────────────────┘   │
└───────────────────────────────────────────┬────────────────────────────────────────────┘
                                            │
                                            ▼ (Outbound adapters implement ports)
┌────────────────────────────────────────────────────────────────────────────────────────┐
│                                       ADAPTERS                                         │
│                                 (internal/adapters/)                                   │
│                                                                                        │
│  ┌───────────────────────────┐   ┌───────────────────────────┐   ┌──────────────────┐  │
│  │     storage/postgres      │   │      storage/memory       │   │      fx/open     │  │
│  │ (NUMERIC(28,8) PostgreSQL)│   │ (Thread-Safe In-Memory)   │   │(OpenExchangeRate)│  │
│  └───────────────────────────┘   └───────────────────────────┘   └──────────────────┘  │
└────────────────────────────────────────────────────────────────────────────────────────┘
```

---

## 📁 Directory Structure

```
telesplit/
├── .ai/                                    # AI & Developer Context Documentation
├── docker-compose.yml                      # Local PostgreSQL 16 Service
├── Makefile                                # Build, test, and container scripts
├── migrations/                             # SQL DDL migrations
│   ├── 000001_init_schema.up.sql
│   └── 000001_init_schema.down.sql
├── internal/
│   ├── core/
│   │   ├── domain/                         # Models, splits, decimal math, Bitmask DP
│   │   ├── ports/                          # Interfaces (Repository, Service, FX Provider)
│   │   └── service/                        # Application orchestration service
│   └── adapters/
│       ├── storage/
│       │   ├── memory/                     # In-memory repository adapter
│       │   └── postgres/                   # PostgreSQL repository adapter (pgx/v5)
│       └── fx/
│           ├── open_exchange_rate_provider.go # Live global open.er-api.com FX adapter
│           └── static_provider.go          # Baseline / test exchange rate adapter
└── test/
    └── e2e/                                # End-to-end multi-currency scenario tests
```

---

## 🔌 How to Add Future Inbound & Outbound Adapters

### 1. Adding an Inbound API / Web Adapter (Future Phase)
- Create `internal/adapters/http/` or `internal/adapters/grpc/`.
- The API handler parses incoming requests (e.g. `POST /expenses`).
- It calls `ports.ExpenseService.RecordExpense(...)`.
- It formats domain responses using `domain.FormatAmount(...)`.
- **Domain code and database code require ZERO modifications**.

### 2. Exchange Rate Provider (`ports.ExchangeRateProvider`)
- Implemented by `OpenExchangeRateProvider` (`open.er-api.com` with in-memory caching) and `StaticFXProvider`.
- Contract:
  ```go
  type ExchangeRateProvider interface {
      GetRate(ctx context.Context, fromCur, toCur string) (decimal.Decimal, error)
  }
  ```

---

## 🔗 Related Documents
- [database.md](file:///Users/aljodomo/workspace/telesplit/.ai/database.md) - Database schema and persistence rules.
- [domain-rules.md](file:///Users/aljodomo/workspace/telesplit/.ai/domain-rules.md) - Business logic and financial invariants.
