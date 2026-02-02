package boundary

import (
	"math"
	"slices"
	"testing"
)

func TestDPKSegment_ThreeTopics(t *testing.T) {
	a := []float64{1, 0, 0}
	b := []float64{0, 1, 0}
	c := []float64{0, 0, 1}
	embeddings := makeEmbeddings(a, a, b, b, c, c)
	gram := NewGramMatrix(embeddings)

	// k=3 segments → 2 boundaries
	boundaries := DPKSegment(&gram, 3, 1, nil)
	if len(boundaries) != 2 {
		t.Fatalf("expected 2 boundaries, got %d: %v", len(boundaries), boundaries)
	}
	if boundaries[0] != 1 || boundaries[1] != 3 {
		t.Errorf("expected [1, 3], got %v", boundaries)
	}
}

func TestDPKSegment_SingleSegment(t *testing.T) {
	a := []float64{1, 0, 0}
	embeddings := makeEmbeddings(a, a, a)
	gram := NewGramMatrix(embeddings)

	// k=1 → no boundaries
	boundaries := DPKSegment(&gram, 1, 1, nil)
	if len(boundaries) != 0 {
		t.Errorf("expected 0 boundaries, got %v", boundaries)
	}
}

func TestDPKSegment_KEqualsN(t *testing.T) {
	a := []float64{1, 0}
	b := []float64{0, 1}
	embeddings := makeEmbeddings(a, b, a)
	gram := NewGramMatrix(embeddings)

	// k=n → n-1 boundaries (one block per segment)
	boundaries := DPKSegment(&gram, 3, 1, nil)
	if len(boundaries) != 2 {
		t.Fatalf("expected 2 boundaries, got %d: %v", len(boundaries), boundaries)
	}
	if boundaries[0] != 0 || boundaries[1] != 1 {
		t.Errorf("expected [0, 1], got %v", boundaries)
	}
}

func TestDPKSegment_KGreaterThanN(t *testing.T) {
	a := []float64{1, 0}
	embeddings := makeEmbeddings(a, a, a)
	gram := NewGramMatrix(embeddings)

	// k > n → capped to n segments
	boundaries := DPKSegment(&gram, 10, 1, nil)
	if len(boundaries) != 2 { // n-1 = 2
		t.Errorf("expected 2 boundaries (capped), got %d: %v", len(boundaries), boundaries)
	}
}

func TestDPKSegment_MinGap(t *testing.T) {
	a := []float64{1, 0, 0}
	b := []float64{0, 1, 0}
	c := []float64{0, 0, 1}
	embeddings := makeEmbeddings(a, b, c, a, b, c)
	gram := NewGramMatrix(embeddings)

	boundaries := DPKSegment(&gram, 3, 2, nil)
	for i := 1; i < len(boundaries); i++ {
		gap := boundaries[i] - boundaries[i-1]
		if gap < 2 {
			t.Errorf("boundary gap %d-%d = %d, less than minGap 2",
				boundaries[i-1], boundaries[i], gap)
		}
	}
}

func TestDPKSegment_Prohibited(t *testing.T) {
	a := []float64{1, 0, 0}
	b := []float64{0, 1, 0}
	embeddings := makeEmbeddings(a, a, a, b, b, b)
	gram := NewGramMatrix(embeddings)

	prohibited := map[int]bool{2: true}
	boundaries := DPKSegment(&gram, 2, 1, prohibited)
	for _, bd := range boundaries {
		if bd == 2 {
			t.Error("boundary at prohibited gap 2 should not be present")
		}
	}
}

// TestDPKSegment_BruteForce verifies DP gives optimal result for small inputs.
func TestDPKSegment_BruteForce(t *testing.T) {
	a := []float64{1, 0, 0}
	b := []float64{0, 1, 0}
	c := []float64{0, 0, 1}
	embeddings := makeEmbeddings(a, a, b, c, c, a)
	gram := NewGramMatrix(embeddings)
	n := gram.n
	k := 3 // 3 segments → 2 boundaries

	// Brute force: enumerate all combinations of 2 boundaries from n-1 gaps
	bestCost := math.Inf(1)
	var bestBoundaries []int
	for i := 0; i < n-1; i++ {
		for j := i + 1; j < n-1; j++ {
			bs := []int{i, j}
			cost := totalCost(&gram, bs, n)
			if cost < bestCost {
				bestCost = cost
				bestBoundaries = slices.Clone(bs)
			}
		}
	}

	dpBoundaries := DPKSegment(&gram, k, 1, nil)
	dpCost := totalCost(&gram, dpBoundaries, n)

	if math.Abs(dpCost-bestCost) > 1e-6 {
		t.Errorf("DP cost %f != brute force cost %f\n  DP: %v\n  Brute: %v",
			dpCost, bestCost, dpBoundaries, bestBoundaries)
	}
}

func TestDPKSegment_Empty(t *testing.T) {
	gram := NewGramMatrix(nil)
	boundaries := DPKSegment(&gram, 3, 1, nil)
	if len(boundaries) != 0 {
		t.Errorf("expected 0 boundaries for empty, got %v", boundaries)
	}
}
