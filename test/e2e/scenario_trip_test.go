package e2e

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"splitle/internal/adapters/fx"
	"splitle/internal/adapters/storage/memory"
	"splitle/internal/core/domain"
	"splitle/internal/core/ports"
	"splitle/internal/core/service"
)

func d(val float64) decimal.Decimal {
	return decimal.NewFromFloat(val)
}

// TestE2E_GroupTripLifecycle simulates a complete real-world international group trip lifecycle
// with 4 friends (Alice, Bob, Charlie, Dave), covering all split types:
// 1. SplitEqual
// 2. SplitExact
// 3. SplitShares
// as well as multi-currency expenses (EUR, USD, JPY), multi-payer splits, soft deletes,
// interim settlements, and final Bitmask DP debt resolution.
func TestE2E_GroupTripLifecycle(t *testing.T) {
	ctx := context.Background()

	// Initialize Hexagonal Ports & Adapters
	repo := memory.NewMemoryRepository()
	fxProvider := fx.NewStaticFXProvider()
	fxProvider.SetRateFloat("USD", "EUR", 0.9200) // 1 USD = 0.92 EUR
	fxProvider.SetRateFloat("JPY", "EUR", 0.0062) // 1 JPY = 0.0062 EUR
	fxProvider.SetRateFloat("EUR", "USD", 1.0870) // 1 EUR = 1.0870 USD

	svc := service.NewExpenseService(repo, fxProvider)

	groupID := "tg-chat-trip-japan-2026"
	baseCurrency := "EUR"

	t.Log("--- Step 1: [SplitEqual] Alice books flights for all 4 in EUR (€1,200.00) ---")
	tx1, err := svc.RecordExpense(ctx, ports.RecordExpenseParams{
		GroupID:           groupID,
		CreatedBy:         "alice",
		Description:       "Flights to Tokyo",
		Currency:          "EUR",
		BaseCurrency:      baseCurrency,
		Payers:            []domain.Payer{{UserID: "alice", Amount: d(1200.00)}}, // €1,200.00
		SplitType:         domain.SplitEqual,
		EqualParticipants: []string{"alice", "bob", "charlie", "dave"},
	})
	require.NoError(t, err)
	assert.True(t, d(1200.00).Equal(tx1.TotalPaid()))

	// Verify interim balances: Alice is +900, everyone else is -300
	bal1, err := svc.GetGroupBalances(ctx, groupID, nil)
	require.NoError(t, err)
	assert.True(t, d(900.00).Equal(bal1.Balances["alice"].NetBalance))
	assert.True(t, d(-300.00).Equal(bal1.Balances["bob"].NetBalance))
	assert.True(t, d(-300.00).Equal(bal1.Balances["charlie"].NetBalance))
	assert.True(t, d(-300.00).Equal(bal1.Balances["dave"].NetBalance))

	t.Log("--- Step 2: [SplitExact] Bob books Hotel in Tokyo in USD ($800.00 @ 0.92 = €736.00) with custom room rates ---")
	// Alice: Master Suite ($300), Bob: Deluxe ($200), Charlie: Standard ($150), Dave: Standard ($150) -> Total $800.00
	tx2, err := svc.RecordExpense(ctx, ports.RecordExpenseParams{
		GroupID:      groupID,
		CreatedBy:    "bob",
		Description:  "Tokyo Hotel Booking (Unequal Rooms)",
		Currency:     "USD",
		BaseCurrency: baseCurrency,
		Payers:       []domain.Payer{{UserID: "bob", Amount: d(800.00)}}, // $800.00
		SplitType:    domain.SplitExact,
		ExactBorrowers: []domain.Borrower{
			{UserID: "alice", Amount: d(300.00)},   // $300.00
			{UserID: "bob", Amount: d(200.00)},     // $200.00
			{UserID: "charlie", Amount: d(150.00)}, // $150.00
			{UserID: "dave", Amount: d(150.00)},    // $150.00
		},
	})
	require.NoError(t, err)
	assert.True(t, d(0.92).Equal(tx2.ExchangeRate))
	assert.True(t, d(800.00).Equal(tx2.TotalPaid()))

	t.Log("--- Step 3: [SplitShares] Charlie pays Sushi in JPY (¥40,000 @ 0.0062 = €248.00) with weighted shares ---")
	// Alice drank premium sake (2 shares), Bob had omakase (2 shares), Charlie had lunch set (1 share), Dave had nothing (0 shares)
	// Total 5 shares -> Alice: ¥16,000, Bob: ¥16,000, Charlie: ¥8,000
	tx3, err := svc.RecordExpense(ctx, ports.RecordExpenseParams{
		GroupID:      groupID,
		CreatedBy:    "charlie",
		Description:  "Michelin Sushi Dinner (Shared Portions)",
		Currency:     "JPY",
		BaseCurrency: baseCurrency,
		Payers:       []domain.Payer{{UserID: "charlie", Amount: d(40000)}}, // ¥40,000
		SplitType:    domain.SplitShares,
		SharesBorrowers: []domain.Borrower{
			{UserID: "alice", Shares: 2},
			{UserID: "bob", Shares: 2},
			{UserID: "charlie", Shares: 1},
		},
	})
	require.NoError(t, err)
	assert.True(t, d(0.0062).Equal(tx3.ExchangeRate))
	assert.True(t, d(40000).Equal(tx3.TotalPaid()))

	t.Log("--- Step 4: [Multi-Payer SplitEqual] Taxi: Alice pays €30 and Dave pays €20 (Total €50) split 4 ways ---")
	tx4, err := svc.RecordExpense(ctx, ports.RecordExpenseParams{
		GroupID:      groupID,
		CreatedBy:    "alice",
		Description:  "Late Night Airport Taxi",
		Currency:     "EUR",
		BaseCurrency: baseCurrency,
		Payers: []domain.Payer{
			{UserID: "alice", Amount: d(30.00)},
			{UserID: "dave", Amount: d(20.00)},
		},
		SplitType:         domain.SplitEqual,
		EqualParticipants: []string{"alice", "bob", "charlie", "dave"},
	})
	require.NoError(t, err)
	assert.True(t, d(50.00).Equal(tx4.TotalPaid()))

	t.Log("--- Step 5: Accidental duplicate transaction logged and soft deleted ---")
	txMistake, err := svc.RecordExpense(ctx, ports.RecordExpenseParams{
		GroupID:           groupID,
		CreatedBy:         "bob",
		Description:       "Accidental Duplicate Shinkansen Ticket",
		Currency:          "EUR",
		BaseCurrency:      baseCurrency,
		Payers:            []domain.Payer{{UserID: "bob", Amount: d(200.00)}},
		SplitType:         domain.SplitEqual,
		EqualParticipants: []string{"alice", "bob", "charlie", "dave"},
	})
	require.NoError(t, err)

	// Total group spend with mistake: 2234.00 + 200.00 = 2434.00
	balWithMistake, err := svc.GetGroupBalances(ctx, groupID, nil)
	require.NoError(t, err)
	assert.True(t, d(2434.00).Equal(balWithMistake.TotalGroupSpend))

	// Delete mistake
	err = svc.DeleteTransaction(ctx, txMistake.ID)
	require.NoError(t, err)

	// Assert total group spend is restored to €2,234.00 (1200 + 736 + 248 + 50 = €2,234.00)
	balAfterDelete, err := svc.GetGroupBalances(ctx, groupID, nil)
	require.NoError(t, err)
	assert.True(t, d(2234.00).Equal(balAfterDelete.TotalGroupSpend))

	t.Log("--- Step 6: Dave makes an interim cash settlement of €150.00 to Alice ---")
	txSettlement1, err := svc.RecordSettlement(ctx, groupID, "dave", "dave", "alice", d(150.00), "EUR", baseCurrency, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, txSettlement1.ID)

	t.Log("--- Step 7: Inspect Final Pre-Settlement Balances & Verify Invariants ---")
	finalBal, err := svc.GetGroupBalances(ctx, groupID, nil)
	require.NoError(t, err)

	sumNet := decimal.Zero
	for uid, b := range finalBal.Balances {
		t.Logf("User: %-8s | Paid: %-10s | Owed: %-10s | Net: %s",
			uid,
			domain.FormatAmount(b.PaidAmount, baseCurrency),
			domain.FormatAmount(b.OwedAmount, baseCurrency),
			domain.FormatAmount(b.NetBalance, baseCurrency),
		)
		sumNet = sumNet.Add(b.NetBalance)
	}
	assert.True(t, decimal.Zero.Equal(sumNet), "Total net balances across group must strictly sum to 0")

	t.Log("--- Step 8: Query group balances dynamically in USD ---")
	usdBal, err := svc.GetGroupBalances(ctx, groupID, &[]string{"USD"}[0])
	require.NoError(t, err)
	assert.Equal(t, "USD", usdBal.BaseCurrency)

	t.Log("--- Step 9: Generate Optimal Debt Simplification via Bitmask DP ---")
	debts, err := svc.GetSimplifiedDebts(ctx, groupID, nil)
	require.NoError(t, err)
	require.NotEmpty(t, debts)
	t.Logf("Bitmask DP generated %d minimal settlement transfers:", len(debts))
	for _, d := range debts {
		t.Logf("  👉 %s pays %s: %s", d.FromUser, d.ToUser, domain.FormatAmount(d.Amount, d.Currency))
	}

	t.Log("--- Step 10: Execute all recommended settlements until group reaches 0 debt ---")
	for _, debt := range debts {
		_, err := svc.RecordSettlement(
			ctx,
			groupID,
			debt.FromUser,
			debt.FromUser,
			debt.ToUser,
			debt.Amount,
			debt.Currency,
			baseCurrency,
			nil,
		)
		require.NoError(t, err)
	}

	t.Log("--- Step 11: Assert 100% Zero Balances & No Remaining Debts ---")
	settledBal, err := svc.GetGroupBalances(ctx, groupID, nil)
	require.NoError(t, err)
	for uid, b := range settledBal.Balances {
		assert.True(t, decimal.Zero.Equal(b.NetBalance), "User %s should have exactly 0 net balance after full settlement", uid)
	}

	t.Log("--- Step 12: Validate Group Analytics ---")
	analytics, err := svc.GetGroupAnalytics(ctx, groupID, domain.AnalyticsFilter{})
	require.NoError(t, err)
	assert.Equal(t, groupID, analytics.GroupID)
	assert.Equal(t, "EUR", analytics.Currency)
	// Pure group spend: 1200 + 736 + 248 + 50 = 2234.00 EUR
	assert.True(t, d(2234.00).Equal(analytics.TotalGroupSpend))
	assert.Equal(t, 4, analytics.ExpenseCount)
	assert.Equal(t, 4, len(analytics.UserSummaries)) // Alice, Bob, Charlie, Dave
	assert.Equal(t, "alice", analytics.TopPayerID)
	assert.True(t, d(1230.00).Equal(analytics.TopPayerAmount))

	t.Log("🎉 E2E Group Trip Lifecycle Scenario passed successfully with SplitEqual, SplitExact, SplitShares, and Analytics!")
}
