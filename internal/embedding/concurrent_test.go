package embedding_test

import (
	"context"
	"testing"

	"github.com/thirdlf03/kire/internal/embedding"
	"github.com/thirdlf03/kire/internal/model"
)

func TestConcurrentEmbedder_SmallBatch(t *testing.T) {
	inner := embedding.NewMockEmbedder(64)
	concurrent := embedding.NewConcurrentEmbedder(inner, 2, 0, 32)

	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "Hello"},
		{Kind: model.BlockParagraph, Text: "World"},
	}

	results, err := concurrent.Embed(context.Background(), blocks, embedding.EmbedOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

func TestConcurrentEmbedder_DefaultBatchSize(t *testing.T) {
	inner := embedding.NewMockEmbedder(64)
	concurrent := embedding.NewConcurrentEmbedder(inner, 2, 0, 0) // batchSize=0 → default 32

	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "Hello"},
	}

	results, err := concurrent.Embed(context.Background(), blocks, embedding.EmbedOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestConcurrentEmbedder_MultiBatch(t *testing.T) {
	inner := embedding.NewMockEmbedder(64)
	// Batch size 2, so 5 blocks = 3 batches
	concurrent := embedding.NewConcurrentEmbedder(inner, 3, 0, 2)

	blocks := make([]model.Block, 5)
	for i := range blocks {
		blocks[i] = model.Block{Kind: model.BlockParagraph, Text: "Text"}
	}

	results, err := concurrent.Embed(context.Background(), blocks, embedding.EmbedOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}
	// Verify block indices
	for i, r := range results {
		if r.BlockIndex != i {
			t.Errorf("result[%d]: expected BlockIndex %d, got %d", i, i, r.BlockIndex)
		}
	}
}
