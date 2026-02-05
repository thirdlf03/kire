package boundary

import (
	"cmp"
	"math"
	"slices"

	"github.com/thirdlf03/kire/internal/model"
	"github.com/thirdlf03/kire/internal/vecmath"
)

// HintKind represents a boundary hint type.
type HintKind int

const (
	HintProhibit HintKind = iota // Prohibit a boundary at this gap
	HintEnforce                  // Enforce a boundary at this gap
	HintPrefer                   // Prefer a boundary (boost depth score)
)

const (
	hintPreferBoost      = 0.1  // Depth score boost for HintPrefer
	variancePenaltyCoeff = 0.01 // Coefficient for size variance penalty in EvaluateSegmentation
	hybridKCPDWeight     = 0.5  // Weight of KCPD depth scores in hybrid mode
	blockKAutoThreshold  = 50   // Block count threshold for auto block-k
)

// BoundaryHint specifies a prohibition or enforcement at a specific gap index.
type BoundaryHint struct {
	GapIndex int
	Kind     HintKind
}

// ScoringConfig configures boundary detection.
type ScoringConfig struct {
	Window       int            // Moving average window size for smoothing
	Threshold    *float64       // Depth score cutoff; nil = auto (mean - std/2)
	MinGap       int            // Minimum gap between boundaries
	Hints        []BoundaryHint // Optional boundary hints (nil = no hints)
	TargetCount  *int           // Desired number of segments; nil = auto (threshold-based)
	BlockK       int            // Block comparison window: k embeddings per side (0 or 1 = adjacent)
	Method       string         // Boundary detection method: "" or "texttiling" (default), "kcpd"
	Beta         *float64       // KCPD penalty parameter; nil = auto
	BetaStrategy string         // Beta estimation strategy: "auto" (default), "bic", "crossval", "theory"
}

// BoundaryResult holds the detected boundaries and scoring details.
type BoundaryResult struct {
	Boundaries   []int     // Block indices where boundaries occur (gap between index and index+1)
	Similarities []float64 // Raw cosine similarities between adjacent blocks
	DepthScores  []float64 // Depth scores for each gap
	Threshold    float64   // Depth score cutoff used for boundary selection
	Confidences  []float64 // Confidence score (0.0-1.0) for each boundary
}

// AutoBlockK returns an adaptive BlockK value based on document length.
// For short documents (< blockKAutoThreshold), returns 1 to preserve local signals.
// For longer documents, returns the provided default value.
func AutoBlockK(numBlocks, defaultK int) int {
	if numBlocks < blockKAutoThreshold {
		return 1
	}
	return defaultK
}

