package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"splitle/internal/core/domain"
	"splitle/internal/core/ports"
)

type expenseService struct {
	repo ports.TransactionRepository
	fx   ports.ExchangeRateProvider
}

var _ ports.ExpenseService = (*expenseService)(nil)

// NewExpenseService creates a new instance of the core ExpenseService.
func NewExpenseService(repo ports.TransactionRepository, fx ports.ExchangeRateProvider) ports.ExpenseService {
	return &expenseService{
		repo: repo,
		fx:   fx,
	}
}

// RecordExpense validates, splits, snapshots exchange rates, and stores an expense transaction.
func (s *expenseService) RecordExpense(ctx context.Context, params ports.RecordExpenseParams) (*domain.Transaction, error) {
	groupID := strings.TrimSpace(params.GroupID)
	if groupID == "" {
		return nil, domain.ErrEmptyGroupID
	}
	createdBy := strings.TrimSpace(params.CreatedBy)
	if createdBy == "" {
		return nil, domain.ErrEmptyCreatedBy
	}
	desc := strings.TrimSpace(params.Description)
	if desc == "" {
		return nil, domain.ErrEmptyDescription
	}

	currency := domain.NormalizeCurrency(params.Currency)
	if currency == "" {
		currency = "EUR"
	}
	baseCurrency := domain.NormalizeCurrency(params.BaseCurrency)
	if baseCurrency == "" {
		baseCurrency = "EUR"
	}

	// Determine exchange rate snapshot
	var rate decimal.Decimal
	if params.CustomExchangeRate != nil && params.CustomExchangeRate.IsPositive() {
		rate = *params.CustomExchangeRate
	} else {
		var err error
		rate, err = s.fx.GetRate(ctx, currency, baseCurrency)
		if err != nil {
			return nil, err
		}
	}

	if !rate.IsPositive() {
		return nil, domain.ErrInvalidExchangeRate
	}

	// Calculate per-user entries in native currency
	entries, err := domain.CalculateSplit(
		params.Payers,
		params.SplitType,
		params.EqualParticipants,
		params.ExactBorrowers,
		params.SharesBorrowers,
	)
	if err != nil {
		return nil, err
	}

	tx := &domain.Transaction{
		ID:           uuid.New(),
		GroupID:      groupID,
		Description:  desc,
		Currency:     currency,
		BaseCurrency: baseCurrency,
		ExchangeRate: rate,
		CreatedBy:    createdBy,
		CreatedAt:    time.Now().UTC(),
		Entries:      entries,
	}

	if err := s.repo.CreateTransaction(ctx, tx); err != nil {
		return nil, err
	}

	return tx, nil
}

// RecordSettlement records a direct payment transaction between two users.
func (s *expenseService) RecordSettlement(
	ctx context.Context,
	groupID, createdBy string,
	fromUser, toUser string,
	amount decimal.Decimal,
	currency, baseCurrency string,
	customRate *decimal.Decimal,
) (*domain.Transaction, error) {
	fromUser = strings.TrimSpace(fromUser)
	toUser = strings.TrimSpace(toUser)
	if fromUser == "" || toUser == "" {
		return nil, domain.ErrEmptyCreatedBy
	}
	if fromUser == toUser {
		return nil, domain.ErrSamePayerAndReceiver
	}
	if !amount.IsPositive() {
		return nil, domain.ErrInvalidAmount
	}

	cur := domain.NormalizeCurrency(currency)
	if cur == "" {
		cur = "EUR"
	}
	baseCur := domain.NormalizeCurrency(baseCurrency)
	if baseCur == "" {
		baseCur = "EUR"
	}

	var rate decimal.Decimal
	if customRate != nil && customRate.IsPositive() {
		rate = *customRate
	} else {
		var err error
		rate, err = s.fx.GetRate(ctx, cur, baseCur)
		if err != nil {
			return nil, err
		}
	}

	if !rate.IsPositive() {
		return nil, domain.ErrInvalidExchangeRate
	}

	txID := uuid.New()
	tx := &domain.Transaction{
		ID:           txID,
		GroupID:      strings.TrimSpace(groupID),
		Description:  fmt.Sprintf("Settlement: %s -> %s", fromUser, toUser),
		Currency:     cur,
		BaseCurrency: baseCur,
		ExchangeRate: rate,
		CreatedBy:    strings.TrimSpace(createdBy),
		CreatedAt:    time.Now().UTC(),
		Entries: []domain.Entry{
			{
				ID:            uuid.New(),
				TransactionID: txID,
				UserID:        fromUser,
				PaidAmount:    amount,
				OwedAmount:    decimal.Zero,
			},
			{
				ID:            uuid.New(),
				TransactionID: txID,
				UserID:        toUser,
				PaidAmount:    decimal.Zero,
				OwedAmount:    amount,
			},
		},
	}

	if err := s.repo.CreateTransaction(ctx, tx); err != nil {
		return nil, err
	}

	return tx, nil
}

