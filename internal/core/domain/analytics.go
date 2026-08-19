package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

// AnalyticsFilter provides optional filtering parameters for group analytics calculations.
type AnalyticsFilter struct {
	StartDate      *time.Time
	EndDate        *time.Time
	TargetCurrency *string
}

// UserSpendingSummary represents spending, funding, and participation metrics for an individual user in a group.
type UserSpendingSummary struct {
	UserID               string          `json:"user_id"`
	Currency             string          `json:"currency"`
	TotalPaid            decimal.Decimal `json:"total_paid"`              // Total funds paid out by user for group expenses
	TotalSpent           decimal.Decimal `json:"total_spent"`             // Total expense share consumed by user (owed)
	NetBalance           decimal.Decimal `json:"net_balance"`             // TotalPaid - TotalSpent
	TotalSettlementsPaid decimal.Decimal `json:"total_settlements_paid"` // Direct settlements paid to other group members
	TotalSettlementsRecv decimal.Decimal `json:"total_settlements_recv"` // Direct settlements received from other group members
	SpendingPercentage   decimal.Decimal `json:"spending_percentage"`     // (TotalSpent / TotalGroupSpend) * 100
	PaidPercentage       decimal.Decimal `json:"paid_percentage"`         // (TotalPaid / TotalGroupSpend) * 100
	ExpenseCount         int             `json:"expense_count"`           // Number of group expense transactions participated in
	SettlementCount      int             `json:"settlement_count"`        // Number of settlement transactions participated in
}

// GroupAnalytics represents aggregated group analytics, total spending, and financial insights.
type GroupAnalytics struct {
	GroupID              string                         `json:"group_id"`
	Currency             string                         `json:"currency"`
	TotalGroupSpend      decimal.Decimal                `json:"total_group_spend"`     // Pure group expenses total (excluding settlements)
	TotalSettlements     decimal.Decimal                `json:"total_settlements"`     // Total settlement transfer volume
	TotalTransactions    int                            `json:"total_transactions"`    // Total active transactions in scope
	ExpenseCount         int                            `json:"expense_count"`         // Total pure expense transactions
	SettlementCount      int                            `json:"settlement_count"`      // Total settlement transactions
	AverageExpenseAmount decimal.Decimal                `json:"average_expense_amount"` // TotalGroupSpend / ExpenseCount
	UserSummaries        map[string]UserSpendingSummary `json:"user_summaries"`        // Per-user spending summaries
	TopPayerID           string                         `json:"top_payer_id"`          // User who paid the highest total amount
	TopPayerAmount       decimal.Decimal                `json:"top_payer_amount"`      // Amount paid by TopPayer
	TopSpenderID         string                         `json:"top_spender_id"`        // User who consumed the highest share of expenses
	TopSpenderAmount     decimal.Decimal                `json:"top_spender_amount"`    // Amount consumed by TopSpender
}

// CalculatePercentage computes (numerator / denominator) * 100 rounded to 2 decimal places.
// Returns decimal.Zero if denominator is zero or negative.
func CalculatePercentage(numerator, denominator decimal.Decimal) decimal.Decimal {
	if !denominator.IsPositive() || numerator.IsNegative() {
		return decimal.Zero
	}
	hundred := decimal.NewFromInt(100)
	return numerator.Mul(hundred).DivRound(denominator, 2)
}
