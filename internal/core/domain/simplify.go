package domain

import (
	"math/bits"
	"sort"

	"github.com/shopspring/decimal"
)

type participantBalance struct {
	UserID     string
	NetBalance decimal.Decimal // Positive = creditor (is owed), Negative = debtor (owes)
}

// SimplifyDebtsOptimal calculates the mathematically optimal minimum number of debt transfers (N - K)
// using Bitmask Dynamic Programming over zero-sum subsets with exact decimal arithmetic.
func SimplifyDebtsOptimal(netBalances map[string]decimal.Decimal, currency string) ([]DebtTransfer, error) {
	// 1. Filter active participants with non-zero net balance
	var participants []participantBalance
	totalNet := decimal.Zero

	for uid, net := range netBalances {
		if !net.IsZero() {
			participants = append(participants, participantBalance{UserID: uid, NetBalance: net})
			totalNet = totalNet.Add(net)
		}
	}

	if len(participants) == 0 {
		return []DebtTransfer{}, nil
	}

	if !totalNet.IsZero() {
		return nil, ErrUnbalancedTransaction
	}

	// Sort participants by UserID for deterministic output
	sort.Slice(participants, func(i, j int) bool {
		return participants[i].UserID < participants[j].UserID
	})

	n := len(participants)

	// Fallback to greedy if group size > 20 to avoid exponential memory
	if n > 20 {
		return simplifyDebtsGreedy(participants, currency), nil
	}

	totalMasks := 1 << n
	maskSum := make([]decimal.Decimal, totalMasks)
	for mask := 1; mask < totalMasks; mask++ {
		lowBit := bits.TrailingZeros(uint(mask))
		maskSum[mask] = maskSum[mask^(1<<lowBit)].Add(participants[lowBit].NetBalance)
	}

	// dp[mask] = maximum number of disjoint zero-sum subsets within mask
	dp := make([]int, totalMasks)
	parent := make([]int, totalMasks) // stores the chosen submask for reconstruction

	for mask := 1; mask < totalMasks; mask++ {
		// Default: skip the lowest bit element
		lowBit := bits.TrailingZeros(uint(mask))
		dp[mask] = dp[mask^(1<<lowBit)]
		parent[mask] = 0

		// If this mask itself sums to 0, it can form at least 1 zero-sum group
		if maskSum[mask].IsZero() {
			dp[mask] = 1
			parent[mask] = mask

			// Check all submasks containing lowBit to find maximum partitioning
			sub := mask
			for sub > 0 {
				if (sub&(1<<lowBit)) != 0 && maskSum[sub].IsZero() {
					prev := mask ^ sub
					if dp[prev]+1 > dp[mask] {
						dp[mask] = dp[prev] + 1
						parent[mask] = sub
					}
				}
				sub = (sub - 1) & mask
			}
		}
	}

	// 2. Reconstruct the disjoint zero-sum partitions
	var partitions [][]participantBalance
	currMask := totalMasks - 1

	for currMask > 0 {
		sub := parent[currMask]
		if sub == 0 || !maskSum[sub].IsZero() {
			// If no zero-sum submask was found (should not happen if total sum is 0), collect remaining
			sub = currMask
		}

		var group []participantBalance
		for i := 0; i < n; i++ {
			if (sub & (1 << i)) != 0 {
				group = append(group, participants[i])
			}
		}
		if len(group) > 0 {
			partitions = append(partitions, group)
		}

		currMask ^= sub
	}

	// 3. For each zero-sum partition, solve using greedy matching (guaranteeing size - 1 transfers)
	var allTransfers []DebtTransfer
	for _, partition := range partitions {
		transfers := simplifyDebtsGreedy(partition, currency)
		allTransfers = append(allTransfers, transfers...)
	}

	// Sort transfers deterministically
	sort.Slice(allTransfers, func(i, j int) bool {
		if allTransfers[i].FromUser != allTransfers[j].FromUser {
			return allTransfers[i].FromUser < allTransfers[j].FromUser
		}
		if allTransfers[i].ToUser != allTransfers[j].ToUser {
			return allTransfers[i].ToUser < allTransfers[j].ToUser
		}
		return allTransfers[i].Amount.LessThan(allTransfers[j].Amount)
	})

	return allTransfers, nil
}

// simplifyDebtsGreedy solves debt settlements within a single zero-sum partition
func simplifyDebtsGreedy(participants []participantBalance, currency string) []DebtTransfer {
	var debtors []participantBalance   // NetBalance < 0 (owes money)
	var creditors []participantBalance // NetBalance > 0 (is owed money)

	for _, p := range participants {
		if p.NetBalance.IsNegative() {
			debtors = append(debtors, participantBalance{UserID: p.UserID, NetBalance: p.NetBalance.Abs()})
		} else if p.NetBalance.IsPositive() {
			creditors = append(creditors, participantBalance{UserID: p.UserID, NetBalance: p.NetBalance})
		}
	}

	var transfers []DebtTransfer
	i, j := 0, 0

	for i < len(debtors) && j < len(creditors) {
		debtor := &debtors[i]
		creditor := &creditors[j]

		settleAmount := decimal.Min(debtor.NetBalance, creditor.NetBalance)

		if settleAmount.IsPositive() {
			transfers = append(transfers, DebtTransfer{
				FromUser: debtor.UserID,
				ToUser:   creditor.UserID,
				Amount:   settleAmount,
				Currency: currency,
			})
		}

		debtor.NetBalance = debtor.NetBalance.Sub(settleAmount)
		creditor.NetBalance = creditor.NetBalance.Sub(settleAmount)

		if debtor.NetBalance.IsZero() {
			i++
		}
		if creditor.NetBalance.IsZero() {
			j++
		}
	}

	return transfers
}