// convertEntriesToBase converts a transaction's entries to the target base currency,
// strictly guaranteeing that sum(PaidInBase) == sum(OwedInBase) == TotalPaidInBase.
func convertEntriesToBase(tx *domain.Transaction, targetBaseCurrency string, fx ports.ExchangeRateProvider, ctx context.Context) (map[string]decimal.Decimal, map[string]decimal.Decimal, decimal.Decimal, error) {
	totalNative := tx.TotalPaid()
	if totalNative.IsZero() {
		return map[string]decimal.Decimal{}, map[string]decimal.Decimal{}, decimal.Zero, nil
	}

	// 1. Total in tx.BaseCurrency
	totalInTxBase, err := domain.ConvertAmount(totalNative, tx.Currency, tx.BaseCurrency, tx.ExchangeRate)
	if err != nil {
		return nil, nil, decimal.Zero, err
	}

	// 2. If targetBaseCurrency differs from tx.BaseCurrency, convert total to targetBaseCurrency
	finalTotal := totalInTxBase
	if targetBaseCurrency != tx.BaseCurrency {
		rate, err := fx.GetRate(ctx, tx.BaseCurrency, targetBaseCurrency)
		if err != nil {
			return nil, nil, decimal.Zero, err
		}
		finalTotal, err = domain.ConvertAmount(totalInTxBase, tx.BaseCurrency, targetBaseCurrency, rate)
		if err != nil {
			return nil, nil, decimal.Zero, err
		}
	}

	// Collect and sort entries by UserID for deterministic remainder distribution
	sortedEntries := make([]domain.Entry, len(tx.Entries))
	copy(sortedEntries, tx.Entries)
	sort.Slice(sortedEntries, func(i, j int) bool {
		return sortedEntries[i].UserID < sortedEntries[j].UserID
	})

	paidMap := distributeProportional(finalTotal, sortedEntries, true, totalNative)
	owedMap := distributeProportional(finalTotal, sortedEntries, false, totalNative)

	return paidMap, owedMap, finalTotal, nil
}

func distributeProportional(total decimal.Decimal, entries []domain.Entry, isPaid bool, totalNative decimal.Decimal) map[string]decimal.Decimal {
	res := make(map[string]decimal.Decimal)
	scale := total.Exponent()
	precision := int32(2)
	if scale < -2 {
		precision = -scale
		if precision > domain.DefaultPrecision {
			precision = domain.DefaultPrecision
		}
	}
	unitMultiplier := decimal.New(1, precision)
	totalUnits := total.Mul(unitMultiplier).IntPart()

	var activeEntries []domain.Entry
	for _, e := range entries {
		amt := e.PaidAmount
		if !isPaid {
			amt = e.OwedAmount
		}
		if amt.IsPositive() {
			activeEntries = append(activeEntries, e)
		}
	}

	if len(activeEntries) == 0 {
		return res
	}

	var allocatedUnits int64
	for _, e := range activeEntries {
		amt := e.PaidAmount
		if !isPaid {
			amt = e.OwedAmount
		}
		shareUnits := amt.Mul(decimal.NewFromInt(totalUnits)).Div(totalNative).IntPart()
		res[e.UserID] = decimal.New(shareUnits, -precision)
		allocatedUnits += shareUnits
	}

	remUnits := totalUnits - allocatedUnits
	oneUnit := decimal.New(1, -precision)
	for i := 0; i < int(remUnits); i++ {
		idx := i % len(activeEntries)
		uid := activeEntries[idx].UserID
		res[uid] = res[uid].Add(oneUnit)
	}

	return res
}

