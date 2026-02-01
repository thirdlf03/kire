package embedding

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/thirdlf03/kire/internal/model"
)

const (
	defaultOpenAIModel     = "text-embedding-3-large"
	defaultOpenAIBatchSize = 2048
)

func init() {
	Register("openai", func(ctx context.Context, cfg ProviderConfig) (Embedder, string, error) {
		apiKey := os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			return nil, "", fmt.Errorf("openai requires OPENAI_API_KEY")
		}
		client := openai.NewClient()

		m := cfg.Model
		if m == "" {
			m = defaultOpenAIModel
		}
		bs := cfg.BatchSize
		if bs <= 0 {
			bs = defaultOpenAIBatchSize
		}

		e := &OpenAIEmbedder{
			client:    client,
			model:     m,
			batchSize: bs,
		}
		return e, fmt.Sprintf("openai(model=%s,batch=%d)", m, bs), nil
	})
}

// OpenAIEmbedder uses OpenAI's Embeddings API for embeddings.
type OpenAIEmbedder struct {
	client    openai.Client
	model     string
	batchSize int
}

func (o *OpenAIEmbedder) Embed(ctx context.Context, blocks []model.Block, opts EmbedOptions) ([]model.Embedding, error) {
	results := make([]model.Embedding, len(blocks))

	for i := 0; i < len(blocks); i += o.batchSize {
		end := i + o.batchSize
		if end > len(blocks) {
			end = len(blocks)
		}
		batch := blocks[i:end]

		embeddings, err := o.embedBatch(ctx, batch, opts.Dimensions)
		if err != nil {
			return nil, fmt.Errorf("embed batch [%d:%d]: %w", i, end, err)
		}

		for j, emb := range embeddings {
			results[i+j] = model.Embedding{
				BlockIndex: i + j,
				Vector:     emb,
			}
		}
	}

	return results, nil
}

func (o *OpenAIEmbedder) embedBatch(ctx context.Context, blocks []model.Block, dims int) ([][]float64, error) {
	texts := make([]string, len(blocks))
	for i, b := range blocks {
		text := b.Text
		if strings.TrimSpace(text) == "" {
			text = "(empty)"
		}
		texts[i] = text
	}

	params := openai.EmbeddingNewParams{
		Input: openai.EmbeddingNewParamsInputUnion{
			OfArrayOfStrings: texts,
		},
		Model: openai.EmbeddingModel(o.model),
	}
	if dims > 0 {
		params.Dimensions = openai.Int(int64(dims))
	}

	resp, err := o.client.Embeddings.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("embeddings.new: %w", err)
	}

	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}

	results := make([][]float64, len(resp.Data))
	for i, d := range resp.Data {
		results[i] = d.Embedding
	}
	return results, nil
}
