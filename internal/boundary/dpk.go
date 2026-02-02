package boundary

import (
	"math"
	"slices"
)

// DPKSegment finds the optimal partition of n blocks into k segments
// that minimizes total kernel cost. Returns k-1 boundary gap indices (sorted).
//
// Time: O(k * n^2), Space: O(k * n).
// prohibited maps gap indices that must not be used as boundaries.
func DPKSegment(gram *GramMatrix, k, minGap int, prohibited map[int]bool) []int {
	n := gram.n
	if n == 0 || k <= 1 {
		return nil
	}
	if k > n {
		k = n
	}
	if minGap < 1 {
		minGap = 1
	}

	inf := math.Inf(1)

	// dp[j][t] = min cost of partitioning [0, t) into j segments
	// split[j][t] = the split point s that achieved dp[j][t]
	dp := make([][]float64, k+1)
	split := make([][]int, k+1)
	for j := 0; j <= k; j++ {
		dp[j] = make([]float64, n+1)
		split[j] = make([]int, n+1)
		for t := 0; t <= n; t++ {
			dp[j][t] = inf
			split[j][t] = -1
		}
	}

	// Base case: 1 segment covering [0, t)
	for t := 1; t <= n; t++ {
		dp[1][t] = gram.SegmentCost(0, t)
	}

	// Fill DP table
	for j := 2; j <= k; j++ {
		for t := j; t <= n; t++ { // need at least j blocks for j segments
			for s := j - 1; s < t; s++ { // s = start of last segment
				// Boundary at gap s-1
				gapIdx := s - 1
				if isProhibited(gapIdx, prohibited) {
					continue
				}
				// Check minGap with previous boundary (encoded in split chain)
				if !checkMinGap(split, j-1, s, minGap, prohibited) {
					continue
				}

				cost := dp[j-1][s] + gram.SegmentCost(s, t)
				if cost < dp[j][t] {
					dp[j][t] = cost
					split[j][t] = s
				}
			}
		}
	}

	// If no valid k-segment partition found, try fewer segments
	if dp[k][n] == inf {
		for tryK := k - 1; tryK >= 1; tryK-- {
			if dp[tryK][n] < inf {
				return backtrackDP(split, tryK, n)
			}
		}
		return nil
	}

	return backtrackDP(split, k, n)
}

// checkMinGap verifies that placing a boundary at gap gapIdx (= s-1) doesn't
// violate minGap with the previous boundary in the split chain.
func checkMinGap(splitTable [][]int, prevJ, s, minGap int, prohibited map[int]bool) bool {
	if minGap <= 1 || prevJ <= 1 {
		return true
	}
	// The previous boundary is at gap (split[prevJ][s] - 1)
	prevS := splitTable[prevJ][s]
	if prevS <= 0 {
		return true
	}
	prevGap := prevS - 1
	thisGap := s - 1
	return thisGap-prevGap >= minGap
}

// backtrackDP reconstructs the boundary indices from the DP split table.
func backtrackDP(split [][]int, k, n int) []int {
	var boundaries []int
	t := n
	for j := k; j >= 2; j-- {
		s := split[j][t]
		if s <= 0 {
			break
		}
		boundaries = append(boundaries, s-1) // gap index
		t = s
	}
	slices.Sort(boundaries)
	return boundaries
}
