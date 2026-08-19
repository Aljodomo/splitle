package domain

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSimplifyDebtsOptimal(t *testing.T) {
	t.Run("empty or all zero balances", func(t *testing.T) {
		transfers, err := SimplifyDebtsOptimal(map[string]decimal.Decimal{}, "EUR")
		require.NoError(t, err)
		assert.Empty(t, transfers)

		transfers, err = SimplifyDebtsOptimal(map[string]decimal.Decimal{"alice": decimal.Zero, "bob": decimal.Zero}, "EUR")
		require.NoError(t, err)
		assert.Empty(t, transfers)
	})

	t.Run("transitive debt (Alice owes Bob $10, Bob owes Charlie $10)", func(t *testing.T) {
		// Alice: -10, Bob: 0, Charlie: +10
		netBalances := map[string]decimal.Decimal{
			"alice":   d(-10),
			"bob":     decimal.Zero,
			"charlie": d(10),
		}

		transfers, err := SimplifyDebtsOptimal(netBalances, "EUR")
		require.NoError(t, err)
		require.Len(t, transfers, 1)

		assert.Equal(t, "alice", transfers[0].FromUser)
		assert.Equal(t, "charlie", transfers[0].ToUser)
		assert.True(t, d(10).Equal(transfers[0].Amount))
		assert.Equal(t, "EUR", transfers[0].Currency)
	})

	t.Run("two independent zero-sum subsets (N=4, K=2 -> strictly 2 transfers)", func(t *testing.T) {
		// Alice: -10, Bob: +10 (Subset 1)
		// Charlie: -50, Dave: +50 (Subset 2)
		netBalances := map[string]decimal.Decimal{
			"alice":   d(-10),
			"bob":     d(10),
			"charlie": d(-50),
			"dave":    d(50),
		}

		transfers, err := SimplifyDebtsOptimal(netBalances, "EUR")
		require.NoError(t, err)
		require.Len(t, transfers, 2, "Bitmask DP must find K=2 subsets resulting in N - K = 2 transfers")

		transferMap := make(map[string]DebtTransfer)
		for _, tr := range transfers {
			transferMap[tr.FromUser] = tr
		}

		assert.Equal(t, "bob", transferMap["alice"].ToUser)
		assert.True(t, d(10).Equal(transferMap["alice"].Amount))

		assert.Equal(t, "dave", transferMap["charlie"].ToUser)
		assert.True(t, d(50).Equal(transferMap["charlie"].Amount))
	})

	t.Run("complex 5-person group with 1 zero-sum subset and 1 3-way cycle", func(t *testing.T) {
		// Group: A=-20, B=+20, C=-30, D=-10, E=+40
		// Subsets: {A, B} (sum 0, size 2 -> 1 transfer), {C, D, E} (sum 0, size 3 -> 2 transfers)
		// Total: N=5, K=2 -> 3 transfers
		netBalances := map[string]decimal.Decimal{
			"alice":   d(-20),
			"bob":     d(20),
			"charlie": d(-30),
			"dave":    d(-10),
			"eve":     d(40),
		}

		transfers, err := SimplifyDebtsOptimal(netBalances, "EUR")
		require.NoError(t, err)
		require.Len(t, transfers, 3)

		// Verify all transfers net out everyone's balance perfectly
		simulatedNet := make(map[string]decimal.Decimal)
		for _, tr := range transfers {
			simulatedNet[tr.FromUser] = simulatedNet[tr.FromUser].Sub(tr.Amount)
			simulatedNet[tr.ToUser] = simulatedNet[tr.ToUser].Add(tr.Amount)
		}

		for uid, origNet := range netBalances {
			assert.True(t, origNet.Equal(simulatedNet[uid]), "User %s net balance must match exactly", uid)
		}
	})

	t.Run("unbalanced net balance returns error", func(t *testing.T) {
		netBalances := map[string]decimal.Decimal{
			"alice": d(-10),
			"bob":   d(5),
		}

		_, err := SimplifyDebtsOptimal(netBalances, "EUR")
		assert.ErrorIs(t, err, ErrUnbalancedTransaction)
	})
}
