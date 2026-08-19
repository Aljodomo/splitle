package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"splitle/internal/core/domain"
	"splitle/internal/core/ports"
)

// PostgresRepository is the PostgreSQL storage adapter implementing ports.TransactionRepository.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

var _ ports.TransactionRepository = (*PostgresRepository)(nil)

// NewPostgresRepository creates a repository using an existing pgxpool connection pool.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// NewPostgresRepositoryFromURL creates a connection pool and returns a repository.
func NewPostgresRepositoryFromURL(ctx context.Context, connString string) (*PostgresRepository, error) {
	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse postgres config: %w", err)
	}

	config.MaxConns = 25
	config.MinConns = 2
	config.MaxConnLifetime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create postgres connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}

	return &PostgresRepository{pool: pool}, nil
}

// Close closes the connection pool.
func (r *PostgresRepository) Close() {
	if r.pool != nil {
		r.pool.Close()
	}
}

// CreateTransaction atomically inserts a transaction and its entries.
func (r *PostgresRepository) CreateTransaction(ctx context.Context, tx *domain.Transaction) error {
	if err := tx.Validate(); err != nil {
		return err
	}

	if tx.ID == uuid.Nil {
		tx.ID = uuid.New()
	}
	if tx.CreatedAt.IsZero() {
		tx.CreatedAt = time.Now().UTC()
	}

	dbTx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = dbTx.Rollback(ctx) }()

	// 1. Insert transaction header
	txQuery := `
		INSERT INTO transactions (
			id, group_id, description, currency, base_currency, exchange_rate, created_by, created_at, deleted_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err = dbTx.Exec(ctx, txQuery,
		tx.ID,
		tx.GroupID,
		tx.Description,
		tx.Currency,
		tx.BaseCurrency,
		tx.ExchangeRate,
		tx.CreatedBy,
		tx.CreatedAt,
		tx.DeletedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert transaction header: %w", err)
	}

	// 2. Insert transaction entries using batch
	batch := &pgx.Batch{}
	entryQuery := `
		INSERT INTO transaction_entries (
			id, transaction_id, user_id, paid_amount, owed_amount
		) VALUES ($1, $2, $3, $4, $5)
	`
	for i := range tx.Entries {
		if tx.Entries[i].ID == uuid.Nil {
			tx.Entries[i].ID = uuid.New()
		}
		tx.Entries[i].TransactionID = tx.ID

		batch.Queue(entryQuery,
			tx.Entries[i].ID,
			tx.ID,
			tx.Entries[i].UserID,
			tx.Entries[i].PaidAmount,
			tx.Entries[i].OwedAmount,
		)
	}

	batchResults := dbTx.SendBatch(ctx, batch)
	for range tx.Entries {
		if _, err := batchResults.Exec(); err != nil {
			_ = batchResults.Close()
			return fmt.Errorf("failed to insert transaction entry: %w", err)
		}
	}
	if err := batchResults.Close(); err != nil {
		return fmt.Errorf("failed to finalize batch inserts: %w", err)
	}

	if err := dbTx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetTransactionByID retrieves an active transaction and its entries.
func (r *PostgresRepository) GetTransactionByID(ctx context.Context, id uuid.UUID) (*domain.Transaction, error) {
	txQuery := `
		SELECT id, group_id, description, currency, base_currency, exchange_rate, created_by, created_at, deleted_at
		FROM transactions
		WHERE id = $1 AND deleted_at IS NULL
	`
	var tx domain.Transaction
	err := r.pool.QueryRow(ctx, txQuery, id).Scan(
		&tx.ID,
		&tx.GroupID,
		&tx.Description,
		&tx.Currency,
		&tx.BaseCurrency,
		&tx.ExchangeRate,
		&tx.CreatedBy,
		&tx.CreatedAt,
		&tx.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrTransactionNotFound
		}
		return nil, fmt.Errorf("failed to get transaction by id: %w", err)
	}

	// Fetch entries
	entriesQuery := `
		SELECT id, transaction_id, user_id, paid_amount, owed_amount
		FROM transaction_entries
		WHERE transaction_id = $1
		ORDER BY user_id ASC
	`
	rows, err := r.pool.Query(ctx, entriesQuery, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get entries: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var e domain.Entry
		if err := rows.Scan(&e.ID, &e.TransactionID, &e.UserID, &e.PaidAmount, &e.OwedAmount); err != nil {
			return nil, fmt.Errorf("failed to scan entry: %w", err)
		}
		tx.Entries = append(tx.Entries, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading entry rows: %w", err)
	}

	return &tx, nil
}

// GetTransactionsByGroup retrieves all active transactions for a group with their entries.
func (r *PostgresRepository) GetTransactionsByGroup(ctx context.Context, groupID string) ([]domain.Transaction, error) {
	txQuery := `
		SELECT id, group_id, description, currency, base_currency, exchange_rate, created_by, created_at, deleted_at
		FROM transactions
		WHERE group_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
	`
	rows, err := r.pool.Query(ctx, txQuery, groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to query transactions for group: %w", err)
	}
	defer rows.Close()

	var transactions []domain.Transaction
	var txIDs []uuid.UUID
	txMap := make(map[uuid.UUID]int)

	for rows.Next() {
		var tx domain.Transaction
		if err := rows.Scan(
			&tx.ID,
			&tx.GroupID,
			&tx.Description,
			&tx.Currency,
			&tx.BaseCurrency,
			&tx.ExchangeRate,
			&tx.CreatedBy,
			&tx.CreatedAt,
			&tx.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan transaction: %w", err)
		}
		txMap[tx.ID] = len(transactions)
		transactions = append(transactions, tx)
		txIDs = append(txIDs, tx.ID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading transaction rows: %w", err)
	}

	if len(txIDs) == 0 {
		return []domain.Transaction{}, nil
	}

	// Fetch all entries for these transactions in a single batch query
	entriesQuery := `
		SELECT id, transaction_id, user_id, paid_amount, owed_amount
		FROM transaction_entries
		WHERE transaction_id = ANY($1)
		ORDER BY user_id ASC
	`
	entryRows, err := r.pool.Query(ctx, entriesQuery, txIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to query entries for transactions: %w", err)
	}
	defer entryRows.Close()

	for entryRows.Next() {
		var e domain.Entry
		if err := entryRows.Scan(&e.ID, &e.TransactionID, &e.UserID, &e.PaidAmount, &e.OwedAmount); err != nil {
			return nil, fmt.Errorf("failed to scan entry: %w", err)
		}
		if idx, ok := txMap[e.TransactionID]; ok {
			transactions[idx].Entries = append(transactions[idx].Entries, e)
		}
	}

	if err := entryRows.Err(); err != nil {
		return nil, fmt.Errorf("error reading entry rows: %w", err)
	}

	return transactions, nil
}

// SoftDeleteTransaction marks a transaction as deleted.
func (r *PostgresRepository) SoftDeleteTransaction(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE transactions
		SET deleted_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`
	tag, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to soft delete transaction: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return domain.ErrTransactionNotFound
	}

	return nil
}

// Ping verifies database connectivity.
func (r *PostgresRepository) Ping(ctx context.Context) error {
	return r.pool.Ping(ctx)
}
