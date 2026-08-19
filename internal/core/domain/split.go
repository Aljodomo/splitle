package domain

import (
	"sort"
	"strings"

	"github.com/shopspring/decimal"
)

// SplitType represents the method used to split expenses among borrowers.
type SplitType string

const (
	SplitEqual  SplitType = "EQUAL"
	SplitExact  SplitType = "EXACT"
	SplitShares SplitType = "SHARES"
)

// CalculateSplit computes the per-user PaidAmount and OwedAmount entries using exact decimal arithmetic.
// It ensures sum(PaidAmount) == sum(OwedAmount) and distributes non-divisible remainder units deterministically.
func CalculateSplit(
	payers []Payer,
	splitType SplitType,
	equalParticipants []string,
	exactBorrowers []Borrower,
	sharesBorrowers []Borrower,
) ([]Entry, error) {
	if len(payers) == 0 {
		return nil, ErrNoPayers
	}

	totalPaid := decimal.Zero
	paidMap := make(map[string]decimal.Decimal, len(payers))
	for _, p := range payers {
		uid := strings.TrimSpace(p.UserID)
		if uid == "" {
			return nil, ErrEmptyCreatedBy
		}
		if !p.Amount.IsPositive() {
			return nil, ErrInvalidAmount
		}
		paidMap[uid] = paidMap[uid].Add(p.Amount)
		totalPaid = totalPaid.Add(p.Amount)
	}

	if !totalPaid.IsPositive() {
		return nil, ErrInvalidAmount
	}

	owedMap := make(map[string]decimal.Decimal)

	// Determine decimal precision scale for remainder distribution (at least 2, up to DefaultPrecision)
	scale := totalPaid.Exponent()
	precision := int32(2)
	if scale < -2 {
		precision = -scale
		if precision > DefaultPrecision {
			precision = DefaultPrecision
		}
	}
	unitMultiplier := decimal.New(1, precision) // 10^precision

	switch splitType {
	case SplitEqual:
		if len(equalParticipants) == 0 {
			return nil, ErrNoBorrowers
		}

		// Deduplicate and sort participants for deterministic remainder distribution
		uniqueUsers := make(map[string]struct{}, len(equalParticipants))
		sortedUsers := make([]string, 0, len(equalParticipants))
		for _, u := range equalParticipants {
			uid := strings.TrimSpace(u)
			if uid == "" {
				return nil, ErrEmptyCreatedBy
			}
			if _, exists := uniqueUsers[uid]; !exists {
				uniqueUsers[uid] = struct{}{}
				sortedUsers = append(sortedUsers, uid)
			}
		}
		sort.Strings(sortedUsers)

		n := int64(len(sortedUsers))
		totalUnits := totalPaid.Mul(unitMultiplier).IntPart()
		baseUnits := totalUnits / n
		remainderUnits := totalUnits % n

		oneUnit := decimal.New(1, -precision)
		baseAmount := decimal.New(baseUnits, -precision)

		for i, uid := range sortedUsers {
			owed := baseAmount
			if int64(i) < remainderUnits {
				owed = owed.Add(oneUnit)
			}
			owedMap[uid] = owed
		}

	case SplitExact:
		if len(exactBorrowers) == 0 {
			return nil, ErrNoBorrowers
		}

		totalExactOwed := decimal.Zero
		for _, b := range exactBorrowers {
			uid := strings.TrimSpace(b.UserID)
			if uid == "" {
				return nil, ErrEmptyCreatedBy
			}
			if !b.Amount.IsPositive() {
				return nil, ErrInvalidAmount
			}
			if _, exists := owedMap[uid]; exists {
				return nil, ErrDuplicateUserInEntries
			}
			owedMap[uid] = b.Amount
			totalExactOwed = totalExactOwed.Add(b.Amount)
		}

		if !totalExactOwed.Equal(totalPaid) {
			return nil, ErrUnbalancedTransaction
		}

	case SplitShares:
		if len(sharesBorrowers) == 0 {
			return nil, ErrNoBorrowers
		}

		var totalShares int64
		for _, b := range sharesBorrowers {
			uid := strings.TrimSpace(b.UserID)
			if uid == "" {
				return nil, ErrEmptyCreatedBy
			}
			if b.Shares <= 0 {
				return nil, ErrInvalidSplit
			}
			if _, exists := owedMap[uid]; exists {
				return nil, ErrDuplicateUserInEntries
			}
			owedMap[uid] = decimal.Zero
			totalShares += int64(b.Shares)
		}

		if totalShares <= 0 {
			return nil, ErrInvalidSplit
		}

		// Sort borrowers by UserID deterministically
		sortedBorrowers := make([]Borrower, len(sharesBorrowers))
		copy(sortedBorrowers, sharesBorrowers)
		sort.Slice(sortedBorrowers, func(i, j int) bool {
			return sortedBorrowers[i].UserID < sortedBorrowers[j].UserID
		})

		totalUnits := totalPaid.Mul(unitMultiplier).IntPart()
		var allocatedUnits int64
		for _, b := range sortedBorrowers {
			shareUnits := (totalUnits * int64(b.Shares)) / totalShares
			owedMap[b.UserID] = decimal.New(shareUnits, -precision)
			allocatedUnits += shareUnits
		}

		remainderUnits := totalUnits - allocatedUnits
		oneUnit := decimal.New(1, -precision)
		for i := 0; i < int(remainderUnits); i++ {
			idx := i % len(sortedBorrowers)
			uid := sortedBorrowers[idx].UserID
			owedMap[uid] = owedMap[uid].Add(oneUnit)
		}

	default:
		return nil, ErrInvalidSplit
	}

	// Combine all distinct users across payers and borrowers into Entries
	allUsersSet := make(map[string]struct{})
	for u := range paidMap {
		allUsersSet[u] = struct{}{}
	}
	for u := range owedMap {
		allUsersSet[u] = struct{}{}
	}

	allUsers := make([]string, 0, len(allUsersSet))
	for u := range allUsersSet {
		allUsers = append(allUsers, u)
	}
	sort.Strings(allUsers)

	entries := make([]Entry, 0, len(allUsers))
	for _, uid := range allUsers {
		paid := paidMap[uid]
		owed := owedMap[uid]
		if paid.IsZero() && owed.IsZero() {
			continue
		}
		entries = append(entries, Entry{
			UserID:     uid,
			PaidAmount: paid,
			OwedAmount: owed,
		})
	}

	return entries, nil
}
