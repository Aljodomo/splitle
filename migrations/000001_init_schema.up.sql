-- Enable UUID extension if not present
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- 1. Central transactions table
CREATE TABLE IF NOT EXISTS transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id VARCHAR(255) NOT NULL,
    description TEXT NOT NULL,
    currency VARCHAR(10) NOT NULL DEFAULT 'EUR',
    base_currency VARCHAR(10) NOT NULL DEFAULT 'EUR',
    exchange_rate NUMERIC(28, 8) NOT NULL DEFAULT 1.00000000,
    created_by VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ NULL
);

CREATE INDEX IF NOT EXISTS idx_transactions_group_id ON transactions (group_id, created_at DESC) WHERE deleted_at IS NULL;

-- 2. Entry breakdown table
CREATE TABLE IF NOT EXISTS transaction_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_id UUID NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
    user_id VARCHAR(255) NOT NULL,
    paid_amount NUMERIC(28, 8) NOT NULL DEFAULT 0.00000000,
    owed_amount NUMERIC(28, 8) NOT NULL DEFAULT 0.00000000,
    CONSTRAINT uq_tx_user UNIQUE (transaction_id, user_id),
    CONSTRAINT chk_positive_amounts CHECK (paid_amount >= 0 AND owed_amount >= 0),
    CONSTRAINT chk_non_zero_entry CHECK (paid_amount > 0 OR owed_amount > 0)
);

CREATE INDEX IF NOT EXISTS idx_entries_user_id ON transaction_entries (user_id);
CREATE INDEX IF NOT EXISTS idx_entries_tx_id ON transaction_entries (transaction_id);
