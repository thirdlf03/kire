package embedding

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/thirdlf03/kire/internal/model"
	"google.golang.org/genai"
)

const (
	DefaultModel           = "gemini-embedding-001"
	DefaultTaskType        = "SEMANTIC_SIMILARITY"
	defaultGeminiBatchSize = 32
)

func init() {
	Register("gemini", func(ctx context.Context, cfg ProviderConfig) (Embedder, string, error) {
		apiKey := os.Getenv("GEMINI_API_KEY")
		if apiKey == "" {
			return nil, "", fmt.Errorf("gemini requires GEMINI_API_KEY")
		}
		client, err := genai.NewClient(ctx, &genai.ClientConfig{
			APIKey:  apiKey,
			Backend: genai.BackendGeminiAPI,
		})
		if err != nil {
			return nil, "", fmt.Errorf("create Gemini client: %w", err)
		}
		gcfg := GeminiConfig(cfg)
		e := NewGeminiEmbedder(client, gcfg)
		model := e.model
		batch := e.batchSize
		return e, fmt.Sprintf("gemini(model=%s,batch=%d)", model, batch), nil
	})
}

// GeminiEmbedder uses Google's Gemini API for embeddings.
type GeminiEmbedder struct {
	client    *genai.Client
	model     string
	batchSize int
}

// GeminiConfig configures the Gemini embedder.
type GeminiConfig struct {
	Model     string
	BatchSize int
}

func NewGeminiEmbedder(client *genai.Client, cfg GeminiConfig) *GeminiEmbedder {
	m := cfg.Model
	if m == "" {
		m = DefaultModel
	}
	bs := cfg.BatchSize
	if bs <= 0 {
		bs = defaultGeminiBatchSize
	}
	return &GeminiEmbedder{
		client:    client,
		model:     m,
		batchSize: bs,
	}
}

func (g *GeminiEmbedder) Embed(ctx context.Context, blocks []model.Block, opts EmbedOptions) ([]model.Embedding, error) {
	taskType := opts.TaskType
	if taskType == "" {
		taskType = DefaultTaskType
	}

	results := make([]model.Embedding, len(blocks))

	for i := 0; i < len(blocks); i += g.batchSize {
		end := i + g.batchSize
		if end > len(blocks) {
			end = len(blocks)
		}
		batch := blocks[i:end]

		embeddings, err := g.embedBatch(ctx, batch, taskType, opts.Dimensions)
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

func (g *GeminiEmbedder) embedBatch(ctx context.Context, blocks []model.Block, taskType string, dims int) ([][]float64, error) {
	contents := make([]*genai.Content, len(blocks))
	for i, b := range blocks {
		text := b.Text
		if strings.TrimSpace(text) == "" {
			text = "(empty)"
		}
		contents[i] = genai.NewContentFromText(text, "user")
	}

	config := &genai.EmbedContentConfig{
		TaskType: taskType,
	}
	if dims > 0 {
		d := int32(dims)
		config.OutputDimensionality = &d
	}

	resp, err := g.client.Models.EmbedContent(ctx, g.model, contents, config)
	if err != nil {
		return nil, fmt.Errorf("embedContent: %w", err)
	}

	if len(resp.Embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}

	results := make([][]float64, len(resp.Embeddings))
	for i, emb := range resp.Embeddings {
		results[i] = float32ToFloat64(emb.Values)
	}
	return results, nil
}

func float32ToFloat64(f32 []float32) []float64 {
	f64 := make([]float64, len(f32))
	for i, v := range f32 {
		f64[i] = float64(v)
	}
	return f64
}