// GetGroupBalances computes aggregate balances per user in the group base currency (or target currency).
func (s *expenseService) GetGroupBalances(ctx context.Context, groupID string, targetCurrency *string) (*domain.GroupBalance, error) {
	transactions, err := s.repo.GetTransactionsByGroup(ctx, strings.TrimSpace(groupID))
	if err != nil {
		return nil, err
	}

	baseCurrency := "EUR"
	if targetCurrency != nil && strings.TrimSpace(*targetCurrency) != "" {
		baseCurrency = domain.NormalizeCurrency(*targetCurrency)
	} else if len(transactions) > 0 && transactions[0].BaseCurrency != "" {
		baseCurrency = transactions[0].BaseCurrency
	}

	balanceMap := make(map[string]domain.UserBalance)
	totalSpend := decimal.Zero

	for _, tx := range transactions {
		paidMap, owedMap, txTotalInBase, err := convertEntriesToBase(&tx, baseCurrency, s.fx, ctx)
		if err != nil {
			return nil, err
		}

		totalSpend = totalSpend.Add(txTotalInBase)

		// Aggregate for all users in this transaction
		allUsers := make(map[string]struct{})
		for u := range paidMap {
			allUsers[u] = struct{}{}
		}
		for u := range owedMap {
			allUsers[u] = struct{}{}
		}

		for u := range allUsers {
			userBal := balanceMap[u]
			userBal.UserID = u
			userBal.Currency = baseCurrency
			if paid, ok := paidMap[u]; ok {
				userBal.PaidAmount = userBal.PaidAmount.Add(paid)
			}
			if owed, ok := owedMap[u]; ok {
				userBal.OwedAmount = userBal.OwedAmount.Add(owed)
			}
			userBal.NetBalance = userBal.PaidAmount.Sub(userBal.OwedAmount)
			balanceMap[u] = userBal
		}
	}

	return &domain.GroupBalance{
		GroupID:         groupID,
		BaseCurrency:    baseCurrency,
		Balances:        balanceMap,
		TotalGroupSpend: totalSpend,
	}, nil
}

// GetGroupAnalytics computes spending statistics, user breakdowns, percentages, and rankings.
func (s *expenseService) GetGroupAnalytics(ctx context.Context, groupID string, filter domain.AnalyticsFilter) (*domain.GroupAnalytics, error) {
	transactions, err := s.repo.GetTransactionsByGroup(ctx, strings.TrimSpace(groupID))
	if err != nil {
		return nil, err
	}

	baseCurrency := "EUR"
	if filter.TargetCurrency != nil && strings.TrimSpace(*filter.TargetCurrency) != "" {
		baseCurrency = domain.NormalizeCurrency(*filter.TargetCurrency)
	} else if len(transactions) > 0 && transactions[0].BaseCurrency != "" {
		baseCurrency = transactions[0].BaseCurrency
	}

	userSummaries := make(map[string]domain.UserSpendingSummary)
	totalGroupSpend := decimal.Zero
	totalSettlements := decimal.Zero
	expenseCount := 0
	settlementCount := 0
	totalActiveTransactions := 0

	for _, tx := range transactions {
		// Apply date filters if set
		if filter.StartDate != nil && tx.CreatedAt.Before(*filter.StartDate) {
			continue
		}
		if filter.EndDate != nil && tx.CreatedAt.After(*filter.EndDate) {
			continue
		}

		paidMap, owedMap, txTotalInBase, err := convertEntriesToBase(&tx, baseCurrency, s.fx, ctx)
		if err != nil {
			return nil, err
		}

		totalActiveTransactions++
		isSettlement := tx.IsSettlement()
		if isSettlement {
			settlementCount++
			totalSettlements = totalSettlements.Add(txTotalInBase)
		} else {
			expenseCount++
			totalGroupSpend = totalGroupSpend.Add(txTotalInBase)
		}

		allUsers := make(map[string]struct{})
		for u := range paidMap {
			allUsers[u] = struct{}{}
		}
		for u := range owedMap {
			allUsers[u] = struct{}{}
		}

		for u := range allUsers {
			summary := userSummaries[u]
			summary.UserID = u
			summary.Currency = baseCurrency

			paid := paidMap[u]
			owed := owedMap[u]

			if isSettlement {
				summary.SettlementCount++
				summary.TotalSettlementsPaid = summary.TotalSettlementsPaid.Add(paid)
				summary.TotalSettlementsRecv = summary.TotalSettlementsRecv.Add(owed)
			} else {
				summary.ExpenseCount++
				summary.TotalPaid = summary.TotalPaid.Add(paid)
				summary.TotalSpent = summary.TotalSpent.Add(owed)
			}

			summary.NetBalance = summary.TotalPaid.Add(summary.TotalSettlementsPaid).Sub(summary.TotalSpent).Sub(summary.TotalSettlementsRecv)
			userSummaries[u] = summary
		}
	}

	var topPayerID string
	topPayerAmount := decimal.Zero
	var topSpenderID string
	topSpenderAmount := decimal.Zero

	// Calculate percentages and rankings
	for u, summary := range userSummaries {
		summary.SpendingPercentage = domain.CalculatePercentage(summary.TotalSpent, totalGroupSpend)
		summary.PaidPercentage = domain.CalculatePercentage(summary.TotalPaid, totalGroupSpend)
		userSummaries[u] = summary

		if summary.TotalPaid.GreaterThan(topPayerAmount) || (topPayerID == "" && summary.TotalPaid.IsPositive()) {
			topPayerID = u
			topPayerAmount = summary.TotalPaid
		}
		if summary.TotalSpent.GreaterThan(topSpenderAmount) || (topSpenderID == "" && summary.TotalSpent.IsPositive()) {
			topSpenderID = u
			topSpenderAmount = summary.TotalSpent
		}
	}

	avgExpense := decimal.Zero
	if expenseCount > 0 {
		avgExpense = totalGroupSpend.DivRound(decimal.NewFromInt(int64(expenseCount)), 2)
	}

	return &domain.GroupAnalytics{
		GroupID:              groupID,
		Currency:             baseCurrency,
		TotalGroupSpend:      totalGroupSpend,
		TotalSettlements:     totalSettlements,
		TotalTransactions:    totalActiveTransactions,
		ExpenseCount:         expenseCount,
		SettlementCount:      settlementCount,
		AverageExpenseAmount: avgExpense,
		UserSummaries:        userSummaries,
		TopPayerID:           topPayerID,
		TopPayerAmount:       topPayerAmount,
		TopSpenderID:         topSpenderID,
		TopSpenderAmount:     topSpenderAmount,
	}, nil
}

