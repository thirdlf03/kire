package boundary

import (
	"math"
	"slices"
	"testing"

	"github.com/thirdlf03/kire/internal/model"
)

func makeEmbeddings(vectors ...[]float64) []model.Embedding {
	out := make([]model.Embedding, len(vectors))
	for i, v := range vectors {
		out[i] = model.Embedding{BlockIndex: i, Vector: v}
	}
	return out
}

func TestPELTDetect_TwoTopics(t *testing.T) {
	topicA := []float64{1, 0, 0}
	topicB := []float64{0, 1, 0}
	embeddings := makeEmbeddings(topicA, topicA, topicA, topicB, topicB, topicB)
	gram := NewGramMatrix(embeddings)

	boundaries := PELTDetect(&gram, 0.1, 1, nil)
	if len(boundaries) != 1 {
		t.Fatalf("expected 1 boundary, got %d: %v", len(boundaries), boundaries)
	}
	if boundaries[0] != 2 {
		t.Errorf("expected boundary at 2, got %v", boundaries)
	}
}

func TestPELTDetect_ThreeTopics(t *testing.T) {
	a := []float64{1, 0, 0}
	b := []float64{0, 1, 0}
	c := []float64{0, 0, 1}
	embeddings := makeEmbeddings(a, a, a, b, b, b, c, c, c)
	gram := NewGramMatrix(embeddings)

	boundaries := PELTDetect(&gram, 0.1, 1, nil)
	if len(boundaries) != 2 {
		t.Fatalf("expected 2 boundaries, got %d: %v", len(boundaries), boundaries)
	}
	if boundaries[0] != 2 || boundaries[1] != 5 {
		t.Errorf("expected [2, 5], got %v", boundaries)
	}
}

func TestPELTDetect_AllIdentical(t *testing.T) {
	v := []float64{1, 0, 0}
	embeddings := makeEmbeddings(v, v, v, v, v)
	gram := NewGramMatrix(embeddings)

	// With reasonable beta, no boundaries should be detected in identical data
	boundaries := PELTDetect(&gram, 0.5, 1, nil)
	if len(boundaries) != 0 {
		t.Errorf("expected 0 boundaries for identical vectors, got %v", boundaries)
	}
}

func TestPELTDetect_SingleEmbedding(t *testing.T) {
	embeddings := makeEmbeddings([]float64{1, 0})
	gram := NewGramMatrix(embeddings)

	boundaries := PELTDetect(&gram, 0.1, 1, nil)
	if len(boundaries) != 0 {
		t.Errorf("expected 0 boundaries for single embedding, got %v", boundaries)
	}
}

func TestPELTDetect_TwoEmbeddings(t *testing.T) {
	embeddings := makeEmbeddings([]float64{1, 0}, []float64{0, 1})
	gram := NewGramMatrix(embeddings)

	// With very small beta, should find boundary between the two different vectors
	boundaries := PELTDetect(&gram, 0.01, 1, nil)
	if len(boundaries) != 1 {
		t.Fatalf("expected 1 boundary, got %d: %v", len(boundaries), boundaries)
	}
	if boundaries[0] != 0 {
		t.Errorf("expected boundary at 0, got %v", boundaries)
	}
}

func TestPELTDetect_LargeBeta(t *testing.T) {
	a := []float64{1, 0, 0}
	b := []float64{0, 1, 0}
	embeddings := makeEmbeddings(a, a, b, b)
	gram := NewGramMatrix(embeddings)

	// Very large beta penalizes splits heavily → no boundaries
	boundaries := PELTDetect(&gram, 100.0, 1, nil)
	if len(boundaries) != 0 {
		t.Errorf("expected 0 boundaries with large beta, got %v", boundaries)
	}
}

func TestPELTDetect_TinyBeta(t *testing.T) {
	a := []float64{1, 0, 0}
	b := []float64{0, 1, 0}
	c := []float64{0, 0, 1}
	embeddings := makeEmbeddings(a, b, c)
	gram := NewGramMatrix(embeddings)

	// Very tiny beta → finds many boundaries
	boundaries := PELTDetect(&gram, 0.001, 1, nil)
	if len(boundaries) < 1 {
		t.Errorf("expected at least 1 boundary with tiny beta, got %v", boundaries)
	}
}

