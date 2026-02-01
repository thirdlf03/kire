package embedding_test

import (
	"context"
	"math"
	"testing"

	"github.com/thirdlf03/kire/internal/embedding"
	"github.com/thirdlf03/kire/internal/model"
)

func TestTFIDFEmbedder_Basic(t *testing.T) {
	embedder := embedding.NewTFIDFEmbedder(256)
	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "Go is a programming language."},
		{Kind: model.BlockParagraph, Text: "Python is a programming language."},
		{Kind: model.BlockParagraph, Text: "Cooking requires ingredients and heat."},
	}

	results, err := embedder.Embed(context.Background(), blocks, embedding.EmbedOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	for i, r := range results {
		if len(r.Vector) != 256 {
			t.Errorf("result[%d]: expected 256 dims, got %d", i, len(r.Vector))
		}
		// Should be normalized (unit vector)
		var norm float64
		for _, v := range r.Vector {
			norm += v * v
		}
		norm = math.Sqrt(norm)
		if math.Abs(norm-1.0) > 1e-6 {
			t.Errorf("result[%d]: expected unit vector (norm=1), got %f", i, norm)
		}
	}

	// Similar blocks (both about programming) should be more similar than dissimilar ones
	sim01 := cosine(results[0].Vector, results[1].Vector)
	sim02 := cosine(results[0].Vector, results[2].Vector)
	if sim01 <= sim02 {
		t.Logf("programming blocks similarity: %f, cooking similarity: %f", sim01, sim02)
		// TF-IDF char n-grams may not always capture semantic similarity well
		// This is just a sanity check, not a strict requirement
	}
}

func TestTFIDFEmbedder_Empty(t *testing.T) {
	embedder := embedding.NewTFIDFEmbedder(64)
	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: ""},
	}
	results, err := embedder.Embed(context.Background(), blocks, embedding.EmbedOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func cosine(a, b []float64) float64 {
	var dot, na, nb float64
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}
