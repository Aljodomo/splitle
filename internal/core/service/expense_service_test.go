package service

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"splitle/internal/adapters/fx"
	"splitle/internal/adapters/storage/memory"
	"splitle/internal/core/domain"
	"splitle/internal/core/ports"
)

func setupTestService() ports.ExpenseService {
	repo := memory.NewMemoryRepository()
	fxProvider := fx.NewStaticFXProvider()
	return NewExpenseService(repo, fxProvider)
}

func d(val float64) decimal.Decimal {
	return decimal.NewFromFloat(val)
}

func TestExpenseService_RecordExpense(t *testing.T) {
	ctx := context.Background()
	svc := setupTestService()

	t.Run("record multi-payer equal split expense", func(t *testing.T) {
		tx, err := svc.RecordExpense(ctx, ports.RecordExpenseParams{
			GroupID:      "group-1",
			CreatedBy:    "alice",
			Description:  "Dinner with friends",
			Currency:     "EUR",
			BaseCurrency: "EUR",
			Payers: []domain.Payer{
				{UserID: "alice", Amount: d(60.00)},
				{UserID: "bob", Amount: d(40.00)},
			},
			SplitType:         domain.SplitEqual,
			EqualParticipants: []string{"alice", "bob", "charlie"},
		})

		require.NoError(t, err)
		assert.NotEmpty(t, tx.ID)
		assert.Equal(t, "group-1", tx.GroupID)
		assert.True(t, decimal.NewFromInt(1).Equal(tx.ExchangeRate))
		assert.Len(t, tx.Entries, 3)

		// Check balances
		groupBal, err := svc.GetGroupBalances(ctx, "group-1", nil)
		require.NoError(t, err)
		assert.True(t, d(100.00).Equal(groupBal.TotalGroupSpend))

		// alice: paid 60.00, owed 33.34 -> net +26.66
		// bob: paid 40.00, owed 33.33 -> net +6.67
		// charlie: paid 0, owed 33.33 -> net -33.33
		assert.True(t, d(26.66).Equal(groupBal.Balances["alice"].NetBalance))
		assert.True(t, d(6.67).Equal(groupBal.Balances["bob"].NetBalance))
		assert.True(t, d(-33.33).Equal(groupBal.Balances["charlie"].NetBalance))
	})

	t.Run("record expense in foreign currency (JPY to EUR)", func(t *testing.T) {
		// 15,000 JPY @ 0.0062 = 93.00 EUR
		// Paid by Alice, split equally with Bob (46.50 each)
		tx, err := svc.RecordExpense(ctx, ports.RecordExpenseParams{
			GroupID:           "group-tokyo",
			CreatedBy:         "alice",
			Description:       "Tokyo Ramen",
			Currency:          "JPY",
			BaseCurrency:      "EUR",
			Payers:            []domain.Payer{{UserID: "alice", Amount: d(15000)}},
			SplitType:         domain.SplitEqual,
			EqualParticipants: []string{"alice", "bob"},
		})

		require.NoError(t, err)
		assert.True(t, d(0.0062).Equal(tx.ExchangeRate))

		groupBal, err := svc.GetGroupBalances(ctx, "group-tokyo", nil)
		require.NoError(t, err)
		assert.True(t, d(93.00).Equal(groupBal.TotalGroupSpend))
		assert.True(t, d(46.50).Equal(groupBal.Balances["alice"].NetBalance))
		assert.True(t, d(-46.50).Equal(groupBal.Balances["bob"].NetBalance))
	})
}

func TestExpenseService_RecordSettlement(t *testing.T) {
	ctx := context.Background()
	svc := setupTestService()

	// Alice paid 100 EUR for Alice & Bob
	_, err := svc.RecordExpense(ctx, ports.RecordExpenseParams{
		GroupID:           "group-2",
		CreatedBy:         "alice",
		Description:       "Hotel",
		Currency:          "EUR",
		BaseCurrency:      "EUR",
		Payers:            []domain.Payer{{UserID: "alice", Amount: d(100.00)}},
		SplitType:         domain.SplitEqual,
		EqualParticipants: []string{"alice", "bob"},
	})
	require.NoError(t, err)

	// Bob owes Alice 50.00 EUR. Bob records partial settlement of 20.00 EUR to Alice
	settlementTx, err := svc.RecordSettlement(ctx, "group-2", "bob", "bob", "alice", d(20.00), "EUR", "EUR", nil)
	require.NoError(t, err)
	assert.Contains(t, settlementTx.Description, "Settlement: bob -> alice")

	// Balances: Alice should now be +30.00, Bob should be -30.00
	groupBal, err := svc.GetGroupBalances(ctx, "group-2", nil)
	require.NoError(t, err)
	assert.True(t, d(30.00).Equal(groupBal.Balances["alice"].NetBalance))
	assert.True(t, d(-30.00).Equal(groupBal.Balances["bob"].NetBalance))

	// Simplified debt should show Bob owes Alice 30.00 EUR
	debts, err := svc.GetSimplifiedDebts(ctx, "group-2", nil)
	require.NoError(t, err)
	require.Len(t, debts, 1)
	assert.Equal(t, "bob", debts[0].FromUser)
	assert.Equal(t, "alice", debts[0].ToUser)
	assert.True(t, d(30.00).Equal(debts[0].Amount))
}

