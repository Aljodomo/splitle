package ports

import (
	"context"

	"github.com/google/uuid"
	"splitle/internal/core/domain"
)

// TransactionRepository defines persistence operations for transactions and entries.
type TransactionRepository interface {
	// CreateTransaction inserts a transaction and its entries atomically.
	CreateTransaction(ctx context.Context, tx *domain.Transaction) error

	// GetTransactionByID retrieves an active (non-deleted) transaction with its entries.
	GetTransactionByID(ctx context.Context, id uuid.UUID) (*domain.Transaction, error)

	// GetTransactionsByGroup retrieves all active transactions for a group, ordered by created_at DESC.
	GetTransactionsByGroup(ctx context.Context, groupID string) ([]domain.Transaction, error)

	// SoftDeleteTransaction marks a transaction as deleted.
	SoftDeleteTransaction(ctx context.Context, id uuid.UUID) error

	// Ping checks repository connectivity.
	Ping(ctx context.Context) error
}
