package domain

import (
	"fmt"
	"strings"

	"github.com/shopspring/decimal"
)

// Currency precision used for standard calculations and database storage.
const DefaultPrecision = 8

var currencySymbols = map[string]string{
	"EUR": "€",
	"USD": "$",
	"GBP": "£",
	"CHF": "CHF",
	"CAD": "CA$",
	"AUD": "A$",
	"NZD": "NZ$",
	"SGD": "S$",
	"HKD": "HK$",
	"SEK": "kr",
	"NOK": "kr",
	"DKK": "kr.",
	"PLN": "zł",
	"BRL": "R$",
	"INR": "₹",
	"MXN": "MX$",
	"THB": "฿",
	"TRY": "₺",
	"ZAR": "R",
	"JPY": "¥",
	"KRW": "₩",
	"VND": "₫",
	"CLP": "CLP$",
	"ISK": "kr",
	"PYG": "₲",
	"UGX": "USh",
	"RWF": "RF",
	"KWD": "KD",
	"BHD": "BD",
	"OMR": "OMR",
	"JOD": "JD",
	"TND": "DT",
	"LYD": "LD",
	"IQD": "IQD",
}

// NormalizeCurrency standardizes currency string representation.
func NormalizeCurrency(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

// GetSymbol returns the currency symbol, defaulting to the normalized currency code.
func GetSymbol(code string) string {
	norm := NormalizeCurrency(code)
	if sym, ok := currencySymbols[norm]; ok {
		return sym
	}
	return norm
}

// ConvertAmount converts an amount from one currency to another using the snapshotted direct exchange rate:
// 1 unit of fromCur = exchangeRate units of toCur.
// Uses exact arbitrary-precision arithmetic rounded to DefaultPrecision (8 decimal places).
func ConvertAmount(amount decimal.Decimal, fromCur, toCur string, exchangeRate decimal.Decimal) (decimal.Decimal, error) {
	if !exchangeRate.IsPositive() {
		return decimal.Zero, ErrInvalidExchangeRate
	}

	fromNorm := NormalizeCurrency(fromCur)
	toNorm := NormalizeCurrency(toCur)

	if fromNorm == toNorm && exchangeRate.Equal(decimal.NewFromInt(1)) {
		return amount, nil
	}

	// Direct-pair multiplication with 8-decimal rounding
	converted := amount.Mul(exchangeRate).Round(DefaultPrecision)
	return converted, nil
}

// FormatAmount formats a decimal amount into a human-readable string with currency symbol.
func FormatAmount(amount decimal.Decimal, code string) string {
	norm := NormalizeCurrency(code)
	sym := GetSymbol(norm)

	sign := ""
	if amount.IsNegative() {
		sign = "-"
		amount = amount.Abs()
	}

	// If the currency is zero-decimal without fractions (e.g. JPY, KRW, VND)
	if (norm == "JPY" || norm == "KRW" || norm == "VND") && amount.Exponent() >= 0 {
		return fmt.Sprintf("%s%s%s", sign, sym, amount.StringFixed(0))
	}

	// For standard display, if fractional part has <= 2 decimals, format with 2 decimals
	// otherwise trim trailing zeroes or display full value
	str := amount.StringFixed(2)
	if amount.Round(2).Equal(amount) {
		return fmt.Sprintf("%s%s%s", sign, sym, str)
	}

	// Display exact fractional decimal string
	return fmt.Sprintf("%s%s%s", sign, sym, amount.String())
}
