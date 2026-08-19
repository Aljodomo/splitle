# Splitle

A high-precision, multi-currency group expense sharing engine written in **Go** using **Hexagonal Architecture**.

> [!NOTE]
> This repository houses the **core business logic engine, domain models, and storage adapters**. It is a library/core engine component and does not currently include a user-facing entry point (e.g., an HTTP REST API or Web/Mobile app backend server).

## Features

- 💰 **High-Precision FX & Math**: Arbitrary-precision base-10 decimal arithmetic (`shopspring/decimal`) with locked transaction-time exchange rate snapshots.
- ⚡ **Optimal Debt Simplification**: Bitmask Dynamic Programming algorithm ($O(2^N \cdot N)$) to minimize settlement transactions.
- 📊 **Flexible Splitting**: Supports Equal, Exact, and Shares split modes with exact remainder allocation.
- 📈 **Group Analytics**: Comprehensive user balances, group summaries, and category spending analytics.
- 🏗️ **Hexagonal Architecture**: Core domain completely decoupled from persistence (PostgreSQL / In-Memory) and interfaces.

## Tech Stack

- **Language**: Go 1.22+
- **Database**: PostgreSQL 16 (`NUMERIC(28,8)`)
- **Infrastructure**: Docker & Docker Compose

## Quick Start

```bash
# Start PostgreSQL database container
docker compose up -d

# Run unit & core domain tests
make test

# Run PostgreSQL integration tests
make test-postgres
```

## Project Structure

```
.
├── internal/
│   ├── core/
│   │   ├── domain/      # Domain models, splits, decimal math, Bitmask DP debt simplification
│   │   ├── ports/       # Repository, Service, and FX provider interfaces
│   │   └── service/     # Application orchestration service
│   └── adapters/
│       ├── storage/     # PostgreSQL & In-Memory repository adapters
│       └── fx/          # Live Open Exchange Rate & Static FX adapters
├── migrations/          # SQL DDL migrations
└── test/                # Multi-currency end-to-end scenario tests
```

## License

MIT
