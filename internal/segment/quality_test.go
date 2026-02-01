package segment

import (
	"math"
	"testing"

	"github.com/thirdlf03/kire/internal/boundary"
	"github.com/thirdlf03/kire/internal/model"
)

func TestComputeCoherence(t *testing.T) {
	tests := []struct {
		name       string
		embeddings []model.Embedding
		indices    []int
		wantMin    float64
		wantMax    float64
	}{
		{
			name:    "empty indices",
			indices: nil,
			wantMin: 0, wantMax: 0,
		},
		{
			name:    "single block",
			indices: []int{0},
			embeddings: []model.Embedding{
				{BlockIndex: 0, Vector: []float64{1, 0, 0}},
			},
			wantMin: 1.0, wantMax: 1.0,
		},
		{
			name:    "identical vectors",
			indices: []int{0, 1, 2},
			embeddings: []model.Embedding{
				{BlockIndex: 0, Vector: []float64{1, 0, 0}},
				{BlockIndex: 1, Vector: []float64{1, 0, 0}},
				{BlockIndex: 2, Vector: []float64{1, 0, 0}},
			},
			wantMin: 0.99, wantMax: 1.01,
		},
		{
			name:    "orthogonal vectors low coherence",
			indices: []int{0, 1},
			embeddings: []model.Embedding{
				{BlockIndex: 0, Vector: []float64{1, 0, 0}},
				{BlockIndex: 1, Vector: []float64{0, 1, 0}},
			},
			wantMin: -0.01, wantMax: 0.01,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeCoherence(tt.indices, tt.embeddings)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("ComputeCoherence() = %v, want [%v, %v]", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestComputeBoundaryConfidences(t *testing.T) {
	tests := []struct {
		name        string
		boundaries  []int
		depthScores []float64
		threshold   float64
		wantLen     int
	}{
		{
			name:       "empty",
			boundaries: nil,
			wantLen:    0,
		},
		{
			name:        "single boundary well above threshold",
			boundaries:  []int{2},
			depthScores: []float64{0.1, 0.2, 0.8, 0.1, 0.3},
			threshold:   0.3,
			wantLen:     1,
		},
		{
			name:        "multiple boundaries",
			boundaries:  []int{1, 3},
			depthScores: []float64{0.1, 0.6, 0.2, 0.9, 0.1},
			threshold:   0.3,
			wantLen:     2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			br := boundary.BoundaryResult{
				Boundaries:  tt.boundaries,
				DepthScores: tt.depthScores,
				Threshold:   tt.threshold,
			}
			got := ComputeBoundaryConfidences(br)
			if len(got) != tt.wantLen {
				t.Fatalf("len = %d, want %d", len(got), tt.wantLen)
			}
			for i, c := range got {
				if c < 0 || c > 1 {
					t.Errorf("confidence[%d] = %v, want [0, 1]", i, c)
				}
			}
		})
	}
}

func TestComputeQualityMetrics(t *testing.T) {
	embeddings := []model.Embedding{
		{BlockIndex: 0, Vector: []float64{1, 0}},
		{BlockIndex: 1, Vector: []float64{0.9, 0.1}},
		{BlockIndex: 2, Vector: []float64{0, 1}},
		{BlockIndex: 3, Vector: []float64{0.1, 0.9}},
	}
	segments := []model.Segment{
		{Blocks: []model.Block{
			{Range: model.SourceRange{Start: 0, End: 5}},
			{Range: model.SourceRange{Start: 5, End: 10}},
		}},
		{Blocks: []model.Block{
			{Range: model.SourceRange{Start: 10, End: 15}},
			{Range: model.SourceRange{Start: 15, End: 20}},
		}},
	}
	allBlocks := []model.Block{
		{Range: model.SourceRange{Start: 0, End: 5}},
		{Range: model.SourceRange{Start: 5, End: 10}},
		{Range: model.SourceRange{Start: 10, End: 15}},
		{Range: model.SourceRange{Start: 15, End: 20}},
	}
	br := boundary.BoundaryResult{
		Boundaries:  []int{1},
		DepthScores: []float64{0.1, 0.8, 0.2},
		Threshold:   0.3,
	}

	qm := ComputeQualityMetrics(segments, allBlocks, embeddings, br)

	if qm.MeanCoherence <= 0 {
		t.Errorf("MeanCoherence = %v, want > 0", qm.MeanCoherence)
	}
	if qm.MinCoherence <= 0 {
		t.Errorf("MinCoherence = %v, want > 0", qm.MinCoherence)
	}
	if qm.MeanConfidence < 0 || qm.MeanConfidence > 1 {
		t.Errorf("MeanConfidence = %v, want [0, 1]", qm.MeanConfidence)
	}
	if math.IsNaN(qm.SegmentCountScore) {
		t.Error("SegmentCountScore is NaN")
	}
}
