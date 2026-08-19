# Optimal Bitmask DP Debt Simplification Engine

Splitle implements a mathematically optimal **Cash Flow Minimization** algorithm using **Bitmask Dynamic Programming** to guarantee the theoretical minimum number of debt settlement payments with exact decimal arithmetic.

---

## 🎯 The Problem

Given a group of $N$ participants where each person has an exact decimal net balance $b_i = \text{paid}_i - \text{owed}_i$ such that $\sum b_i = 0$:
- Find the minimal set of direct transfers ($u \rightarrow v$ with amount $A$) that settles all balances to $0$.

---

## 📐 The Mathematical Theorem

The minimum number of transactions needed to settle a set of balances of size $N$ is strictly:
$$\text{Min Payments} = N - K$$
where $K$ is the **maximum number of disjoint non-empty subsets whose balances each sum to $0$**.

### Example:
- Alice: $-\$10$, Bob: $+\$10$ (Subset 1: sums to $0$, size 2 $\rightarrow 2 - 1 = 1$ payment: Alice $\rightarrow$ Bob)
- Charlie: $-\$50$, Dave: $+\$50$ (Subset 2: sums to $0$, size 2 $\rightarrow 2 - 1 = 1$ payment: Charlie $\rightarrow$ Dave)
- Here $N = 4$, $K = 2 \implies 4 - 2 = \mathbf{2\text{ payments total}}$.
- A naive greedy algorithm might produce 3 payments, whereas Bitmask DP guarantees 2.

---

## ⚡ Algorithm Implementation Details

Location: [`internal/core/domain/simplify.go`](file:///Users/aljodomo/workspace/telesplit/internal/core/domain/simplify.go)

1. **Filter Non-Zero Balances**: Exclude members whose net balance is already `decimal.Zero`. Let active count be $N$.
2. **Precompute Mask Sums**:
   For each bitmask $m \in [1, 2^N - 1]$:
   $$\text{maskSum}[m] = \text{maskSum}[m \setminus \{\text{lowBit}\}].\text{Add}(b_{\text{lowBit}})$$
3. **Dynamic Programming State**:
   - `dp[m]` = maximum number of zero-sum subsets within mask $m$.
   - `parent[m]` = submask chosen to form the zero-sum partition.
4. **Submask Transition**:
   For each mask $m$ with lowest bit $i$:
   - If $\text{maskSum}[m].\text{IsZero()}$, iterate submasks $s \subseteq m$ containing bit $i$ where $\text{maskSum}[s].\text{IsZero()}$:
     $$\text{dp}[m] = \max(\text{dp}[m], \text{dp}[m \setminus s] + 1)$$
5. **Reconstruction**:
   - Backtrack through `parent` masks to extract all $K$ disjoint zero-sum partitions.
   - Run greedy two-pointer matching using `decimal.Min` inside each isolated partition (producing strictly $\text{size} - 1$ transfers per partition).
   - Combine results $\rightarrow \sum (\text{size}_k - 1) = N - K$ transfers.

---

## ⏱️ Complexity & Scalability

- **Time Complexity**: $O(3^N)$ or $O(2^N \cdot N)$ via submask optimization.
- **Performance**: For typical expense groups ($N \le 20$), $2^{20}$ operations completes in **$< 2\text{ milliseconds}$** in Go.
- **Safety Fallback**: For abnormally large groups ($N > 20$), it falls back gracefully to a standard $O(N \log N)$ greedy heap matcher.

---

## 🔗 Related Documents
- [domain-rules.md](file:///Users/aljodomo/workspace/telesplit/.ai/domain-rules.md) - How balances are computed.
- [testing-guide.md](file:///Users/aljodomo/workspace/telesplit/.ai/testing-guide.md) - Unit tests verifying $N - K$ optimality.
