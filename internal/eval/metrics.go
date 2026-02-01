package eval

// BoundaryScores holds precision, recall, and F1 for boundary detection.
type BoundaryScores struct {
	Precision float64 `json:"precision"`
	Recall    float64 `json:"recall"`
	F1        float64 `json:"f1"`
}

// Pk computes the Pk evaluation metric for text segmentation.
// ref and hyp are sorted boundary indices (gap positions between blocks).
// numBlocks is the total number of blocks.
// k is the window half-width; 0 means auto (average reference segment length / 2).
// Returns a value in [0.0, 1.0] where lower is better.
func Pk(ref, hyp []int, numBlocks, k int) float64 {
	if numBlocks < 2 {
		return 0
	}
	if k <= 0 {
		k = autoK(ref, numBlocks)
	}
	if k < 1 {
		k = 1
	}
	if k >= numBlocks {
		k = numBlocks - 1
	}

	refSeg := buildSegmentMap(ref, numBlocks)
	hypSeg := buildSegmentMap(hyp, numBlocks)

	mismatches := 0
	total := numBlocks - k
	for i := 0; i < total; i++ {
		refSame := refSeg[i] == refSeg[i+k]
		hypSame := hypSeg[i] == hypSeg[i+k]
		if refSame != hypSame {
			mismatches++
		}
	}
	return float64(mismatches) / float64(total)
}

// WindowDiff computes the WindowDiff metric, an improvement over Pk.
// It counts windows where the number of boundaries differs between ref and hyp.
// Returns a value in [0.0, 1.0] where lower is better.
func WindowDiff(ref, hyp []int, numBlocks, k int) float64 {
	if numBlocks < 2 {
		return 0
	}
	if k <= 0 {
		k = autoK(ref, numBlocks)
	}
	if k < 1 {
		k = 1
	}
	if k >= numBlocks {
		k = numBlocks - 1
	}

	refSet := toSet(ref)
	hypSet := toSet(hyp)

	penalties := 0
	total := numBlocks - k
	for i := 0; i < total; i++ {
		refCount := countBoundariesInWindow(refSet, i, i+k)
		hypCount := countBoundariesInWindow(hypSet, i, i+k)
		if refCount != hypCount {
			penalties++
		}
	}
	return float64(penalties) / float64(total)
}

// BoundaryPRF computes precision, recall, and F1 for boundary detection.
// tolerance is the allowed distance (in blocks) for a match; 0 means exact match only.
func BoundaryPRF(ref, hyp []int, numBlocks, tolerance int) BoundaryScores {
	if len(ref) == 0 && len(hyp) == 0 {
		return BoundaryScores{Precision: 1, Recall: 1, F1: 1}
	}
	if len(ref) == 0 {
		return BoundaryScores{}
	}
	if len(hyp) == 0 {
		return BoundaryScores{}
	}

	// Greedy matching: for each hyp boundary, find closest unmatched ref boundary
	matched := make([]bool, len(ref))
	tp := 0
	for _, h := range hyp {
		bestIdx := -1
		bestDist := tolerance + 1
		for i, g := range ref {
			if matched[i] {
				continue
			}
			d := h - g
			if d < 0 {
				d = -d
			}
			if d <= tolerance && d < bestDist {
				bestDist = d
				bestIdx = i
			}
		}
		if bestIdx >= 0 {
			matched[bestIdx] = true
			tp++
		}
	}

	precision := float64(tp) / float64(len(hyp))
	recall := float64(tp) / float64(len(ref))
	f1 := 0.0
	if precision+recall > 0 {
		f1 = 2 * precision * recall / (precision + recall)
	}
	return BoundaryScores{
		Precision: precision,
		Recall:    recall,
		F1:        f1,
	}
}

// autoK computes the default window half-width: average reference segment length / 2.
func autoK(ref []int, numBlocks int) int {
	numSegs := len(ref) + 1
	avgLen := float64(numBlocks) / float64(numSegs)
	k := int(avgLen / 2)
	if k < 1 {
		k = 1
	}
	return k
}

// buildSegmentMap assigns a segment index to each block position.
func buildSegmentMap(boundaries []int, numBlocks int) []int {
	seg := make([]int, numBlocks)
	segIdx := 0
	bIdx := 0
	for i := 0; i < numBlocks; i++ {
		if bIdx < len(boundaries) && i > boundaries[bIdx] {
			segIdx++
			bIdx++
		}
		seg[i] = segIdx
	}
	return seg
}

func toSet(indices []int) map[int]bool {
	s := make(map[int]bool, len(indices))
	for _, i := range indices {
		s[i] = true
	}
	return s
}

// countBoundariesInWindow counts boundaries in the half-open interval [lo, hi).
// A boundary at index b means a split between block b and b+1,
// so it falls in window [lo, hi) if lo <= b < hi.
func countBoundariesInWindow(set map[int]bool, lo, hi int) int {
	count := 0
	for i := lo; i < hi; i++ {
		if set[i] {
			count++
		}
	}
	return count
}
