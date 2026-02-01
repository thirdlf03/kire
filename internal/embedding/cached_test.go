package embedding_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/thirdlf03/kire/internal/cache"
	"github.com/thirdlf03/kire/internal/embedding"
	"github.com/thirdlf03/kire/internal/model"
)

type callCountEmbedder struct {
	inner embedding.Embedder
	calls int
}

func (c *callCountEmbedder) Embed(ctx context.Context, blocks []model.Block, opts embedding.EmbedOptions) ([]model.Embedding, error) {
	c.calls++
	return c.inner.Embed(ctx, blocks, opts)
}

func TestCachedEmbedder_SkipsAPIOnHit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "embed.cache.json")
	c, err := cache.NewJSONCache(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()

	inner := &callCountEmbedder{inner: embedding.NewMockEmbedder(64)}
	cached := embedding.NewCachedEmbedder(inner, c, "test-model")

	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "Hello world"},
		{Kind: model.BlockParagraph, Text: "Foo bar"},
	}
	opts := embedding.EmbedOptions{TaskType: "SEMANTIC_SIMILARITY", Dimensions: 64}

	// First call: should use inner embedder
	_, err = cached.Embed(context.Background(), blocks, opts)
	if err != nil {
		t.Fatal(err)
	}
	if inner.calls != 1 {
		t.Errorf("expected 1 inner call, got %d", inner.calls)
	}

	// Second call: should use cache (no additional inner call)
	results, err := cached.Embed(context.Background(), blocks, opts)
	if err != nil {
		t.Fatal(err)
	}
	if inner.calls != 1 {
		t.Errorf("expected still 1 inner call after cache hit, got %d", inner.calls)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

func TestCachedEmbedder_Close(t *testing.T) {
	path := filepath.Join(t.TempDir(), "embed.cache.json")
	c, err := cache.NewJSONCache(path)
	if err != nil {
		t.Fatal(err)
	}
	inner := embedding.NewMockEmbedder(64)
	cached := embedding.NewCachedEmbedder(inner, c, "test-model")
	if err := cached.Close(); err != nil {
		t.Errorf("Close() returned error: %v", err)
	}
}

func TestCachedEmbedder_PartialCacheHit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "embed.cache.json")
	c, err := cache.NewJSONCache(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()

	inner := &callCountEmbedder{inner: embedding.NewMockEmbedder(64)}
	cached := embedding.NewCachedEmbedder(inner, c, "test-model")

	block1 := []model.Block{{Kind: model.BlockParagraph, Text: "Cached text"}}
	opts := embedding.EmbedOptions{TaskType: "SEMANTIC_SIMILARITY", Dimensions: 64}

	// Cache block1
	_, _ = cached.Embed(context.Background(), block1, opts)
	if inner.calls != 1 {
		t.Fatalf("expected 1 call, got %d", inner.calls)
	}

	// Now embed block1 + new block2
	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "Cached text"},
		{Kind: model.BlockParagraph, Text: "New text"},
	}
	results, err := cached.Embed(context.Background(), blocks, opts)
	if err != nil {
		t.Fatal(err)
	}
	// Should call inner only for the new block
	if inner.calls != 2 {
		t.Errorf("expected 2 inner calls (1 for new block), got %d", inner.calls)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}
