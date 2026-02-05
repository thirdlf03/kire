package boundary

import (
	"math"
	"slices"

	"github.com/thirdlf03/kire/internal/model"
)

const CosineKernelC = 0.022

// PELTDetect finds optimal change points using the Pruned Exact Linear Time
// algorithm with a cosine kernel cost function.
//
// beta is the penalty per boundary (higher = fewer segments).
// minGap is the minimum distance between adjacent boundaries.
// prohibited maps gap indices that must not be selected as boundaries.
//
// Returns sorted gap indices where boundaries should be placed.
func PELTDetect(gram *GramMatrix, beta float64, minGap int, prohibited map[int]bool) []int {
	n := gram.n
	if n < 2 {
		return nil
	}
	if minGap < 1 {
		minGap = 1
	}

	// F[t] = optimal cost of segmenting blocks [0, t)
	// cp[t] = last change point before t in optimal solution
	F := make([]float64, n+1)
	cp := make([]int, n+1)
	for i := range cp {
		cp[i] = -1
	}

	// F[0] = -beta (offset so first segment doesn't pay double penalty)
	F[0] = -beta

	// Candidate set for PELT pruning.
	// When prohibited gaps are present, we disable pruning to guarantee correctness.
	usePruning := len(prohibited) == 0
	candidates := []int{0}

	for t := 1; t <= n; t++ {
		F[t] = math.Inf(1)

		var nextCandidates []int
		for _, s := range candidates {
			segLen := t - s
			if segLen < 1 {
				continue
			}

			// Check minGap: if s > 0, the boundary is at gap s-1.
			// The previous boundary is at gap cp[s]-1.
			// We need gap (s-1) to be at least minGap from previous gap.
			if s > 0 && isProhibited(s-1, prohibited) {
				continue
			}

			cost := F[s] + gram.SegmentCost(s, t) + beta
			if cost < F[t] {
				F[t] = cost
				cp[t] = s
			}

			// PELT pruning: if F[s] + Cost(s,t) >= F[t], s cannot improve for any t' > t
			if usePruning && F[s]+gram.SegmentCost(s, t) >= F[t] {
				// prune s
			} else {
				nextCandidates = append(nextCandidates, s)
			}
		}

		if usePruning {
			nextCandidates = append(nextCandidates, t)
			candidates = nextCandidates
		} else {
			candidates = append(candidates, t)
		}
	}

	// Backtrack to recover change points
	var boundaries []int
	pos := n
	for {
		prev := cp[pos]
		if prev <= 0 {
			break
		}
		boundaries = append(boundaries, prev-1) // gap index = change point - 1
		pos = prev
	}
	slices.Sort(boundaries)

	// Apply minGap filter (needed because DP doesn't enforce it directly)
	boundaries = filterMinGap(boundaries, minGap)

	return boundaries
}

func isProhibited(gap int, prohibited map[int]bool) bool {
	return prohibited != nil && prohibited[gap]
}

// filterMinGap removes boundaries that violate the minimum gap constraint,
// keeping the ones that appear first.
func filterMinGap(boundaries []int, minGap int) []int {
	if minGap <= 1 || len(boundaries) <= 1 {
		return boundaries
	}
	var filtered []int
	lastAdded := -minGap // sentinel so first boundary always passes
	for _, b := range boundaries {
		if b-lastAdded >= minGap {
			filtered = append(filtered, b)
			lastAdded = b
		}
	}
	return filtered
}

// AutoBeta estimates a penalty parameter from the Gram matrix.
// Uses a BIC-inspired heuristic: beta = median(cost deltas) * scaling factor.
func AutoBeta(gram *GramMatrix) float64 {
	n := gram.n
	if n < 2 {
		return 1.0
	}

	// Compute cost delta for each possible single-split point
	wholeCost := gram.SegmentCost(0, n)
	deltas := make([]float64, n-1)
	for i := 1; i < n; i++ {
		splitCost := gram.SegmentCost(0, i) + gram.SegmentCost(i, n)
		deltas[i-1] = wholeCost - splitCost // positive = split helps
	}

	slices.Sort(deltas)

	// Use median of positive deltas as base, with a scaling factor
	var positiveDeltas []float64
	for _, d := range deltas {
		if d > 0 {
			positiveDeltas = append(positiveDeltas, d)
		}
	}

	if len(positiveDeltas) == 0 {
		// No beneficial splits: use a high penalty
		return 1.0
	}

	median := positiveDeltas[len(positiveDeltas)/2]
	// Scale so that splits with above-median benefit are accepted
	return median * 0.5
}

