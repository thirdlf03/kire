package boundary_test

import (
	"math"
	"testing"

	"github.com/thirdlf03/kire/internal/boundary"
	"github.com/thirdlf03/kire/internal/model"
)

func TestCosineSimilarity_Identical(t *testing.T) {
	a := []float64{1, 0, 0}
	b := []float64{1, 0, 0}
	sim := boundary.CosineSimilarity(a, b)
	if math.Abs(sim-1.0) > 1e-9 {
		t.Errorf("expected 1.0, got %f", sim)
	}
}

func TestCosineSimilarity_Orthogonal(t *testing.T) {
	a := []float64{1, 0}
	b := []float64{0, 1}
	sim := boundary.CosineSimilarity(a, b)
	if math.Abs(sim) > 1e-9 {
		t.Errorf("expected 0.0, got %f", sim)
	}
}

func TestCosineSimilarity_Opposite(t *testing.T) {
	a := []float64{1, 0}
	b := []float64{-1, 0}
	sim := boundary.CosineSimilarity(a, b)
	if math.Abs(sim-(-1.0)) > 1e-9 {
		t.Errorf("expected -1.0, got %f", sim)
	}
}

func TestCosineSimilarity_ZeroVector(t *testing.T) {
	a := []float64{0, 0, 0}
	b := []float64{1, 2, 3}
	sim := boundary.CosineSimilarity(a, b)
	if sim != 0 {
		t.Errorf("expected 0 for zero vector, got %f", sim)
	}
}

func TestCosineSimilarity_NaNInput(t *testing.T) {
	a := []float64{math.NaN(), 1, 0}
	b := []float64{1, 0, 0}
	sim := boundary.CosineSimilarity(a, b)
	if math.IsNaN(sim) || math.IsInf(sim, 0) {
		t.Errorf("expected 0 for NaN input, got %f", sim)
	}
}

func TestCosineSimilarity_InfInput(t *testing.T) {
	a := []float64{math.Inf(1), 1, 0}
	b := []float64{1, 0, 0}
	sim := boundary.CosineSimilarity(a, b)
	if math.IsNaN(sim) || math.IsInf(sim, 0) {
		t.Errorf("expected 0 for Inf input, got %f", sim)
	}
}

func TestSmooth_Identity(t *testing.T) {
	vals := []float64{1, 2, 3, 4, 5}
	smoothed := boundary.Smooth(vals, 0)
	for i, v := range vals {
		if smoothed[i] != v {
			t.Errorf("smooth[%d]: expected %f, got %f", i, v, smoothed[i])
		}
	}
}

func TestSmooth_Window1(t *testing.T) {
	vals := []float64{1, 3, 1, 3, 1}
	smoothed := boundary.Smooth(vals, 1)
	if len(smoothed) != 5 {
		t.Fatalf("expected 5 values, got %d", len(smoothed))
	}
}

func TestDepthScore(t *testing.T) {
	sims := []float64{0.8, 0.9, 0.5, 0.3, 0.5, 0.9, 0.8}
	depths := boundary.DepthScore(sims)
	if len(depths) != len(sims) {
		t.Fatalf("expected %d depth scores, got %d", len(sims), len(depths))
	}
	maxIdx := 0
	for i, d := range depths {
		if d > depths[maxIdx] {
			maxIdx = i
		}
	}
	if maxIdx != 3 {
		t.Errorf("expected deepest valley at index 3, got %d (depths: %v)", maxIdx, depths)
	}
}

func TestMeanPool(t *testing.T) {
	tests := []struct {
		name   string
		vecs   [][]float64
		expect []float64
	}{
		{"empty", nil, nil},
		{"single", [][]float64{{1, 2, 3}}, []float64{1, 2, 3}},
		{"two vectors", [][]float64{{1, 0, 0}, {0, 1, 0}}, []float64{0.5, 0.5, 0}},
		{"three vectors", [][]float64{{3, 0, 0}, {0, 6, 0}, {0, 0, 9}}, []float64{1, 2, 3}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := boundary.MeanPool(tt.vecs)
			if tt.expect == nil {
				if got != nil {
					t.Errorf("expected nil, got %v", got)
				}
				return
			}
			if len(got) != len(tt.expect) {
				t.Fatalf("expected len %d, got %d", len(tt.expect), len(got))
			}
			for i := range got {
				if math.Abs(got[i]-tt.expect[i]) > 1e-9 {
					t.Errorf("[%d] expected %f, got %f", i, tt.expect[i], got[i])
				}
			}
		})
	}
}

