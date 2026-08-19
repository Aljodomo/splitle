package domain

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCurrencySymbols(t *testing.T) {
	assert.Equal(t, "€", GetSymbol("EUR"))
	assert.Equal(t, "$", GetSymbol("USD"))
	assert.Equal(t, "¥", GetSymbol("JPY"))
	assert.Equal(t, "KD", GetSymbol("KWD"))
	assert.Equal(t, "XYZ", GetSymbol("XYZ")) // fallback to code
}

func TestConvertAmount(t *testing.T) {
	t.Run("same currency 1.0 rate", func(t *testing.T) {
		amt := decimal.NewFromFloat(10.50)
		res, err := ConvertAmount(amt, "EUR", "EUR", decimal.NewFromInt(1))
		require.NoError(t, err)
		assert.True(t, amt.Equal(res))
	})

	t.Run("EUR to USD", func(t *testing.T) {
		// 10.00 EUR at 1.085 USD/EUR = 10.85 USD
		amt := decimal.NewFromFloat(10.00)
		rate := decimal.NewFromFloat(1.085)
		res, err := ConvertAmount(amt, "EUR", "USD", rate)
		require.NoError(t, err)
		assert.True(t, decimal.NewFromFloat(10.85).Equal(res))
	})

	t.Run("JPY to EUR", func(t *testing.T) {
		// 15,000 JPY at 0.0062 EUR/JPY = 93.00 EUR
		amt := decimal.NewFromInt(15000)
		rate := decimal.NewFromFloat(0.0062)
		res, err := ConvertAmount(amt, "JPY", "EUR", rate)
		require.NoError(t, err)
		assert.True(t, decimal.NewFromFloat(93.00).Equal(res))
	})

	t.Run("KWD to USD", func(t *testing.T) {
		// 5.125 KWD at 3.25 USD/KWD = 16.65625 USD
		amt := decimal.NewFromFloat(5.125)
		rate := decimal.NewFromFloat(3.25)
		res, err := ConvertAmount(amt, "KWD", "USD", rate)
		require.NoError(t, err)
		assert.True(t, decimal.NewFromFloat(16.65625).Equal(res))
	})

	t.Run("invalid non-positive rate", func(t *testing.T) {
		amt := decimal.NewFromInt(10)
		_, err := ConvertAmount(amt, "EUR", "USD", decimal.Zero)
		assert.ErrorIs(t, err, ErrInvalidExchangeRate)
		_, err = ConvertAmount(amt, "EUR", "USD", decimal.NewFromFloat(-1.5))
		assert.ErrorIs(t, err, ErrInvalidExchangeRate)
	})
}

func TestFormatAmount(t *testing.T) {
	assert.Equal(t, "€10.50", FormatAmount(decimal.NewFromFloat(10.50), "EUR"))
	assert.Equal(t, "-$5.00", FormatAmount(decimal.NewFromFloat(-5.00), "USD"))
	assert.Equal(t, "¥1500", FormatAmount(decimal.NewFromInt(1500), "JPY"))
	assert.Equal(t, "KD5.125", FormatAmount(decimal.NewFromFloat(5.125), "KWD"))
}
