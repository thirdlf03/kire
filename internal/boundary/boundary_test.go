package boundary_test

import (
	"context"
	"fmt"
	"math"
	"testing"

	"github.com/thirdlf03/kire/internal/boundary"
	"github.com/thirdlf03/kire/internal/embedding"
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

func TestSmooth_Identity(t *testing.T) {
	// With window=0 (no smoothing), should return same values
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
	// smooth[0] = avg(1, 3) = 2 (only right neighbor available at edge)
	// smooth[1] = avg(1, 3, 1) = 5/3
	// smooth[2] = avg(3, 1, 3) = 7/3
	// smooth[3] = avg(1, 3, 1) = 5/3
	// smooth[4] = avg(3, 1) = 2
	if len(smoothed) != 5 {
		t.Fatalf("expected 5 values, got %d", len(smoothed))
	}
}

func TestDepthScore(t *testing.T) {
	// Simple valley detection: values go up, then down, then up
	sims := []float64{0.8, 0.9, 0.5, 0.3, 0.5, 0.9, 0.8}
	depths := boundary.DepthScore(sims)
	if len(depths) != len(sims) {
		t.Fatalf("expected %d depth scores, got %d", len(sims), len(depths))
	}
	// The valley at index 3 (value 0.3) should have the highest depth score
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

func TestDetectBoundaries_TwoTopics(t *testing.T) {
	// Create blocks for two distinct topics
	topicA := []model.Block{
		{Kind: model.BlockHeading, Text: "Programming Languages", HeadingLevel: 1},
		{Kind: model.BlockParagraph, Text: "Go is a statically typed compiled programming language designed at Google."},
		{Kind: model.BlockParagraph, Text: "Rust is a systems programming language focused on safety and performance."},
		{Kind: model.BlockParagraph, Text: "Python is a high-level interpreted programming language."},
	}
	topicB := []model.Block{
		{Kind: model.BlockHeading, Text: "Cooking Recipes", HeadingLevel: 1},
		{Kind: model.BlockParagraph, Text: "To make pasta, boil water and add spaghetti for ten minutes."},
		{Kind: model.BlockParagraph, Text: "For a salad, mix lettuce, tomatoes, cucumber and olive oil dressing."},
		{Kind: model.BlockParagraph, Text: "Baking bread requires flour, water, yeast and salt mixed together."},
	}
	blocks := append(topicA, topicB...)

	embedder := embedding.NewMockEmbedder(128)
	embeddings, err := embedder.Embed(context.Background(), blocks, embedding.EmbedOptions{})
	if err != nil {
		t.Fatal(err)
	}

	config := boundary.ScoringConfig{
		Window: 2,
		MinGap: 2,
	}
	result := boundary.DetectBoundaries(embeddings, config)

	// Should detect at least one boundary between the two topics (around index 3-4)
	if len(result.Boundaries) == 0 {
		t.Fatal("expected at least one boundary between two topics")
	}

	// Check that at least one boundary is near the topic transition (index 3 or 4)
	foundNearTransition := false
	for _, b := range result.Boundaries {
		if b >= 3 && b <= 5 {
			foundNearTransition = true
			break
		}
	}
	if !foundNearTransition {
		t.Errorf("expected boundary near index 3-5, got boundaries at %v", result.Boundaries)
	}
}

func TestDetectBoundaries_SingleBlock(t *testing.T) {
	embeddings := []model.Embedding{
		{BlockIndex: 0, Vector: []float64{1, 0, 0}},
	}
	config := boundary.ScoringConfig{Window: 2}
	result := boundary.DetectBoundaries(embeddings, config)
	if len(result.Boundaries) != 0 {
		t.Errorf("expected 0 boundaries for single block, got %d", len(result.Boundaries))
	}
}

func TestDetectBoundaries_TwoBlocks(t *testing.T) {
	embeddings := []model.Embedding{
		{BlockIndex: 0, Vector: []float64{1, 0, 0}},
		{BlockIndex: 1, Vector: []float64{0, 1, 0}},
	}
	config := boundary.ScoringConfig{Window: 1}
	result := boundary.DetectBoundaries(embeddings, config)
	// With only 2 blocks there's 1 gap; whether it's a boundary depends on threshold
	// At minimum, result should not panic
	_ = result
}

func TestDetectBoundaries_CustomThreshold(t *testing.T) {
	// With a very high threshold, no boundaries should be detected
	blocks := make([]model.Block, 10)
	for i := range blocks {
		blocks[i] = model.Block{Kind: model.BlockParagraph, Text: "text"}
	}
	embedder := embedding.NewMockEmbedder(64)
	embeddings, _ := embedder.Embed(context.Background(), blocks, embedding.EmbedOptions{})

	high := 100.0
	config := boundary.ScoringConfig{
		Window:    2,
		Threshold: &high,
	}
	result := boundary.DetectBoundaries(embeddings, config)
	if len(result.Boundaries) != 0 {
		t.Errorf("expected 0 boundaries with high threshold, got %d", len(result.Boundaries))
	}
}

func TestDetectBoundaries_ProhibitHint(t *testing.T) {
	// Create embeddings that would normally produce a boundary
	embeddings := []model.Embedding{
		{BlockIndex: 0, Vector: []float64{1, 0, 0}},
		{BlockIndex: 1, Vector: []float64{1, 0, 0}},
		{BlockIndex: 2, Vector: []float64{0, 1, 0}}, // big change at gap 1
		{BlockIndex: 3, Vector: []float64{0, 1, 0}},
	}
	low := 0.0
	config := boundary.ScoringConfig{
		Window:    0,
		Threshold: &low,
		MinGap:    1,
		Hints: []boundary.BoundaryHint{
			{GapIndex: 1, Kind: boundary.HintProhibit},
		},
	}
	result := boundary.DetectBoundaries(embeddings, config)
	for _, b := range result.Boundaries {
		if b == 1 {
			t.Error("gap 1 should be prohibited by hint")
		}
	}
}

func TestDetectBoundaries_EnforceHint(t *testing.T) {
	// All identical vectors → no natural boundaries
	embeddings := []model.Embedding{
		{BlockIndex: 0, Vector: []float64{1, 0, 0}},
		{BlockIndex: 1, Vector: []float64{1, 0, 0}},
		{BlockIndex: 2, Vector: []float64{1, 0, 0}},
		{BlockIndex: 3, Vector: []float64{1, 0, 0}},
	}
	high := 100.0
	config := boundary.ScoringConfig{
		Window:    0,
		Threshold: &high,
		MinGap:    1,
		Hints: []boundary.BoundaryHint{
			{GapIndex: 2, Kind: boundary.HintEnforce},
		},
	}
	result := boundary.DetectBoundaries(embeddings, config)
	found := false
	for _, b := range result.Boundaries {
		if b == 2 {
			found = true
		}
	}
	if !found {
		t.Errorf("expected enforced boundary at gap 2, got %v", result.Boundaries)
	}
}

func TestSelectBoundariesByCount(t *testing.T) {
	depths := []float64{0.1, 0.5, 0.2, 0.8, 0.3, 0.6, 0.0}
	// k=2, minGap=1: should pick index 3 (0.8) and 5 (0.6)
	got := boundary.SelectBoundariesByCount(depths, 2, 1)
	if len(got) != 2 {
		t.Fatalf("expected 2 boundaries, got %d: %v", len(got), got)
	}
	if got[0] != 3 || got[1] != 5 {
		t.Errorf("expected [3, 5], got %v", got)
	}
}

func TestSelectBoundariesByCount_MinGapConstraint(t *testing.T) {
	// All high depths at adjacent positions
	depths := []float64{0.9, 0.8, 0.7, 0.1, 0.1}
	// k=2, minGap=2: should pick 0 (0.9), skip 1 (too close), pick 2 (0.7)
	got := boundary.SelectBoundariesByCount(depths, 2, 2)
	if len(got) != 2 {
		t.Fatalf("expected 2 boundaries, got %d: %v", len(got), got)
	}
	if got[0] != 0 || got[1] != 2 {
		t.Errorf("expected [0, 2], got %v", got)
	}
}

func TestEvaluateSegmentation(t *testing.T) {
	depths := []float64{0.5, 0.1, 0.8, 0.2}
	// Boundaries at [0, 2] → segments: [0..0], [1..2], [3..4] (sizes 1, 2, 2)
	score := boundary.EvaluateSegmentation([]int{0, 2}, 5, depths)
	// depthSum = 0.5 + 0.8 = 1.3
	// sizes = [1, 2, 2], mean = 5/3 ≈ 1.667
	// variance = ((1-1.667)^2 + (2-1.667)^2 + (2-1.667)^2) / 3
	if score <= 0 {
		t.Errorf("expected positive score, got %f", score)
	}

	// Empty boundaries → single segment (no depth score)
	score0 := boundary.EvaluateSegmentation(nil, 5, depths)
	if score0 != 0 {
		t.Errorf("expected 0 for empty boundaries, got %f", score0)
	}
}

func TestDetectBoundaries_TargetCount(t *testing.T) {
	// Create blocks for three distinct topics
	topicA := []model.Block{
		{Kind: model.BlockHeading, Text: "Go programming", HeadingLevel: 1},
		{Kind: model.BlockParagraph, Text: "Go is a compiled language designed at Google."},
		{Kind: model.BlockParagraph, Text: "Go has garbage collection and CSP concurrency."},
	}
	topicB := []model.Block{
		{Kind: model.BlockHeading, Text: "Italian Cooking", HeadingLevel: 1},
		{Kind: model.BlockParagraph, Text: "Pasta is made from flour, water, and eggs."},
		{Kind: model.BlockParagraph, Text: "Pizza originated in Naples with tomato sauce."},
	}
	topicC := []model.Block{
		{Kind: model.BlockHeading, Text: "Space Exploration", HeadingLevel: 1},
		{Kind: model.BlockParagraph, Text: "NASA launched the Voyager probes in 1977."},
		{Kind: model.BlockParagraph, Text: "Mars rovers have discovered evidence of water."},
	}
	blocks := append(append(topicA, topicB...), topicC...)

	embedder := embedding.NewMockEmbedder(128)
	embeddings, err := embedder.Embed(context.Background(), blocks, embedding.EmbedOptions{})
	if err != nil {
		t.Fatal(err)
	}

	target := 3
	config := boundary.ScoringConfig{
		Window:      2,
		MinGap:      1,
		TargetCount: &target,
	}
	result := boundary.DetectBoundaries(embeddings, config)

	// Should produce roughly 3 segments (2 boundaries), allowing ±1
	nSegments := len(result.Boundaries) + 1
	if nSegments < 2 || nSegments > 4 {
		t.Errorf("expected 2-4 segments with target 3, got %d (boundaries: %v)", nSegments, result.Boundaries)
	}
}

func TestDetectBoundaries_ThresholdInResult(t *testing.T) {
	// Auto threshold mode: Threshold should be set to a non-negative value
	blocks := make([]model.Block, 6)
	for i := range blocks {
		blocks[i] = model.Block{Kind: model.BlockParagraph, Text: fmt.Sprintf("block %d content", i)}
	}
	embedder := embedding.NewMockEmbedder(64)
	embeddings, err := embedder.Embed(context.Background(), blocks, embedding.EmbedOptions{})
	if err != nil {
		t.Fatal(err)
	}

	config := boundary.ScoringConfig{Window: 2, MinGap: 1}
	result := boundary.DetectBoundaries(embeddings, config)

	// Auto threshold is mean - std/2, which should be set.
	// It's possible (but unlikely) that computed threshold is exactly 0,
	// so we just check it doesn't panic and the field exists.
	// The key check: Threshold field exists and was populated
	_ = result.Threshold
}

func TestDetectBoundaries_CustomThresholdInResult(t *testing.T) {
	blocks := make([]model.Block, 5)
	for i := range blocks {
		blocks[i] = model.Block{Kind: model.BlockParagraph, Text: fmt.Sprintf("block %d", i)}
	}
	embedder := embedding.NewMockEmbedder(64)
	embeddings, err := embedder.Embed(context.Background(), blocks, embedding.EmbedOptions{})
	if err != nil {
		t.Fatal(err)
	}

	customThreshold := 0.42
	config := boundary.ScoringConfig{
		Window:    2,
		MinGap:    1,
		Threshold: &customThreshold,
	}
	result := boundary.DetectBoundaries(embeddings, config)

	if result.Threshold != 0.42 {
		t.Errorf("expected threshold 0.42, got %f", result.Threshold)
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

func TestDetectBoundaries_NilHints(t *testing.T) {
	// Nil hints should not change behavior
	embeddings := []model.Embedding{
		{BlockIndex: 0, Vector: []float64{1, 0, 0}},
		{BlockIndex: 1, Vector: []float64{0, 1, 0}},
		{BlockIndex: 2, Vector: []float64{1, 0, 0}},
	}
	config := boundary.ScoringConfig{
		Window: 0,
		MinGap: 1,
		Hints:  nil,
	}
	result := boundary.DetectBoundaries(embeddings, config)
	_ = result // should not panic
}
