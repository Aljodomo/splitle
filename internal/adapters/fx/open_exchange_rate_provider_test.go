package fx_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"splitle/internal/adapters/fx"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenExchangeRateProvider_SameCurrency(t *testing.T) {
	t.Parallel()

	// Provider should return 1.0 without making any network calls
	p := fx.NewOpenExchangeRateProvider(
		fx.WithBaseURL("http://invalid.local"),
	)

	rate, err := p.GetRate(context.Background(), "EUR", "EUR")
	require.NoError(t, err)
	assert.True(t, decimal.NewFromInt(1).Equal(rate))

	rate, err = p.GetRate(context.Background(), "usd", "USD")
	require.NoError(t, err)
	assert.True(t, decimal.NewFromInt(1).Equal(rate))
}

func TestOpenExchangeRateProvider_FetchAndCache(t *testing.T) {
	t.Parallel()

	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)

		if r.URL.Path == "/USD" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"result":    "success",
				"base_code": "USD",
				"rates": map[string]float64{
					"EUR": 0.92,
					"GBP": 0.78,
					"JPY": 155.0,
					"KWD": 0.31,
				},
			})
			return
		}

		http.NotFound(w, r)
	}))
	defer server.Close()

	provider := fx.NewOpenExchangeRateProvider(
		fx.WithBaseURL(server.URL),
		fx.WithCacheTTL(1*time.Hour),
		fx.WithHTTPClient(server.Client()),
	)

	ctx := context.Background()

	// First call: Should make HTTP request and populate cache
	rate, err := provider.GetRate(ctx, "USD", "EUR")
	require.NoError(t, err)
	assert.True(t, decimal.NewFromFloat(0.92).Equal(rate))
	assert.Equal(t, int32(1), atomic.LoadInt32(&requestCount))

	// Second call for different target currency under same base: Should hit cache (no new HTTP request)
	rateGBP, err := provider.GetRate(ctx, "USD", "GBP")
	require.NoError(t, err)
	assert.True(t, decimal.NewFromFloat(0.78).Equal(rateGBP))
	assert.Equal(t, int32(1), atomic.LoadInt32(&requestCount))

	// Third call with lowercase codes: Should hit cache
	rateJPY, err := provider.GetRate(ctx, "usd", "jpy")
	require.NoError(t, err)
	assert.True(t, decimal.NewFromInt(155).Equal(rateJPY))
	assert.Equal(t, int32(1), atomic.LoadInt32(&requestCount))
}

func TestOpenExchangeRateProvider_CacheTTLExpiry(t *testing.T) {
	t.Parallel()

	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"result":    "success",
			"base_code": "EUR",
			"rates": map[string]float64{
				"USD": 1.08,
			},
		})
	}))
	defer server.Close()

	// Short TTL of 50ms for testing expiry
	provider := fx.NewOpenExchangeRateProvider(
		fx.WithBaseURL(server.URL),
		fx.WithCacheTTL(50*time.Millisecond),
		fx.WithHTTPClient(server.Client()),
	)

	ctx := context.Background()

	rate, err := provider.GetRate(ctx, "EUR", "USD")
	require.NoError(t, err)
	assert.True(t, decimal.NewFromFloat(1.08).Equal(rate))
	assert.Equal(t, int32(1), atomic.LoadInt32(&requestCount))

	// Wait for TTL to expire
	time.Sleep(60 * time.Millisecond)

	rate2, err := provider.GetRate(ctx, "EUR", "USD")
	require.NoError(t, err)
	assert.True(t, decimal.NewFromFloat(1.08).Equal(rate2))
	assert.Equal(t, int32(2), atomic.LoadInt32(&requestCount))
}

func TestOpenExchangeRateProvider_StaleCacheFallbackOnError(t *testing.T) {
	t.Parallel()

	var shouldFail int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.LoadInt32(&shouldFail) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"result":    "success",
			"base_code": "USD",
			"rates": map[string]float64{
				"EUR": 0.925,
			},
		})
	}))
	defer server.Close()

	provider := fx.NewOpenExchangeRateProvider(
		fx.WithBaseURL(server.URL),
		fx.WithCacheTTL(10*time.Millisecond),
		fx.WithHTTPClient(server.Client()),
	)

	ctx := context.Background()

	// Initial fetch succeeds and caches
	rate, err := provider.GetRate(ctx, "USD", "EUR")
	require.NoError(t, err)
	assert.True(t, decimal.NewFromFloat(0.925).Equal(rate))

	// Wait for TTL expiry and trigger failure
	time.Sleep(20 * time.Millisecond)
	atomic.StoreInt32(&shouldFail, 1)

	// When API fails, stale cache should be served as graceful fallback
	fallbackRate, err := provider.GetRate(ctx, "USD", "EUR")
	require.NoError(t, err)
	assert.True(t, decimal.NewFromFloat(0.925).Equal(fallbackRate))
}

func TestOpenExchangeRateProvider_UnsupportedCurrency(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"result":    "success",
			"base_code": "USD",
			"rates": map[string]float64{
				"EUR": 0.92,
			},
		})
	}))
	defer server.Close()

	provider := fx.NewOpenExchangeRateProvider(
		fx.WithBaseURL(server.URL),
		fx.WithHTTPClient(server.Client()),
	)

	_, err := provider.GetRate(context.Background(), "USD", "NONEXISTENT")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}
