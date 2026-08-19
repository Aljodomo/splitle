package domain

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestCalculatePercentage(t *testing.T) {
	tests := []struct {
		name        string
		numerator   decimal.Decimal
		denominator decimal.Decimal
		expected    decimal.Decimal
	}{
		{
			name:        "exact half",
			numerator:   decimal.NewFromInt(50),
			denominator: decimal.NewFromInt(100),
			expected:    decimal.NewFromInt(50),
		},
		{
			name:        "one third rounded",
			numerator:   decimal.NewFromInt(1),
			denominator: decimal.NewFromInt(3),
			expected:    decimal.NewFromFloat(33.33),
		},
		{
			name:        "zero denominator",
			numerator:   decimal.NewFromInt(50),
			denominator: decimal.Zero,
			expected:    decimal.Zero,
		},
		{
			name:        "negative numerator",
			numerator:   decimal.NewFromInt(-10),
			denominator: decimal.NewFromInt(100),
			expected:    decimal.Zero,
		},
		{
			name:        "zero numerator",
			numerator:   decimal.Zero,
			denominator: decimal.NewFromInt(100),
			expected:    decimal.Zero,
		},
		{
			name:        "100 percent",
			numerator:   decimal.NewFromInt(120),
			denominator: decimal.NewFromInt(120),
			expected:    decimal.NewFromInt(100),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := CalculatePercentage(tt.numerator, tt.denominator)
			assert.True(t, tt.expected.Equal(res), "expected %s, got %s", tt.expected.String(), res.String())
		})
	}
}
