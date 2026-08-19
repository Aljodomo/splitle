# Splitle AI Context & Architecture Guide

Welcome to the AI & Developer context directory for **Splitle**—a high-precision group expense sharing engine (similar to Splitwise) built with Golang and PostgreSQL.

This directory serves as the source of truth for future AI coding agents and human engineers when extending the codebase.

---

## 📑 Linked Documentation Index

| Document | Description |
| :--- | :--- |
| [architecture.md](file:///Users/aljodomo/workspace/telesplit/.ai/architecture.md) | Hexagonal (Ports & Adapters) design, layer boundaries, and how to add HTTP or API adapters. |
| [database.md](file:///Users/aljodomo/workspace/telesplit/.ai/database.md) | Strict 2-table PostgreSQL schema (`NUMERIC(28, 8)`), string IDs, atomic batch inserts, and soft-delete invariants. |
| [domain-rules.md](file:///Users/aljodomo/workspace/telesplit/.ai/domain-rules.md) | Financial correctness, arbitrary-precision decimal arithmetic (`shopspring/decimal`), direct-pair FX snapshotting, and split invariants. |
| [algorithms.md](file:///Users/aljodomo/workspace/telesplit/.ai/algorithms.md) | The Optimal Bitmask DP Debt Simplification engine ($O(2^N \cdot N)$), math proofs, and settlement rules. |
| [testing-guide.md](file:///Users/aljodomo/workspace/telesplit/.ai/testing-guide.md) | Automated testing philosophy: Testcontainers for live Postgres, In-Memory unit tests, and E2E scenario testing. |

---

## 🚀 Quick Reference & Principles

1. **Strict Hexagonal Boundaries**: The `internal/core/domain/` package contains pure business logic and math.
2. **Never Use Binary Floating-Point for Currency Math**: All amounts in DB and domain models are stored and calculated using exact arbitrary-precision base-10 decimals (`shopspring/decimal` in Go and `NUMERIC(28, 8)` in PostgreSQL).
3. **Locked Direct-Pair FX Snapshots**: Transactions snapshot the exact rate `currency` $\rightarrow$ `base_currency` at creation time. Debts are immutable and never drift due to FX market movements.
4. **Lean 2-Table Schema**: Only `transactions` and `transaction_entries`. No redundant user tables or timestamp fields in entries.
5. **Optimal Debt Simplification**: Debts are settled in the absolute minimum number of payments ($N - K$, where $K$ is the number of disjoint zero-sum subsets) using Bitmask DP.