// GetUserSpending retrieves the spending summary for a single user in a group.
func (s *expenseService) GetUserSpending(ctx context.Context, groupID, userID string, filter domain.AnalyticsFilter) (*domain.UserSpendingSummary, error) {
	analytics, err := s.GetGroupAnalytics(ctx, groupID, filter)
	if err != nil {
		return nil, err
	}

	userID = strings.TrimSpace(userID)
	if summary, ok := analytics.UserSummaries[userID]; ok {
		return &summary, nil
	}

	return &domain.UserSpendingSummary{
		UserID:               userID,
		Currency:             analytics.Currency,
		TotalPaid:            decimal.Zero,
		TotalSpent:           decimal.Zero,
		NetBalance:           decimal.Zero,
		TotalSettlementsPaid: decimal.Zero,
		TotalSettlementsRecv: decimal.Zero,
		SpendingPercentage:   decimal.Zero,
		PaidPercentage:       decimal.Zero,
		ExpenseCount:         0,
		SettlementCount:      0,
	}, nil
}

// GetSimplifiedDebts computes the minimal debt transfers to settle all net balances.
func (s *expenseService) GetSimplifiedDebts(ctx context.Context, groupID string, targetCurrency *string) ([]domain.DebtTransfer, error) {
	groupBalance, err := s.GetGroupBalances(ctx, groupID, targetCurrency)
	if err != nil {
		return nil, err
	}

	netMap := make(map[string]decimal.Decimal, len(groupBalance.Balances))
	for uid, b := range groupBalance.Balances {
		netMap[uid] = b.NetBalance
	}

	return domain.SimplifyDebtsOptimal(netMap, groupBalance.BaseCurrency)
}

// GetGroupTransactions retrieves active transactions for a group.
func (s *expenseService) GetGroupTransactions(ctx context.Context, groupID string) ([]domain.Transaction, error) {
	return s.repo.GetTransactionsByGroup(ctx, strings.TrimSpace(groupID))
}

// GetTransaction retrieves a transaction by ID.
func (s *expenseService) GetTransaction(ctx context.Context, id uuid.UUID) (*domain.Transaction, error) {
	return s.repo.GetTransactionByID(ctx, id)
}

// DeleteTransaction soft-deletes a transaction.
func (s *expenseService) DeleteTransaction(ctx context.Context, id uuid.UUID) error {
	return s.repo.SoftDeleteTransaction(ctx, id)
}