func TestExpenseService_SoftDelete(t *testing.T) {
	ctx := context.Background()
	svc := setupTestService()

	tx, err := svc.RecordExpense(ctx, ports.RecordExpenseParams{
		GroupID:           "group-del",
		CreatedBy:         "alice",
		Description:       "Mistake expense",
		Currency:          "EUR",
		BaseCurrency:      "EUR",
		Payers:            []domain.Payer{{UserID: "alice", Amount: d(50.00)}},
		SplitType:         domain.SplitEqual,
		EqualParticipants: []string{"alice", "bob"},
	})
	require.NoError(t, err)

	// Check balance before delete
	balBefore, err := svc.GetGroupBalances(ctx, "group-del", nil)
	require.NoError(t, err)
	assert.True(t, d(25.00).Equal(balBefore.Balances["alice"].NetBalance))

	// Soft delete
	err = svc.DeleteTransaction(ctx, tx.ID)
	require.NoError(t, err)

	// Check balance after delete -> 0
	balAfter, err := svc.GetGroupBalances(ctx, "group-del", nil)
	require.NoError(t, err)
	assert.Empty(t, balAfter.Balances)
	assert.True(t, decimal.Zero.Equal(balAfter.TotalGroupSpend))
}

func TestExpenseService_Analytics(t *testing.T) {
	ctx := context.Background()
	svc := setupTestService()

	// 1. Expense 1: Alice pays 120 EUR, split equally among Alice, Bob, Charlie (40 EUR each)
	_, err := svc.RecordExpense(ctx, ports.RecordExpenseParams{
		GroupID:           "group-analytics",
		CreatedBy:         "alice",
		Description:       "Dinner",
		Currency:          "EUR",
		BaseCurrency:      "EUR",
		Payers:            []domain.Payer{{UserID: "alice", Amount: d(120.00)}},
		SplitType:         domain.SplitEqual,
		EqualParticipants: []string{"alice", "bob", "charlie"},
	})
	require.NoError(t, err)

	// 2. Expense 2: Bob pays 80 EUR, split Alice 20 EUR, Bob 60 EUR
	_, err = svc.RecordExpense(ctx, ports.RecordExpenseParams{
		GroupID:      "group-analytics",
		CreatedBy:    "bob",
		Description:  "Groceries",
		Currency:     "EUR",
		BaseCurrency: "EUR",
		Payers:       []domain.Payer{{UserID: "bob", Amount: d(80.00)}},
		SplitType:    domain.SplitExact,
		ExactBorrowers: []domain.Borrower{
			{UserID: "alice", Amount: d(20.00)},
			{UserID: "bob", Amount: d(60.00)},
		},
	})
	require.NoError(t, err)

	// 3. Settlement: Charlie pays Alice 30 EUR direct debt settlement
	_, err = svc.RecordSettlement(ctx, "group-analytics", "charlie", "charlie", "alice", d(30.00), "EUR", "EUR", nil)
	require.NoError(t, err)

	t.Run("group analytics calculations", func(t *testing.T) {
		analytics, err := svc.GetGroupAnalytics(ctx, "group-analytics", domain.AnalyticsFilter{})
		require.NoError(t, err)

		// Group Totals (Expenses: 120 + 80 = 200 EUR, Settlements: 30 EUR)
		assert.Equal(t, "group-analytics", analytics.GroupID)
		assert.Equal(t, "EUR", analytics.Currency)
		assert.True(t, d(200.00).Equal(analytics.TotalGroupSpend))
		assert.True(t, d(30.00).Equal(analytics.TotalSettlements))
		assert.Equal(t, 3, analytics.TotalTransactions)
		assert.Equal(t, 2, analytics.ExpenseCount)
		assert.Equal(t, 1, analytics.SettlementCount)
		assert.True(t, d(100.00).Equal(analytics.AverageExpenseAmount))

		// Alice metrics:
		// - Paid for expenses: 120.00 (60% of total group spend)
		// - Spent (consumed): 40.00 + 20.00 = 60.00 (30% of total group spend)
		// - Settlement received: 30.00
		// - Net balance: (120 - 60) - 30 = +30.00 (TotalPaid - TotalSpent - SettlementsRecv)
		alice := analytics.UserSummaries["alice"]
		assert.Equal(t, "alice", alice.UserID)
		assert.True(t, d(120.00).Equal(alice.TotalPaid))
		assert.True(t, d(60.00).Equal(alice.TotalSpent))
		assert.True(t, d(60.00).Equal(alice.PaidPercentage))
		assert.True(t, d(30.00).Equal(alice.SpendingPercentage))
		assert.True(t, d(30.00).Equal(alice.TotalSettlementsRecv))
		assert.True(t, decimal.Zero.Equal(alice.TotalSettlementsPaid))
		assert.True(t, d(30.00).Equal(alice.NetBalance))
		assert.Equal(t, 2, alice.ExpenseCount)
		assert.Equal(t, 1, alice.SettlementCount)

		// Bob metrics:
		// - Paid for expenses: 80.00 (40% of total group spend)
		// - Spent (consumed): 40.00 + 60.00 = 100.00 (50% of total group spend)
		// - Net balance: 80 - 100 = -20.00
		bob := analytics.UserSummaries["bob"]
		assert.Equal(t, "bob", bob.UserID)
		assert.True(t, d(80.00).Equal(bob.TotalPaid))
		assert.True(t, d(100.00).Equal(bob.TotalSpent))
		assert.True(t, d(40.00).Equal(bob.PaidPercentage))
		assert.True(t, d(50.00).Equal(bob.SpendingPercentage))
		assert.True(t, d(-20.00).Equal(bob.NetBalance))
		assert.Equal(t, 2, bob.ExpenseCount)
		assert.Equal(t, 0, bob.SettlementCount)

		// Charlie metrics:
		// - Paid for expenses: 0.00 (0%)
		// - Spent (consumed): 40.00 (20% of total group spend)
		// - Settlement paid: 30.00
		// - Net balance: -40.00 + 30.00 = -10.00
		charlie := analytics.UserSummaries["charlie"]
		assert.Equal(t, "charlie", charlie.UserID)
		assert.True(t, decimal.Zero.Equal(charlie.TotalPaid))
		assert.True(t, d(40.00).Equal(charlie.TotalSpent))
		assert.True(t, decimal.Zero.Equal(charlie.PaidPercentage))
		assert.True(t, d(20.00).Equal(charlie.SpendingPercentage))
		assert.True(t, d(30.00).Equal(charlie.TotalSettlementsPaid))
		assert.True(t, d(-10.00).Equal(charlie.NetBalance))
		assert.Equal(t, 1, charlie.ExpenseCount)
		assert.Equal(t, 1, charlie.SettlementCount)

		// Rankings:
		// Top payer: Alice (120 EUR)
		// Top spender: Bob (100 EUR)
		assert.Equal(t, "alice", analytics.TopPayerID)
		assert.True(t, d(120.00).Equal(analytics.TopPayerAmount))
		assert.Equal(t, "bob", analytics.TopSpenderID)
		assert.True(t, d(100.00).Equal(analytics.TopSpenderAmount))
	})

	t.Run("individual user spending query", func(t *testing.T) {
		summary, err := svc.GetUserSpending(ctx, "group-analytics", "bob", domain.AnalyticsFilter{})
		require.NoError(t, err)
		assert.Equal(t, "bob", summary.UserID)
		assert.True(t, d(100.00).Equal(summary.TotalSpent))
		assert.True(t, d(80.00).Equal(summary.TotalPaid))

		// Non-existent user
		nonExistent, err := svc.GetUserSpending(ctx, "group-analytics", "unknown", domain.AnalyticsFilter{})
		require.NoError(t, err)
		assert.Equal(t, "unknown", nonExistent.UserID)
		assert.True(t, decimal.Zero.Equal(nonExistent.TotalSpent))
		assert.True(t, decimal.Zero.Equal(nonExistent.TotalPaid))
	})

	t.Run("analytics with target currency conversion (USD)", func(t *testing.T) {
		usd := "USD"
		analytics, err := svc.GetGroupAnalytics(ctx, "group-analytics", domain.AnalyticsFilter{
			TargetCurrency: &usd,
		})
		require.NoError(t, err)
		assert.Equal(t, "USD", analytics.Currency)

		// Rate EUR -> USD = 1.0850
		// Total spend 200 EUR * 1.0850 = 217.00 USD
		assert.True(t, d(217.00).Equal(analytics.TotalGroupSpend))
	})

	t.Run("analytics with date filtering", func(t *testing.T) {
		future := time.Now().Add(24 * time.Hour)
		analytics, err := svc.GetGroupAnalytics(ctx, "group-analytics", domain.AnalyticsFilter{
			StartDate: &future,
		})
		require.NoError(t, err)
		assert.Equal(t, 0, analytics.TotalTransactions)
		assert.True(t, decimal.Zero.Equal(analytics.TotalGroupSpend))
		assert.Empty(t, analytics.UserSummaries)
	})
}