// DetectBoundaries identifies semantic boundaries between blocks.
	// When config.Method is "kcpd", uses Kernel Change-Point Detection with PELT.
	// When config.Method is "hybrid", blends TextTiling depth scores with KCPD signals.
	// Otherwise uses the default TextTiling approach.
	func DetectBoundaries(embeddings []model.Embedding, config ScoringConfig) BoundaryResult {
		n := len(embeddings)
		if n < 2 {
			return BoundaryResult{}
		}

		if config.Method == "kcpd" {
			return detectBoundariesKCPD(embeddings, config)
		}
		if config.Method == "hybrid" {
			return detectBoundariesHybrid(embeddings, config)
		}

	// Step 1: Compute cosine similarities (block window or adjacent)
	sims := BlockSimilarities(embeddings, config.BlockK)

	// Step 2: Smooth similarities
	smoothed := Smooth(sims, config.Window)

	// Step 3: Compute depth scores
	depths := DepthScore(smoothed)

	// Step 4: Apply hints — prohibit
	if len(config.Hints) > 0 {
		for _, h := range config.Hints {
			if h.Kind == HintProhibit && h.GapIndex >= 0 && h.GapIndex < len(depths) {
				depths[h.GapIndex] = 0
			}
		}
	}

	// Step 4b: Apply hints — prefer (boost depth scores)
	if len(config.Hints) > 0 {
		for _, h := range config.Hints {
			if h.Kind == HintPrefer && h.GapIndex >= 0 && h.GapIndex < len(depths) {
				depths[h.GapIndex] += hintPreferBoost
			}
		}
	}

	// Step 5: Select boundaries
	var boundaries []int
	var threshold float64
	if config.TargetCount != nil && *config.TargetCount > 1 {
		// Target count mode: try N-2, N-1, N boundary counts and pick the best
		k := *config.TargetCount - 1 // number of boundaries = segments - 1
		minGap := config.MinGap
		if minGap <= 0 {
			minGap = 1
		}
		bestScore := math.Inf(-1)
		for _, candidate := range []int{k - 1, k, k + 1} {
			if candidate <= 0 {
				continue
			}
			b := SelectBoundariesByCount(depths, candidate, minGap)
			score := EvaluateSegmentation(b, n, depths)
			if score > bestScore {
				bestScore = score
				boundaries = b
			}
		}
		// Informational: compute the auto cutoff for report purposes
		threshold = computeCutoff(depths, config.Threshold)
	} else {
		threshold = computeCutoff(depths, config.Threshold)
		boundaries = selectBoundaries(depths, threshold, config.MinGap)
	}

	// Step 6: Apply hints — enforce
	if len(config.Hints) > 0 {
		enforced := make(map[int]bool)
		for _, h := range config.Hints {
			if h.Kind == HintEnforce && h.GapIndex >= 0 && h.GapIndex < n-1 {
				enforced[h.GapIndex] = true
			}
		}
		if len(enforced) > 0 {
			existing := make(map[int]bool, len(boundaries))
			for _, b := range boundaries {
				existing[b] = true
			}
			for idx := range enforced {
				if !existing[idx] {
					boundaries = append(boundaries, idx)
				}
			}
			slices.Sort(boundaries)
		}
	}

	// Compute confidence for each boundary
	maxDepth := 0.0
	for _, d := range depths {
		if d > maxDepth {
			maxDepth = d
		}
	}
	confidences := make([]float64, len(boundaries))
	for i, b := range boundaries {
		if b >= 0 && b < len(depths) && maxDepth > 0 {
			raw := (depths[b] - threshold) / maxDepth
			if raw < 0 {
				raw = 0
			}
			if raw > 1 {
				raw = 1
			}
			confidences[i] = raw
		}
	}

	return BoundaryResult{
		Boundaries:   boundaries,
		Similarities: sims,
		DepthScores:  depths,
		Threshold:    threshold,
		Confidences:  confidences,
	}
}

// detectBoundariesHybrid blends TextTiling depth scores with KCPD depth scores.
func detectBoundariesHybrid(embeddings []model.Embedding, config ScoringConfig) BoundaryResult {
	n := len(embeddings)
	if n < 2 {
		return BoundaryResult{}
	}

	// Step 1: Compute cosine similarities (block window or adjacent)
	sims := BlockSimilarities(embeddings, config.BlockK)

	// Step 2: Smooth similarities
	smoothed := Smooth(sims, config.Window)

	// Step 3: Compute base depth scores (TextTiling)
	depths := DepthScore(smoothed)

	// Step 3b: Blend KCPD depth scores into the base depths
	kcpd := detectBoundariesKCPD(embeddings, config)
	depths = blendDepthScores(depths, kcpd.DepthScores)

	// Step 4: Apply hints — prohibit/prefer
	if len(config.Hints) > 0 {
		for _, h := range config.Hints {
			if h.GapIndex < 0 || h.GapIndex >= len(depths) {
				continue
			}
			switch h.Kind {
			case HintProhibit:
				depths[h.GapIndex] = 0
			case HintPrefer:
				depths[h.GapIndex] += hintPreferBoost
			}
		}
	}

	// Step 5: Select boundaries (same as TextTiling)
	var boundaries []int
	var threshold float64
	if config.TargetCount != nil && *config.TargetCount > 1 {
		k := *config.TargetCount - 1
		minGap := config.MinGap
		if minGap <= 0 {
			minGap = 1
		}
		bestScore := math.Inf(-1)
		for _, candidate := range []int{k - 1, k, k + 1} {
			if candidate <= 0 {
				continue
			}
			b := SelectBoundariesByCount(depths, candidate, minGap)
			score := EvaluateSegmentation(b, n, depths)
			if score > bestScore {
				bestScore = score
				boundaries = b
			}
		}
		threshold = computeCutoff(depths, config.Threshold)
	} else {
		threshold = computeCutoff(depths, config.Threshold)
		boundaries = selectBoundaries(depths, threshold, config.MinGap)
	}

	// Step 6: Apply hints — enforce
	if len(config.Hints) > 0 {
		enforced := make(map[int]bool)
		for _, h := range config.Hints {
			if h.Kind == HintEnforce && h.GapIndex >= 0 && h.GapIndex < n-1 {
				enforced[h.GapIndex] = true
			}
		}
		if len(enforced) > 0 {
			existing := make(map[int]bool, len(boundaries))
			for _, b := range boundaries {
				existing[b] = true
			}
			for idx := range enforced {
				if !existing[idx] {
					boundaries = append(boundaries, idx)
				}
			}
			slices.Sort(boundaries)
		}
	}

	// Compute confidence for each boundary
	maxDepth := maxFloat(depths)
	confidences := make([]float64, len(boundaries))
	for i, b := range boundaries {
		if b >= 0 && b < len(depths) && maxDepth > 0 {
			raw := (depths[b] - threshold) / maxDepth
			if raw < 0 {
				raw = 0
			}
			if raw > 1 {
				raw = 1
			}
			confidences[i] = raw
		}
	}

	return BoundaryResult{
		Boundaries:   boundaries,
		Similarities: sims,
		DepthScores:  depths,
		Threshold:    threshold,
		Confidences:  confidences,
	}
}

