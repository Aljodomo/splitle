package ports

import (
	"context"

	"github.com/shopspring/decimal"
)

// ExchangeRateProvider resolves the direct-pair exchange rate between two currencies.
// 1 unit of fromCur = rate units of toCur.
type ExchangeRateProvider interface {
	GetRate(ctx context.Context, fromCur, toCur string) (decimal.Decimal, error)
}
