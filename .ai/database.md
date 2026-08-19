# Database Design & PostgreSQL Engine

Splitle enforces a **strict, lean 2-table schema** designed specifically for group expense sharing and ledger integrity.

---

## 📊 The 2-Table Schema

```sql
-- 1. Central transactions table
CREATE TABLE transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id VARCHAR(255) NOT NULL,          -- Group / Workspace ID
    description TEXT NOT NULL,               -- e.g. "Dinner at Luigi's", "Settlement"
    currency VARCHAR(10) NOT NULL DEFAULT 'EUR',      -- Transaction native currency (e.g. JPY, USD)
    base_currency VARCHAR(10) NOT NULL DEFAULT 'EUR', -- Group base currency at creation (e.g. EUR)
    exchange_rate NUMERIC(28, 8) NOT NULL DEFAULT 1.00000000, -- Rate snapshot: 1 currency = X base_currency
    created_by VARCHAR(255) NOT NULL,        -- User ID / handle of author
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ NULL              -- Soft delete timestamp
);

CREATE INDEX idx_transactions_group_id ON transactions (group_id, created_at DESC) WHERE deleted_at IS NULL;

-- 2. Entry breakdown table
CREATE TABLE transaction_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_id UUID NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
    user_id VARCHAR(255) NOT NULL,           -- User ID
    paid_amount NUMERIC(28, 8) NOT NULL DEFAULT 0.00000000, -- Exact paid amount
    owed_amount NUMERIC(28, 8) NOT NULL DEFAULT 0.00000000, -- Exact owed amount
    CONSTRAINT uq_tx_user UNIQUE (transaction_id, user_id),
    CONSTRAINT chk_positive_amounts CHECK (paid_amount >= 0 AND owed_amount >= 0),
    CONSTRAINT chk_non_zero_entry CHECK (paid_amount > 0 OR owed_amount > 0)
);

CREATE INDEX idx_entries_user_id ON transaction_entries (user_id);
CREATE INDEX idx_entries_tx_id ON transaction_entries (transaction_id);
```

---

## 🛡️ Database Constraints & Invariants

1. **Exact Precision (`NUMERIC(28, 8)`)**: `paid_amount`, `owed_amount`, and `exchange_rate` use 8-decimal `NUMERIC` to guarantee exact base-10 arithmetic without floating-point rounding errors or dependency on manual minor-unit conversion tables.
2. **String Identifiers (`VARCHAR(255)`)**: `group_id`, `created_by`, and `user_id` are strings to accommodate arbitrary group/workspace IDs, 64-bit user IDs, usernames, or future channel identifiers without casting issues.
3. **No Redundant Tables**: User state and group membership are derived organically from transaction entries. There are no separate `users` or `groups` tables.
4. **Atomic Batch Transactions**: The PostgreSQL adapter (`PostgresRepository`) inserts the `transactions` row and all `transaction_entries` rows inside a single ACID database transaction (`tx.Begin` + `pgx.Batch`).
5. **Soft Deletes (`deleted_at`)**: Transactions are never hard-deleted during regular operation. Soft-deleting immediately excludes the transaction from balance calculations and query results.
6. **Cascade Deletion**: If a transaction is purged, `ON DELETE CASCADE` automatically removes all associated entries.

---

## 🔗 Related Documents
- [domain-rules.md](file:///Users/aljodomo/workspace/telesplit/.ai/domain-rules.md) - How amounts, currencies, and splits are calculated.
- [testing-guide.md](file:///Users/aljodomo/workspace/telesplit/.ai/testing-guide.md) - How PostgreSQL integration tests run via Testcontainers.
