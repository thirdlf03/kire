package embedding

import (
	"context"
	"fmt"

	"github.com/thirdlf03/kire/internal/concurrency"
	"github.com/thirdlf03/kire/internal/model"
)

// ConcurrentEmbedder parallelizes embedding across batches using a worker pool.
type ConcurrentEmbedder struct {
	inner     Embedder
	pool      *concurrency.Pool
	batchSize int
}

func NewConcurrentEmbedder(inner Embedder, concurrencyLimit int, qps float64, batchSize int) *ConcurrentEmbedder {
	if batchSize <= 0 {
		batchSize = 32
	}
	return &ConcurrentEmbedder{
		inner:     inner,
		pool:      concurrency.NewPool(concurrencyLimit, qps),
		batchSize: batchSize,
	}
}

func (ce *ConcurrentEmbedder) Embed(ctx context.Context, blocks []model.Block, opts EmbedOptions) ([]model.Embedding, error) {
	if len(blocks) <= ce.batchSize {
		return ce.inner.Embed(ctx, blocks, opts)
	}

	// Split into batches
	type batch struct {
		start  int
		blocks []model.Block
	}
	var batches []batch
	for i := 0; i < len(blocks); i += ce.batchSize {
		end := i + ce.batchSize
		if end > len(blocks) {
			end = len(blocks)
		}
		batches = append(batches, batch{start: i, blocks: blocks[i:end]})
	}

	// Execute batches concurrently
	batchResults, err := concurrency.Map(ctx, ce.pool, len(batches), func(i int) ([]model.Embedding, error) {
		return ce.inner.Embed(ctx, batches[i].blocks, opts)
	})
	if err != nil {
		return nil, fmt.Errorf("concurrent embed: %w", err)
	}

	// Merge results
	results := make([]model.Embedding, len(blocks))
	for i, br := range batchResults {
		offset := batches[i].start
		for j, emb := range br {
			emb.BlockIndex = offset + j
			results[offset+j] = emb
		}
	}

	return results, nil
}
