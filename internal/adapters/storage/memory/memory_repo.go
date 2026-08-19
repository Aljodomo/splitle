package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"splitle/internal/core/domain"
	"splitle/internal/core/ports"
)

// MemoryRepository is a concurrent-safe in-memory transaction repository for testing.
type MemoryRepository struct {
	mu           sync.RWMutex
	transactions map[uuid.UUID]*domain.Transaction
}

var _ ports.TransactionRepository = (*MemoryRepository)(nil)

// NewMemoryRepository initializes an empty in-memory repository.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		transactions: make(map[uuid.UUID]*domain.Transaction),
	}
}

// CreateTransaction inserts a transaction and its entries into memory.
func (m *MemoryRepository) CreateTransaction(ctx context.Context, tx *domain.Transaction) error {
	if err := tx.Validate(); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if tx.ID == uuid.Nil {
		tx.ID = uuid.New()
	}
	if tx.CreatedAt.IsZero() {
		tx.CreatedAt = time.Now().UTC()
	}

	// Deep copy to prevent mutation outside repository
	txCopy := *tx
	txCopy.Entries = make([]domain.Entry, len(tx.Entries))
	for i, e := range tx.Entries {
		entryCopy := e
		if entryCopy.ID == uuid.Nil {
			entryCopy.ID = uuid.New()
		}
		entryCopy.TransactionID = txCopy.ID
		txCopy.Entries[i] = entryCopy
	}

	m.transactions[txCopy.ID] = &txCopy
	return nil
}

// GetTransactionByID retrieves an active (non-deleted) transaction by ID.
func (m *MemoryRepository) GetTransactionByID(ctx context.Context, id uuid.UUID) (*domain.Transaction, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tx, exists := m.transactions[id]
	if !exists || tx.IsDeleted() {
		return nil, domain.ErrTransactionNotFound
	}

	// Return deep copy
	txCopy := *tx
	txCopy.Entries = make([]domain.Entry, len(tx.Entries))
	copy(txCopy.Entries, tx.Entries)
	return &txCopy, nil
}

// GetTransactionsByGroup retrieves all active transactions for a group, ordered by created_at DESC.
func (m *MemoryRepository) GetTransactionsByGroup(ctx context.Context, groupID string) ([]domain.Transaction, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []domain.Transaction
	for _, tx := range m.transactions {
		if tx.GroupID == groupID && !tx.IsDeleted() {
			txCopy := *tx
			txCopy.Entries = make([]domain.Entry, len(tx.Entries))
			copy(txCopy.Entries, tx.Entries)
			result = append(result, txCopy)
		}
	}

	// Sort by created_at DESC
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	return result, nil
}

// SoftDeleteTransaction marks a transaction as deleted.
func (m *MemoryRepository) SoftDeleteTransaction(ctx context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tx, exists := m.transactions[id]
	if !exists || tx.IsDeleted() {
		return domain.ErrTransactionNotFound
	}

	now := time.Now().UTC()
	tx.DeletedAt = &now
	return nil
}

// Ping checks repository connectivity.
func (m *MemoryRepository) Ping(ctx context.Context) error {
	return nil
}