func blendDepthScores(base, kcpd []float64) []float64 {
	if len(base) == 0 || len(base) != len(kcpd) {
		return base
	}
	maxBase := maxFloat(base)
	maxKCPD := maxFloat(kcpd)
	if maxBase <= 0 || maxKCPD <= 0 {
		return base
	}
	for i := range base {
		base[i] += (kcpd[i] / maxKCPD) * maxBase * hybridKCPDWeight
	}
	return base
}

func maxFloat(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	max := values[0]
	for _, v := range values[1:] {
		if v > max {
			max = v
		}
	}
	return max
}

// MeanPool returns the element-wise mean of the given vectors.
// Returns nil for empty input. Returns a clone for single input.
func MeanPool(vectors [][]float64) []float64 {
	if len(vectors) == 0 {
		return nil
	}
	dims := len(vectors[0])
	result := make([]float64, dims)
	if len(vectors) == 1 {
		copy(result, vectors[0])
		return result
	}
	for _, v := range vectors {
		for j, val := range v {
			result[j] += val
		}
	}
	inv := 1.0 / float64(len(vectors))
	for j := range result {
		result[j] *= inv
	}
	return result
}

// BlockSimilarities computes cosine similarity between left and right block
// windows of size k for each gap. When k <= 1, falls back to adjacent
// comparison (equivalent to the original TextTiling implementation).
func BlockSimilarities(embeddings []model.Embedding, k int) []float64 {
	n := len(embeddings)
	if n < 2 {
		return nil
	}

	sims := make([]float64, n-1)

	if k <= 1 {
		// Adjacent comparison (original behavior)
		for i := 0; i < n-1; i++ {
			sims[i] = CosineSimilarity(embeddings[i].Vector, embeddings[i+1].Vector)
		}
		return sims
	}

	for i := 0; i < n-1; i++ {
		// Left window: embeddings[max(0, i-k+1) .. i]
		lo := i - k + 1
		if lo < 0 {
			lo = 0
		}
		leftVecs := make([][]float64, 0, i-lo+1)
		for j := lo; j <= i; j++ {
			leftVecs = append(leftVecs, embeddings[j].Vector)
		}

		// Right window: embeddings[i+1 .. min(n-1, i+k)]
		hi := i + k
		if hi > n-1 {
			hi = n - 1
		}
		rightVecs := make([][]float64, 0, hi-i)
		for j := i + 1; j <= hi; j++ {
			rightVecs = append(rightVecs, embeddings[j].Vector)
		}

		sims[i] = CosineSimilarity(MeanPool(leftVecs), MeanPool(rightVecs))
	}
	return sims
}

// CosineSimilarity computes the cosine similarity between two vectors.
// Delegates to vecmath.CosineSimilarity.
func CosineSimilarity(a, b []float64) float64 {
	return vecmath.CosineSimilarity(a, b)
}

// Smooth applies a moving average to the similarity scores.
func Smooth(vals []float64, window int) []float64 {
	if window <= 0 || len(vals) == 0 {
		return slices.Clone(vals)
	}
	result := make([]float64, len(vals))
	for i := range vals {
		lo := i - window
		if lo < 0 {
			lo = 0
		}
		hi := i + window
		if hi >= len(vals) {
			hi = len(vals) - 1
		}
		sum := 0.0
		count := 0
		for j := lo; j <= hi; j++ {
			sum += vals[j]
			count++
		}
		result[i] = sum / float64(count)
	}
	return result
}

