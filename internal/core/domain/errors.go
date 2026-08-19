package domain

import "errors"

var (
	ErrInvalidAmount           = errors.New("amount must be strictly positive")
	ErrUnbalancedTransaction    = errors.New("transaction is unbalanced: sum of paid amounts does not equal sum of owed amounts")
	ErrEmptyGroupID            = errors.New("group_id cannot be empty")
	ErrEmptyCreatedBy          = errors.New("created_by cannot be empty")
	ErrEmptyDescription        = errors.New("description cannot be empty")
	ErrNoPayers                = errors.New("at least one payer is required")
	ErrNoBorrowers             = errors.New("at least one borrower is required")
	ErrDuplicateUserInEntries  = errors.New("duplicate user in transaction entries")
	ErrTransactionNotFound     = errors.New("transaction not found")
	ErrInvalidExchangeRate     = errors.New("exchange rate must be strictly positive")
	ErrUnsupportedCurrency     = errors.New("unsupported currency code")
	ErrInvalidSplit            = errors.New("invalid split configuration")
	ErrSamePayerAndReceiver    = errors.New("payer and receiver cannot be the same user in a settlement")
)
