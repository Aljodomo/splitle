package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Transaction represents a single financial event in a group (expense or settlement).
type Transaction struct {
	ID           uuid.UUID
	GroupID      string
	Description  string
	Currency     string
	BaseCurrency string
	ExchangeRate decimal.Decimal
	CreatedBy    string
	CreatedAt    time.Time
	DeletedAt    *time.Time
	Entries      []Entry
}

// Entry represents a single user's participation (amount paid and/or owed) in a transaction.
type Entry struct {
	ID            uuid.UUID
	TransactionID uuid.UUID
	UserID        string
	PaidAmount    decimal.Decimal
	OwedAmount    decimal.Decimal
}

// Payer represents a user contributing funds to pay for a transaction.
type Payer struct {
	UserID string
	Amount decimal.Decimal
}

// Borrower represents a participant who owes a portion of the transaction.
type Borrower struct {
	UserID string
	Amount decimal.Decimal // Used for exact amount splits
	Shares int             // Used for share-based splits
}

// TotalPaid returns the sum of all paid amounts across entries.
func (t *Transaction) TotalPaid() decimal.Decimal {
	total := decimal.Zero
	for _, e := range t.Entries {
		total = total.Add(e.PaidAmount)
	}
	return total
}

// TotalOwed returns the sum of all owed amounts across entries.
func (t *Transaction) TotalOwed() decimal.Decimal {
	total := decimal.Zero
	for _, e := range t.Entries {
		total = total.Add(e.OwedAmount)
	}
	return total
}

// IsDeleted checks if the transaction has been soft-deleted.
func (t *Transaction) IsDeleted() bool {
	return t.DeletedAt != nil
}

// IsSettlement checks if the transaction represents a debt settlement repayment between users.
func (t *Transaction) IsSettlement() bool {
	return strings.HasPrefix(t.Description, "Settlement: ")
}

// Validate checks all domain invariants for a transaction.
func (t *Transaction) Validate() error {
	if strings.TrimSpace(t.GroupID) == "" {
		return ErrEmptyGroupID
	}
	if strings.TrimSpace(t.CreatedBy) == "" {
		return ErrEmptyCreatedBy
	}
	if strings.TrimSpace(t.Description) == "" {
		return ErrEmptyDescription
	}
	if strings.TrimSpace(t.Currency) == "" || strings.TrimSpace(t.BaseCurrency) == "" {
		return ErrUnsupportedCurrency
	}
	if !t.ExchangeRate.IsPositive() {
		return ErrInvalidExchangeRate
	}
	if len(t.Entries) == 0 {
		return ErrNoBorrowers
	}

	userSet := make(map[string]struct{}, len(t.Entries))
	totalPaid := decimal.Zero
	totalOwed := decimal.Zero

	for _, e := range t.Entries {
		if strings.TrimSpace(e.UserID) == "" {
			return ErrEmptyCreatedBy
		}
		if _, exists := userSet[e.UserID]; exists {
			return ErrDuplicateUserInEntries
		}
		userSet[e.UserID] = struct{}{}

		if e.PaidAmount.IsNegative() || e.OwedAmount.IsNegative() {
			return ErrInvalidAmount
		}
		if e.PaidAmount.IsZero() && e.OwedAmount.IsZero() {
			return ErrInvalidAmount
		}

		totalPaid = totalPaid.Add(e.PaidAmount)
		totalOwed = totalOwed.Add(e.OwedAmount)
	}

	if totalPaid.IsZero() {
		return ErrNoPayers
	}
	if totalOwed.IsZero() {
		return ErrNoBorrowers
	}
	if !totalPaid.Equal(totalOwed) {
		return ErrUnbalancedTransaction
	}

	return nil
}
