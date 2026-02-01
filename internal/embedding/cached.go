package embedding

import (
	"context"
	"log"

	"github.com/thirdlf03/kire/internal/cache"
	"github.com/thirdlf03/kire/internal/model"
)

// CachedEmbedder wraps an Embedder with a cache layer.
type CachedEmbedder struct {
	inner     Embedder
	cache     cache.Cache
	modelName string
}

func NewCachedEmbedder(inner Embedder, c cache.Cache, modelName string) *CachedEmbedder {
	return &CachedEmbedder{inner: inner, cache: c, modelName: modelName}
}

func (ce *CachedEmbedder) Embed(ctx context.Context, blocks []model.Block, opts EmbedOptions) ([]model.Embedding, error) {
	results := make([]model.Embedding, len(blocks))
	var uncachedBlocks []model.Block
	var uncachedIndices []int

	// Check cache (fail-open: cache errors are silently ignored, causing re-embedding)
	for i, b := range blocks {
		key := cache.CacheKey(b.Text, ce.modelName, opts.TaskType, opts.Dimensions)
		if emb, ok, err := ce.cache.Get(key); err == nil && ok {
			emb.BlockIndex = i
			results[i] = emb
		} else {
			uncachedBlocks = append(uncachedBlocks, b)
			uncachedIndices = append(uncachedIndices, i)
		}
	}

	if len(uncachedBlocks) == 0 {
		return results, nil
	}

	// Embed uncached blocks
	embeddings, err := ce.inner.Embed(ctx, uncachedBlocks, opts)
	if err != nil {
		return nil, err
	}

	// Store in cache and results
	for j, emb := range embeddings {
		idx := uncachedIndices[j]
		emb.BlockIndex = idx
		results[idx] = emb

		key := cache.CacheKey(uncachedBlocks[j].Text, ce.modelName, opts.TaskType, opts.Dimensions)
		if err := ce.cache.Put(key, emb); err != nil {
			log.Printf("WARNING: cache put: %v", err)
		}
	}

	return results, nil
}

// Close flushes the underlying cache to disk.
func (ce *CachedEmbedder) Close() error {
	return ce.cache.Close()
}
