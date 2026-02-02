package boundary

import (
	"math"
	"slices"

	"github.com/thirdlf03/kire/internal/model"
)

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
			beta = AutoBeta(&gram)
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