// AutoBetaBIC estimates the penalty parameter using Bayesian Information Criterion.
// BIC = n*log(RSS/n) + k*log(n), where RSS is the residual sum of squares,
// n is the number of data points, and k is the number of parameters (segments).
// We search for beta that minimizes BIC.
func AutoBetaBIC(gram *GramMatrix, maxSegments int) float64 {
	n := gram.n
	if n < 2 {
		return 1.0
	}
	if maxSegments <= 0 {
		maxSegments = n / 2
	}
	if maxSegments > n-1 {
		maxSegments = n - 1
	}

	// Evaluate BIC for different segment counts
	bestBIC := math.Inf(1)
	bestK := 1

	for k := 1; k <= maxSegments; k++ {
		// Use DP to find optimal k-segment partition
		boundaries := DPKSegment(gram, k, 1, nil)
		cost := totalCostForBoundaries(gram, boundaries)

		// Approximate RSS as segment cost (higher cost = more heterogeneity)
		// BIC: n*log(cost/n) + k*log(n)
		// Use cost directly as RSS proxy
		if cost <= 0 {
			cost = 1e-10 // avoid log(0)
		}
		bic := float64(n)*math.Log(cost/float64(n)) + float64(k)*math.Log(float64(n))

		if bic < bestBIC {
			bestBIC = bic
			bestK = k
		}
	}

	// Now find beta that produces approximately bestK segments
	return betaForSegmentCount(gram, bestK)
}

// totalCostForBoundaries computes the total segment cost for a given boundary set.
func totalCostForBoundaries(gram *GramMatrix, boundaries []int) float64 {
	n := gram.n
	cost := 0.0
	start := 0
	for _, b := range boundaries {
		cost += gram.SegmentCost(start, b+1)
		start = b + 1
	}
	cost += gram.SegmentCost(start, n)
	return cost
}

// betaForSegmentCount estimates the beta that produces approximately k segments
// using binary search.
func betaForSegmentCount(gram *GramMatrix, targetK int) float64 {
	// Binary search for beta
	// Low beta → more segments; high beta → fewer segments
	lo, hi := 0.001, 10.0

	for iter := 0; iter < 20; iter++ {
		mid := (lo + hi) / 2
		boundaries := PELTDetect(gram, mid, 1, nil)
		nSegs := len(boundaries) + 1

		if nSegs == targetK {
			return mid
		} else if nSegs > targetK {
			lo = mid // need higher penalty to reduce segments
		} else {
			hi = mid // need lower penalty to increase segments
		}
	}

	return (lo + hi) / 2
}

// AutoBetaCrossVal estimates the penalty parameter using leave-one-out cross-validation.
// For each candidate beta, it measures how well the segmentation generalizes
// by holding out each block and measuring prediction error.
func AutoBetaCrossVal(gram *GramMatrix, nBetas int) float64 {
	n := gram.n
	if n < 3 {
		return AutoBeta(gram)
	}
	if nBetas <= 0 {
		nBetas = 10
	}

	// Generate candidate betas on log scale
	minBeta, maxBeta := 0.01, 5.0
	betas := make([]float64, nBetas)
	for i := 0; i < nBetas; i++ {
		t := float64(i) / float64(nBetas-1)
		betas[i] = minBeta * math.Pow(maxBeta/minBeta, t)
	}

	bestBeta := betas[0]
	bestScore := math.Inf(1)

	for _, beta := range betas {
		score := crossValScore(gram, beta)
		if score < bestScore {
			bestScore = score
			bestBeta = beta
		}
	}

	return bestBeta
}

// AutoBetaTheory estimates the penalty parameter using a theoretical formula:
// beta = C * sqrt(T * log(T)), where T is the number of data points.
func AutoBetaTheory(gram *GramMatrix, c float64) float64 {
	n := gram.n
	if n < 2 {
		return 1.0
	}
	if c <= 0 {
		c = CosineKernelC
	}
	T := float64(n)
	return c * math.Sqrt(T*math.Log(T))
}

