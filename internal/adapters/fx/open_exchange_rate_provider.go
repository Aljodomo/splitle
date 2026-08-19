package fx

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/shopspring/decimal"
	"splitle/internal/core/domain"
	"splitle/internal/core/ports"
)

const (
	DefaultOpenExchangeRateBaseURL = "https://open.er-api.com/v6/latest"
	DefaultCacheTTL                = 1 * time.Hour
	DefaultHTTPTimeout             = 10 * time.Second
)

type openERResponse struct {
	Result             string             `json:"result"`
	Provider           string             `json:"provider"`
	Documentation      string             `json:"documentation"`
	TermsOfUse         string             `json:"terms_of_use"`
	TimeLastUpdateUnix int64              `json:"time_last_update_unix"`
	TimeLastUpdateUTC  string             `json:"time_last_update_utc"`
	TimeNextUpdateUnix int64              `json:"time_next_update_unix"`
	TimeNextUpdateUTC  string             `json:"time_next_update_utc"`
	BaseCode           string             `json:"base_code"`
	Rates              map[string]float64 `json:"rates"`
	ErrorType          string             `json:"error-type,omitempty"`
}

type cachedRates struct {
	fetchedAt time.Time
	rates     map[string]decimal.Decimal
}

// OpenExchangeRateProvider queries open.er-api.com for live global fiat exchange rates
// and caches results in-memory with a configurable TTL (defaults to 1 hour).
type OpenExchangeRateProvider struct {
	baseURL    string
	httpClient *http.Client
	cacheTTL   time.Duration

	mu    sync.RWMutex
	cache map[string]cachedRates // key: normalized base currency code
}

var _ ports.ExchangeRateProvider = (*OpenExchangeRateProvider)(nil)

// OpenExchangeRateOption allows customizing the provider.
type OpenExchangeRateOption func(*OpenExchangeRateProvider)

// WithHTTPClient configures a custom http.Client.
func WithHTTPClient(client *http.Client) OpenExchangeRateOption {
	return func(p *OpenExchangeRateProvider) {
		if client != nil {
			p.httpClient = client
		}
	}
}

// WithCacheTTL configures a custom in-memory cache TTL.
func WithCacheTTL(ttl time.Duration) OpenExchangeRateOption {
	return func(p *OpenExchangeRateProvider) {
		if ttl > 0 {
			p.cacheTTL = ttl
		}
	}
}

// WithBaseURL configures a custom API base URL (useful for testing or proxying).
func WithBaseURL(baseURL string) OpenExchangeRateOption {
	return func(p *OpenExchangeRateProvider) {
		if baseURL != "" {
			p.baseURL = baseURL
		}
	}
}

// NewOpenExchangeRateProvider creates a new ExchangeRateProvider backed by open.er-api.com.
func NewOpenExchangeRateProvider(opts ...OpenExchangeRateOption) *OpenExchangeRateProvider {
	p := &OpenExchangeRateProvider{
		baseURL:    DefaultOpenExchangeRateBaseURL,
		httpClient: &http.Client{Timeout: DefaultHTTPTimeout},
		cacheTTL:   DefaultCacheTTL,
		cache:      make(map[string]cachedRates),
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// GetRate resolves the direct-pair exchange rate (1 unit of fromCur = rate units of toCur).
func (p *OpenExchangeRateProvider) GetRate(ctx context.Context, fromCur, toCur string) (decimal.Decimal, error) {
	from := domain.NormalizeCurrency(fromCur)
	to := domain.NormalizeCurrency(toCur)

	if from == "" || to == "" {
		return decimal.Zero, fmt.Errorf("invalid currency codes: from='%s', to='%s'", fromCur, toCur)
	}

	// Direct 1:1 for identical currencies
	if from == to {
		return decimal.NewFromInt(1), nil
	}

	// 1. Check in-memory cache
	p.mu.RLock()
	entry, found := p.cache[from]
	p.mu.RUnlock()

	if found && time.Since(entry.fetchedAt) < p.cacheTTL {
		if rate, ok := entry.rates[to]; ok && rate.IsPositive() {
			return rate, nil
		}
		return decimal.Zero, fmt.Errorf("currency %s not supported in FX rates for base %s", to, from)
	}

	// 2. Fetch fresh rates from API
	rates, err := p.fetchRates(ctx, from)
	if err != nil {
		// Fallback: If network/API fails but we have stale cached rates, serve them gracefully
		if found {
			if rate, ok := entry.rates[to]; ok && rate.IsPositive() {
				return rate, nil
			}
		}
		return decimal.Zero, fmt.Errorf("open exchange rate fetch failed for %s: %w", from, err)
	}

	// 3. Update in-memory cache
	p.mu.Lock()
	p.cache[from] = cachedRates{
		fetchedAt: time.Now(),
		rates:     rates,
	}
	p.mu.Unlock()

	rate, ok := rates[to]
	if !ok || !rate.IsPositive() {
		return decimal.Zero, fmt.Errorf("currency %s not supported in FX rates for base %s", to, from)
	}

	return rate, nil
}

func (p *OpenExchangeRateProvider) fetchRates(ctx context.Context, base string) (map[string]decimal.Decimal, error) {
	url := fmt.Sprintf("%s/%s", p.baseURL, base)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected http status: %d", resp.StatusCode)
	}

	var data openERResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode JSON response: %w", err)
	}

	if data.Result != "success" {
		return nil, fmt.Errorf("API error result '%s' (type: %s)", data.Result, data.ErrorType)
	}

	if len(data.Rates) == 0 {
		return nil, fmt.Errorf("no exchange rates returned for base %s", base)
	}

	decimalRates := make(map[string]decimal.Decimal, len(data.Rates))
	for cur, rate := range data.Rates {
		decimalRates[domain.NormalizeCurrency(cur)] = decimal.NewFromFloat(rate)
	}

	return decimalRates, nil
}