func TestPELTDetect_MinGap(t *testing.T) {
	a := []float64{1, 0, 0}
	b := []float64{0, 1, 0}
	c := []float64{0, 0, 1}
	// Each block is a different topic, so many potential boundaries
	embeddings := makeEmbeddings(a, b, c, a, b, c)
	gram := NewGramMatrix(embeddings)

	boundaries := PELTDetect(&gram, 0.01, 3, nil)
	// All boundaries should respect minGap
	for i := 1; i < len(boundaries); i++ {
		gap := boundaries[i] - boundaries[i-1]
		if gap < 3 {
			t.Errorf("boundary gap %d-%d = %d, less than minGap 3",
				boundaries[i-1], boundaries[i], gap)
		}
	}
}

func TestPELTDetect_ProhibitedGaps(t *testing.T) {
	a := []float64{1, 0, 0}
	b := []float64{0, 1, 0}
	embeddings := makeEmbeddings(a, a, a, b, b, b)
	gram := NewGramMatrix(embeddings)

	prohibited := map[int]bool{2: true} // prohibit the natural boundary
	boundaries := PELTDetect(&gram, 0.1, 1, prohibited)

	for _, bd := range boundaries {
		if bd == 2 {
			t.Error("boundary at prohibited gap 2 should not be present")
		}
	}
}

func TestAutoBeta(t *testing.T) {
	a := []float64{1, 0, 0}
	b := []float64{0, 1, 0}
	embeddings := makeEmbeddings(a, a, a, b, b, b)
	gram := NewGramMatrix(embeddings)

	beta := AutoBeta(&gram)
	if beta <= 0 {
		t.Errorf("auto beta should be positive, got %f", beta)
	}
	if math.IsNaN(beta) || math.IsInf(beta, 0) {
		t.Errorf("auto beta should be finite, got %f", beta)
	}

	// With auto beta, should still find the clear boundary
	boundaries := PELTDetect(&gram, beta, 1, nil)
	if len(boundaries) == 0 {
		t.Error("auto beta should allow detection of clear boundary")
	}
}

func TestAutoBeta_SingleBlock(t *testing.T) {
	embeddings := makeEmbeddings([]float64{1, 0})
	gram := NewGramMatrix(embeddings)
	beta := AutoBeta(&gram)
	if beta <= 0 || math.IsNaN(beta) || math.IsInf(beta, 0) {
		t.Errorf("auto beta for single block should be positive finite, got %f", beta)
	}
}

// TestPELTDetect_BruteForceComparison verifies PELT gives the same result
// as brute-force enumeration on small inputs.
func TestPELTDetect_BruteForceComparison(t *testing.T) {
	a := []float64{1, 0, 0}
	b := []float64{0, 1, 0}
	c := []float64{0, 0, 1}
	embeddings := makeEmbeddings(a, a, b, b, c, c)
	gram := NewGramMatrix(embeddings)
	n := gram.n
	beta := 0.3

	// Brute force: enumerate all possible boundary sets for n-1 gaps
	bestCost := math.Inf(1)
	var bestBoundaries []int
	numGaps := n - 1
	for mask := 0; mask < (1 << numGaps); mask++ {
		var bs []int
		for g := 0; g < numGaps; g++ {
			if mask&(1<<g) != 0 {
				bs = append(bs, g)
			}
		}
		cost := totalCost(&gram, bs, n) + beta*float64(len(bs))
		if cost < bestCost {
			bestCost = cost
			bestBoundaries = slices.Clone(bs)
		}
	}

	peltBoundaries := PELTDetect(&gram, beta, 1, nil)
	peltCost := totalCost(&gram, peltBoundaries, n) + beta*float64(len(peltBoundaries))

	if math.Abs(peltCost-bestCost) > 1e-6 {
		t.Errorf("PELT cost %f != brute force cost %f\n  PELT: %v\n  Brute: %v",
			peltCost, bestCost, peltBoundaries, bestBoundaries)
	}
}

// totalCost computes the sum of segment costs for a given set of boundaries.
func totalCost(gram *GramMatrix, boundaries []int, n int) float64 {
	cost := 0.0
	start := 0
	for _, b := range boundaries {
		cost += gram.SegmentCost(start, b+1)
		start = b + 1
	}
	cost += gram.SegmentCost(start, n)
	return cost
}