func TestMeanPool_SingleReturnsClone(t *testing.T) {
	v := []float64{1, 2, 3}
	got := boundary.MeanPool([][]float64{v})
	v[0] = 99
	if got[0] != 1 {
		t.Error("MeanPool single should return a clone")
	}
}

func TestBlockSimilarities_K1_MatchesAdjacent(t *testing.T) {
	embeddings := []model.Embedding{
		{BlockIndex: 0, Vector: []float64{1, 0, 0}},
		{BlockIndex: 1, Vector: []float64{0.9, 0.1, 0}},
		{BlockIndex: 2, Vector: []float64{0, 1, 0}},
		{BlockIndex: 3, Vector: []float64{0, 0.9, 0.1}},
	}
	got := boundary.BlockSimilarities(embeddings, 1)
	if len(got) != 3 {
		t.Fatalf("expected 3 sims, got %d", len(got))
	}
	for i := 0; i < 3; i++ {
		adj := boundary.CosineSimilarity(embeddings[i].Vector, embeddings[i+1].Vector)
		if math.Abs(got[i]-adj) > 1e-9 {
			t.Errorf("gap %d: expected %f, got %f", i, adj, got[i])
		}
	}
}

func TestBlockSimilarities_K0_MatchesAdjacent(t *testing.T) {
	embeddings := []model.Embedding{
		{BlockIndex: 0, Vector: []float64{1, 0, 0}},
		{BlockIndex: 1, Vector: []float64{0, 1, 0}},
		{BlockIndex: 2, Vector: []float64{0, 0, 1}},
	}
	got := boundary.BlockSimilarities(embeddings, 0)
	if len(got) != 2 {
		t.Fatalf("expected 2 sims, got %d", len(got))
	}
	for i := 0; i < 2; i++ {
		adj := boundary.CosineSimilarity(embeddings[i].Vector, embeddings[i+1].Vector)
		if math.Abs(got[i]-adj) > 1e-9 {
			t.Errorf("gap %d: expected %f, got %f", i, adj, got[i])
		}
	}
}

func TestBlockSimilarities_K3_Basic(t *testing.T) {
	topicA := []float64{1, 0, 0}
	topicB := []float64{0, 1, 0}
	embeddings := []model.Embedding{
		{BlockIndex: 0, Vector: topicA},
		{BlockIndex: 1, Vector: topicA},
		{BlockIndex: 2, Vector: topicA},
		{BlockIndex: 3, Vector: topicB},
		{BlockIndex: 4, Vector: topicB},
		{BlockIndex: 5, Vector: topicB},
	}
	sims := boundary.BlockSimilarities(embeddings, 3)
	if len(sims) != 5 {
		t.Fatalf("expected 5 sims, got %d", len(sims))
	}
	minIdx := 0
	for i, s := range sims {
		if s < sims[minIdx] {
			minIdx = i
		}
	}
	if minIdx != 2 {
		t.Errorf("expected lowest sim at gap 2, got gap %d (sims: %v)", minIdx, sims)
	}
}

func TestBlockSimilarities_EdgeCases(t *testing.T) {
	t.Run("k > n/2", func(t *testing.T) {
		embeddings := []model.Embedding{
			{BlockIndex: 0, Vector: []float64{1, 0}},
			{BlockIndex: 1, Vector: []float64{0, 1}},
			{BlockIndex: 2, Vector: []float64{1, 0}},
		}
		sims := boundary.BlockSimilarities(embeddings, 10)
		if len(sims) != 2 {
			t.Errorf("expected 2 sims, got %d", len(sims))
		}
	})
	t.Run("n=2", func(t *testing.T) {
		embeddings := []model.Embedding{
			{BlockIndex: 0, Vector: []float64{1, 0}},
			{BlockIndex: 1, Vector: []float64{0, 1}},
		}
		sims := boundary.BlockSimilarities(embeddings, 3)
		if len(sims) != 1 {
			t.Errorf("expected 1 sim, got %d", len(sims))
		}
	})
	t.Run("n=1", func(t *testing.T) {
		embeddings := []model.Embedding{
			{BlockIndex: 0, Vector: []float64{1, 0}},
		}
		sims := boundary.BlockSimilarities(embeddings, 3)
		if len(sims) != 0 {
			t.Errorf("expected 0 sims, got %d", len(sims))
		}
	})
	t.Run("empty", func(t *testing.T) {
		sims := boundary.BlockSimilarities(nil, 3)
		if len(sims) != 0 {
			t.Errorf("expected 0 sims, got %d", len(sims))
		}
	})
}
