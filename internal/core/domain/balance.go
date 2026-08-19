package domain

import "github.com/shopspring/decimal"

// UserBalance represents the financial summary for a user within a group.
type UserBalance struct {
	UserID     string
	Currency   string
	PaidAmount decimal.Decimal
	OwedAmount decimal.Decimal
	NetBalance decimal.Decimal // PaidAmount - OwedAmount (positive = is owed money, negative = owes money)
}

// GroupBalance represents the overall financial balance summary for a group.
type GroupBalance struct {
	GroupID         string
	BaseCurrency    string
	Balances        map[string]UserBalance
	TotalGroupSpend decimal.Decimal
}

// DebtTransfer represents an optimal payment instruction from a debtor to a creditor.
type DebtTransfer struct {
	FromUser string
	ToUser   string
	Amount   decimal.Decimal
	Currency string // The currency of the transfer (typically group base currency)
}
