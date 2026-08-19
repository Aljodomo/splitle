package domain

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func d(val float64) decimal.Decimal {
	return decimal.NewFromFloat(val)
}

func TestCalculateSplit_Equal(t *testing.T) {
	t.Run("perfectly divisible equal split", func(t *testing.T) {
		payers := []Payer{{UserID: "alice", Amount: d(9.00)}}
		participants := []string{"alice", "bob", "charlie"}

		entries, err := CalculateSplit(payers, SplitEqual, participants, nil, nil)
		require.NoError(t, err)
		assert.Len(t, entries, 3)

		// alice: paid 9.00, owed 3.00
		// bob: paid 0, owed 3.00
		// charlie: paid 0, owed 3.00
		entryMap := make(map[string]Entry)
		for _, e := range entries {
			entryMap[e.UserID] = e
		}

		assert.True(t, d(9.00).Equal(entryMap["alice"].PaidAmount))
		assert.True(t, d(3.00).Equal(entryMap["alice"].OwedAmount))
		assert.True(t, decimal.Zero.Equal(entryMap["bob"].PaidAmount))
		assert.True(t, d(3.00).Equal(entryMap["bob"].OwedAmount))
		assert.True(t, decimal.Zero.Equal(entryMap["charlie"].PaidAmount))
		assert.True(t, d(3.00).Equal(entryMap["charlie"].OwedAmount))
	})

	t.Run("non-divisible equal split with 1-cent remainder distribution", func(t *testing.T) {
		// 10.00 / 3 = 3.333...
		// sorted order: "alice", "bob", "charlie"
		// alice gets 3.34, bob gets 3.33, charlie gets 3.33
		payers := []Payer{{UserID: "alice", Amount: d(10.00)}}
		participants := []string{"charlie", "bob", "alice"}

		entries, err := CalculateSplit(payers, SplitEqual, participants, nil, nil)
		require.NoError(t, err)

		entryMap := make(map[string]Entry)
		totalOwed := decimal.Zero
		for _, e := range entries {
			entryMap[e.UserID] = e
			totalOwed = totalOwed.Add(e.OwedAmount)
		}

		assert.True(t, d(10.00).Equal(totalOwed))
		assert.True(t, d(3.34).Equal(entryMap["alice"].OwedAmount))
		assert.True(t, d(3.33).Equal(entryMap["bob"].OwedAmount))
		assert.True(t, d(3.33).Equal(entryMap["charlie"].OwedAmount))
	})

	t.Run("multi-payer equal split", func(t *testing.T) {
		// Alice paid 6.00, Bob paid 4.00 (Total 10.00)
		// Split between Alice, Bob, Charlie (3.34, 3.33, 3.33)
		payers := []Payer{
			{UserID: "alice", Amount: d(6.00)},
			{UserID: "bob", Amount: d(4.00)},
		}
		participants := []string{"alice", "bob", "charlie"}

		entries, err := CalculateSplit(payers, SplitEqual, participants, nil, nil)
		require.NoError(t, err)

		entryMap := make(map[string]Entry)
		for _, e := range entries {
			entryMap[e.UserID] = e
		}

		assert.True(t, d(6.00).Equal(entryMap["alice"].PaidAmount))
		assert.True(t, d(3.34).Equal(entryMap["alice"].OwedAmount))

		assert.True(t, d(4.00).Equal(entryMap["bob"].PaidAmount))
		assert.True(t, d(3.33).Equal(entryMap["bob"].OwedAmount))

		assert.True(t, decimal.Zero.Equal(entryMap["charlie"].PaidAmount))
		assert.True(t, d(3.33).Equal(entryMap["charlie"].OwedAmount))
	})
}

func TestCalculateSplit_Exact(t *testing.T) {
	t.Run("valid exact split", func(t *testing.T) {
		payers := []Payer{{UserID: "alice", Amount: d(10.00)}}
		exactBorrowers := []Borrower{
			{UserID: "bob", Amount: d(6.00)},
			{UserID: "charlie", Amount: d(4.00)},
		}

		entries, err := CalculateSplit(payers, SplitExact, nil, exactBorrowers, nil)
		require.NoError(t, err)

		entryMap := make(map[string]Entry)
		for _, e := range entries {
			entryMap[e.UserID] = e
		}

		assert.True(t, d(10.00).Equal(entryMap["alice"].PaidAmount))
		assert.True(t, decimal.Zero.Equal(entryMap["alice"].OwedAmount))
		assert.True(t, d(6.00).Equal(entryMap["bob"].OwedAmount))
		assert.True(t, d(4.00).Equal(entryMap["charlie"].OwedAmount))
	})

	t.Run("unbalanced exact split returns error", func(t *testing.T) {
		payers := []Payer{{UserID: "alice", Amount: d(10.00)}}
		exactBorrowers := []Borrower{
			{UserID: "bob", Amount: d(5.00)},
			{UserID: "charlie", Amount: d(4.00)},
		}

		_, err := CalculateSplit(payers, SplitExact, nil, exactBorrowers, nil)
		assert.ErrorIs(t, err, ErrUnbalancedTransaction)
	})
}

func TestCalculateSplit_Shares(t *testing.T) {
	t.Run("valid shares split", func(t *testing.T) {
		// Total 10.00: Alice 1 share, Bob 3 shares (Total 4 shares) -> Alice 2.50, Bob 7.50
		payers := []Payer{{UserID: "charlie", Amount: d(10.00)}}
		sharesBorrowers := []Borrower{
			{UserID: "alice", Shares: 1},
			{UserID: "bob", Shares: 3},
		}

		entries, err := CalculateSplit(payers, SplitShares, nil, nil, sharesBorrowers)
		require.NoError(t, err)

		entryMap := make(map[string]Entry)
		for _, e := range entries {
			entryMap[e.UserID] = e
		}

		assert.True(t, d(2.50).Equal(entryMap["alice"].OwedAmount))
		assert.True(t, d(7.50).Equal(entryMap["bob"].OwedAmount))
	})
}
