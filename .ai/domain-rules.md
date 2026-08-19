# Financial Domain Rules & Currency Engine

Splitle adheres to strict financial invariants to ensure 100% mathematical correctness, zero rounding drift, and immutable auditability.

---

## 💰 1. Arbitrary-Precision Fixed-Point Representation (`shopspring/decimal`)

**Rule:** NEVER use floating-point types (`float64` / `float32`) to store or calculate monetary balances in database or domain state.

All monetary amounts and exchange rates are represented using **[`github.com/shopspring/decimal`](https://github.com/shopspring/decimal)** in Go and stored as **`NUMERIC(28, 8)`** in PostgreSQL:
- **Exact Base-10 Arithmetic**: Prevents binary floating-point rounding errors (e.g. `0.1 + 0.2` is strictly `0.3`).
- **Universal Currency Support**: Eliminates the need for hardcoded decimal-place registries. Any ISO-4217 fiat currency (e.g. `USD`, `EUR`, `JPY`, `KWD`, `BHD`, `VND`) or custom currency works natively without manual configuration.
- **8-Decimal Precision**: Supports fractional splits, micro-rates, and multi-step FX conversions without loss of precision.

Currency symbols and formatting are resolved by [`domain.GetSymbol`](file:///Users/aljodomo/workspace/telesplit/internal/core/domain/currency.go) and [`domain.FormatAmount`](file:///Users/aljodomo/workspace/telesplit/internal/core/domain/currency.go).

---

## 💱 2. Direct-Pair Exchange Rate Snapshots

To prevent debt amounts from changing due to foreign exchange market fluctuations weeks after an expense is logged:

1. Every transaction stores:
   - `currency`: The currency spent (e.g. `JPY`).
   - `base_currency`: The group base currency (e.g. `EUR`).
   - `exchange_rate`: Stored as `NUMERIC(28, 8)` (e.g. `0.00620000`), representing $1\text{ currency} = X\text{ base\_currency}$ captured at creation time.
2. **Cross-Currency Formula**:
   $$\text{ConvertedAmount} = \text{Amount} \times \text{ExchangeRate}$$
3. **Zero-Sum Multi-Currency Invariant**: When converting a foreign transaction to base currency balances, proportional distribution guarantees:
   $$\sum \text{PaidInBase} == \sum \text{OwedInBase} == \text{TotalPaidInBase}$$
   preventing any discrepancy from independent entry conversions.

---

## 🍰 3. Split Strategies & Remainder Allocation

### Equal Split (`SplitEqual`)
- Total amount is divided equally among all participants.
- Fractional remainder units are assigned to participants sorted deterministically by `user_id`.
- Guarantees $\sum \text{OwedAmount} == \sum \text{PaidAmount}$ strictly.

### Exact Split (`SplitExact`)
- Each participant specifies their exact owed amount.
- Validation checks $\sum \text{exactBorrowers.Amount} == \sum \text{payers.Amount}$.

### Shares Split (`SplitShares`)
- Each participant has an integer number of shares (e.g. Alice 2, Bob 1).
- $\text{ShareAmount} = \frac{\text{TotalPaid} \times \text{Shares}}{\text{TotalShares}}$
- Remainder units are distributed deterministically across borrowers.

### Multi-Payer Invariant
- Any transaction can have 1 or more payers.
- Total paid equals the sum of all `payers.Amount`.

---

## 📊 4. Spending Analytics & Settlement Separation

To deliver accurate financial statistics and insights:
- **Pure Expense Spending**: `TotalGroupSpend` aggregates only pure expense transactions, excluding debt settlement repayments.
- **User Metrics**:
  - `TotalPaid`: Out-of-pocket funds contributed towards group expenses.
  - `TotalSpent`: Net expense share consumed by the user (their owed share).
  - `NetBalance`: `(TotalPaid + TotalSettlementsPaid) - (TotalSpent + TotalSettlementsRecv)`.
  - `SpendingPercentage`: Exact share of total group spending consumed by the user: $\frac{\text{TotalSpent}}{\text{TotalGroupSpend}} \times 100$.
  - `PaidPercentage`: Exact share of total group funding provided by the user: $\frac{\text{TotalPaid}}{\text{TotalGroupSpend}} \times 100$.
- **Settlement Isolation**: Direct settlements are tracked separately in `TotalSettlements`, `TotalSettlementsPaid`, and `TotalSettlementsRecv` to prevent artificial inflation of group spending volume.
- **Dynamic FX & Date Filtering**: Group analytics support dynamic conversion to any target currency via `ports.ExchangeRateProvider` and optional date windowing (`StartDate` / `EndDate`).

---

## 🔗 Related Documents
- [algorithms.md](file:///Users/aljodomo/workspace/telesplit/.ai/algorithms.md) - How balances are simplified into minimal debt payments.
- [database.md](file:///Users/aljodomo/workspace/telesplit/.ai/database.md) - How these records are stored in PostgreSQL.
