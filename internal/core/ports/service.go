package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"splitle/internal/core/domain"
)

// RecordExpenseParams holds input data to create an expense transaction.
type RecordExpenseParams struct {
	GroupID            string
	CreatedBy          string
	Description        string
	Currency           string
	BaseCurrency       string
	CustomExchangeRate *decimal.Decimal // Optional manual FX rate override
	Payers             []domain.Payer
	SplitType          domain.SplitType
	EqualParticipants  []string
	ExactBorrowers     []domain.Borrower
	SharesBorrowers    []domain.Borrower
}

// ExpenseService defines the business use cases for group expense management.
type ExpenseService interface {
	// RecordExpense records a new multi-payer expense with the given split strategy.
	RecordExpense(ctx context.Context, params RecordExpenseParams) (*domain.Transaction, error)

	// RecordSettlement records a direct debt repayment transaction between two users.
	RecordSettlement(
		ctx context.Context,
		groupID, createdBy string,
		fromUser, toUser string,
		amount decimal.Decimal,
		currency, baseCurrency string,
		customRate *decimal.Decimal,
	) (*domain.Transaction, error)

	// GetGroupBalances returns aggregated net balances per user in the group base currency (or optional targetCurrency).
	GetGroupBalances(ctx context.Context, groupID string, targetCurrency *string) (*domain.GroupBalance, error)

	// GetGroupAnalytics calculates group-level spending insights, user breakdowns, and rankings.
	GetGroupAnalytics(ctx context.Context, groupID string, filter domain.AnalyticsFilter) (*domain.GroupAnalytics, error)

	// GetUserSpending returns individual spending and contribution metrics for a specific user.
	GetUserSpending(ctx context.Context, groupID, userID string, filter domain.AnalyticsFilter) (*domain.UserSpendingSummary, error)

	// GetSimplifiedDebts calculates the mathematically optimal minimal debt settlement transfers.
	GetSimplifiedDebts(ctx context.Context, groupID string, targetCurrency *string) ([]domain.DebtTransfer, error)

	// GetGroupTransactions retrieves all active transactions for a group.
	GetGroupTransactions(ctx context.Context, groupID string) ([]domain.Transaction, error)

	// GetTransaction retrieves a specific transaction by ID.
	GetTransaction(ctx context.Context, id uuid.UUID) (*domain.Transaction, error)

	// DeleteTransaction soft-deletes a transaction and triggers immediate balance recalculation.
	DeleteTransaction(ctx context.Context, id uuid.UUID) error
}