// DepthScore computes the depth score for each gap.
// For each position i, depth = (leftPeak - val[i]) + (rightPeak - val[i])
// where leftPeak is the max value to the left and rightPeak is the max to the right.
func DepthScore(sims []float64) []float64 {
	n := len(sims)
	depths := make([]float64, n)
	for i := 0; i < n; i++ {
		leftPeak := sims[i]
		for j := i - 1; j >= 0; j-- {
			if sims[j] > leftPeak {
				leftPeak = sims[j]
			}
			if sims[j] < sims[j+1] {
				break // descending into another valley
			}
		}
		rightPeak := sims[i]
		for j := i + 1; j < n; j++ {
			if sims[j] > rightPeak {
				rightPeak = sims[j]
			}
			if sims[j] < sims[j-1] {
				break
			}
		}
		depths[i] = (leftPeak - sims[i]) + (rightPeak - sims[i])
	}
	return depths
}

func computeCutoff(depths []float64, threshold *float64) float64 {
	if threshold != nil {
		return *threshold
	}
	if len(depths) == 0 {
		return 0
	}
	mean, std := meanStd(depths)
	return mean - std/2
}

func meanStd(vals []float64) (float64, float64) {
	if len(vals) == 0 {
		return 0, 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	mean := sum / float64(len(vals))
	variance := 0.0
	for _, v := range vals {
		diff := v - mean
		variance += diff * diff
	}
	variance /= float64(len(vals))
	return mean, math.Sqrt(variance)
}

type candidate struct {
	index int
	depth float64
}

// greedySelect picks up to maxN candidates (0 = unlimited) respecting minGap,
// sorted by depth descending. Returns indices sorted ascending.
func greedySelect(candidates []candidate, minGap, maxN int) []int {
	slices.SortFunc(candidates, func(a, b candidate) int {
		return cmp.Compare(b.depth, a.depth)
	})

	var selected []int
	for _, c := range candidates {
		if maxN > 0 && len(selected) >= maxN {
			break
		}
		tooClose := false
		for _, s := range selected {
			if abs(c.index-s) < minGap {
				tooClose = true
				break
			}
		}
		if !tooClose {
			selected = append(selected, c.index)
		}
	}

	slices.Sort(selected)
	return selected
}

func selectBoundaries(depths []float64, cutoff float64, minGap int) []int {
	if minGap <= 0 {
		minGap = 1
	}
	var candidates []candidate
	for i, d := range depths {
		if d > cutoff {
			candidates = append(candidates, candidate{index: i, depth: d})
		}
	}
	return greedySelect(candidates, minGap, 0)
}

// SelectBoundariesByCount greedily selects k boundaries from depths,
// choosing the highest depth score gaps while respecting minGap.
func SelectBoundariesByCount(depths []float64, k, minGap int) []int {
	if k <= 0 || len(depths) == 0 {
		return nil
	}
	if minGap <= 0 {
		minGap = 1
	}
	var candidates []candidate
	for i, d := range depths {
		if d > 0 {
			candidates = append(candidates, candidate{index: i, depth: d})
		}
	}
	return greedySelect(candidates, minGap, k)
}

// EvaluateSegmentation scores a set of boundaries.
// Higher is better: sum of depth scores at boundaries minus a size variance penalty.
func EvaluateSegmentation(boundaries []int, numBlocks int, depths []float64) float64 {
	if numBlocks <= 0 {
		return 0
	}

	// Sum depth scores at chosen boundaries
	depthSum := 0.0
	for _, b := range boundaries {
		if b >= 0 && b < len(depths) {
			depthSum += depths[b]
		}
	}

	// Compute segment sizes
	nSegs := len(boundaries) + 1
	sizes := make([]float64, nSegs)
	prev := 0
	for i, b := range boundaries {
		sizes[i] = float64(b + 1 - prev)
		prev = b + 1
	}
	sizes[nSegs-1] = float64(numBlocks - prev)

	// Size variance penalty
	meanSize := float64(numBlocks) / float64(nSegs)
	variance := 0.0
	for _, s := range sizes {
		diff := s - meanSize
		variance += diff * diff
	}
	variance /= float64(nSegs)

	return depthSum - variancePenaltyCoeff*variance
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