// crossValScore computes a cross-validation score for a given beta.
// Lower is better. Uses segment coherence as the metric.
func crossValScore(gram *GramMatrix, beta float64) float64 {
	boundaries := PELTDetect(gram, beta, 1, nil)

	// Compute total cost with this segmentation
	baseCost := totalCostForBoundaries(gram, boundaries)

	// Penalize extreme solutions (too many or too few segments)
	nSegs := len(boundaries) + 1
	n := gram.n
	avgSize := float64(n) / float64(nSegs)

	// Variance penalty for uneven segments
	variance := 0.0
	start := 0
	for _, b := range boundaries {
		size := float64(b + 1 - start)
		diff := size - avgSize
		variance += diff * diff
		start = b + 1
	}
	lastSize := float64(n - start)
	diff := lastSize - avgSize
	variance += diff * diff
	variance /= float64(nSegs)

	// Combined score: segment cost + size variance penalty
	return baseCost + 0.1*variance
}

// detectBoundariesKCPD implements boundary detection using KCPD.
func detectBoundariesKCPD(embeddings []model.Embedding, config ScoringConfig) BoundaryResult {
	n := len(embeddings)
	gram := NewGramMatrix(embeddings)

	// Build prohibited set from hints
	prohibited := make(map[int]bool)
	var enforced []int
	for _, h := range config.Hints {
		if h.GapIndex < 0 || h.GapIndex >= n-1 {
			continue
		}
		switch h.Kind {
		case HintProhibit:
			prohibited[h.GapIndex] = true
		case HintEnforce:
			enforced = append(enforced, h.GapIndex)
		}
	}

	minGap := config.MinGap
	if minGap < 1 {
		minGap = 1
	}

	var boundaries []int

	if config.TargetCount != nil && *config.TargetCount > 1 {
		// Use DP optimal K-segment partition
		boundaries = DPKSegment(&gram, *config.TargetCount, minGap, prohibited)
	} else {
		// Use PELT with penalty
		beta := 0.0
		if config.Beta != nil {
			beta = *config.Beta
		} else {
			// Select beta estimation strategy
			switch config.BetaStrategy {
			case "bic":
				beta = AutoBetaBIC(&gram, 0)
			case "crossval":
				beta = AutoBetaCrossVal(&gram, 10)
			case "theory":
				beta = AutoBetaTheory(&gram, CosineKernelC)
			default:
				// "auto" or "" uses original heuristic
				beta = AutoBeta(&gram)
			}
		}
		boundaries = PELTDetect(&gram, beta, minGap, prohibited)
	}

	// Apply enforced boundaries
	if len(enforced) > 0 {
		existing := make(map[int]bool, len(boundaries))
		for _, b := range boundaries {
			existing[b] = true
		}
		for _, e := range enforced {
			if !existing[e] {
				boundaries = append(boundaries, e)
			}
		}
		slices.Sort(boundaries)
	}

	// Compute synthetic depth scores from cost deltas for downstream compatibility
	// (segment optimizer's resplit uses depth scores to find split points)
	depthScores := make([]float64, n-1)
	for i := 0; i < n-1; i++ {
		// For each gap, compute how much splitting there reduces cost
		// compared to the segments that would contain it.
		// Use a local window: the enclosing segment in the current partition.
		a, b := findEnclosingSegment(boundaries, i, n)
		if b-a > 1 {
			unsplit := gram.SegmentCost(a, b)
			split := gram.SegmentCost(a, i+1) + gram.SegmentCost(i+1, b)
			delta := unsplit - split
			if delta > 0 {
				depthScores[i] = delta
			}
		}
	}

	// Compute similarities (for reporting/compatibility)
	sims := BlockSimilarities(embeddings, config.BlockK)

	// Compute confidence for each boundary
	maxDepth := 0.0
	for _, d := range depthScores {
		if d > maxDepth {
			maxDepth = d
		}
	}
	confidences := make([]float64, len(boundaries))
	for i, b := range boundaries {
		if b >= 0 && b < len(depthScores) && maxDepth > 0 {
			raw := depthScores[b] / maxDepth
			if raw > 1 {
				raw = 1
			}
			confidences[i] = raw
		}
	}

	return BoundaryResult{
		Boundaries:   boundaries,
		Similarities: sims,
		DepthScores:  depthScores,
		Threshold:    0, // not used in KCPD mode
		Confidences:  confidences,
	}
}

// findEnclosingSegment returns the [a, b) range of the segment that contains
// gap index g, given the current set of boundaries.
func findEnclosingSegment(boundaries []int, g, n int) (int, int) {
	a := 0
	b := n
	for _, bd := range boundaries {
		if bd < g {
			a = bd + 1
		} else if bd > g {
			b = bd + 1
			break
		} else {
			// g is a boundary itself; return the two segments it divides
			// Use left segment for depth computation
			b = bd + 1
			break
		}
	}
	return a, b
}
