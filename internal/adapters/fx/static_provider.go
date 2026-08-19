package fx

import (
	"context"
	"fmt"
	"sync"

	"github.com/shopspring/decimal"
	"splitle/internal/core/domain"
	"splitle/internal/core/ports"
)

// StaticFXProvider is a thread-safe in-memory exchange rate provider with preset standard rates.
type StaticFXProvider struct {
	mu    sync.RWMutex
	rates map[string]decimal.Decimal
}

var _ ports.ExchangeRateProvider = (*StaticFXProvider)(nil)

// NewStaticFXProvider initializes the provider with default baseline rates.
func NewStaticFXProvider() *StaticFXProvider {
	p := &StaticFXProvider{
		rates: make(map[string]decimal.Decimal),
	}

	// Preset common sample rates
	p.SetRateFloat("EUR", "USD", 1.0850)
	p.SetRateFloat("USD", "EUR", 0.9216)
	p.SetRateFloat("EUR", "GBP", 0.8550)
	p.SetRateFloat("GBP", "EUR", 1.1696)
	p.SetRateFloat("JPY", "EUR", 0.0062)
	p.SetRateFloat("EUR", "JPY", 161.29)
	p.SetRateFloat("JPY", "USD", 0.0067)
	p.SetRateFloat("USD", "JPY", 149.25)
	p.SetRateFloat("KWD", "USD", 3.2500)
	p.SetRateFloat("USD", "KWD", 0.3077)
	p.SetRateFloat("KWD", "EUR", 3.0000)
	p.SetRateFloat("EUR", "KWD", 0.3333)

	return p
}

// SetRateFloat registers or updates the rate for a currency pair using float64.
func (p *StaticFXProvider) SetRateFloat(fromCur, toCur string, rate float64) {
	p.SetRate(fromCur, toCur, decimal.NewFromFloat(rate))
}

// SetRate registers or updates the rate for a currency pair (1 fromCur = rate toCur).
func (p *StaticFXProvider) SetRate(fromCur, toCur string, rate decimal.Decimal) {
	p.mu.Lock()
	defer p.mu.Unlock()
	key := fmt.Sprintf("%s:%s", domain.NormalizeCurrency(fromCur), domain.NormalizeCurrency(toCur))
	p.rates[key] = rate
}

// GetRate returns the exchange rate for the requested pair.
func (p *StaticFXProvider) GetRate(ctx context.Context, fromCur, toCur string) (decimal.Decimal, error) {
	fromNorm := domain.NormalizeCurrency(fromCur)
	toNorm := domain.NormalizeCurrency(toCur)

	if fromNorm == toNorm {
		return decimal.NewFromInt(1), nil
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	key := fmt.Sprintf("%s:%s", fromNorm, toNorm)
	if rate, exists := p.rates[key]; exists && rate.IsPositive() {
		return rate, nil
	}

	// Inverse lookup fallback
	invKey := fmt.Sprintf("%s:%s", toNorm, fromNorm)
	if invRate, exists := p.rates[invKey]; exists && invRate.IsPositive() {
		return decimal.NewFromInt(1).DivRound(invRate, domain.DefaultPrecision), nil
	}

	return decimal.NewFromInt(1), nil // Default 1:1 if rate not explicitly configured
}
