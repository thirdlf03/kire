package embedding

import (
	"context"

	"github.com/thirdlf03/kire/internal/model"
)

// EmbedOptions holds options for embedding generation.
type EmbedOptions struct {
	TaskType   string
	Dimensions int
}

// Embedder generates embeddings for blocks.
type Embedder interface {
	Embed(ctx context.Context, blocks []model.Block, opts EmbedOptions) ([]model.Embedding, error)
}
