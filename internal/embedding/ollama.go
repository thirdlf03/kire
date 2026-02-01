package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/thirdlf03/kire/internal/model"
)

const (
	defaultOllamaHost      = "http://localhost:11434"
	defaultOllamaModel     = "nomic-embed-text"
	defaultOllamaBatchSize = 32
)

func init() {
	Register("ollama", func(_ context.Context, cfg ProviderConfig) (Embedder, string, error) {
		host := os.Getenv("OLLAMA_HOST")
		if host == "" {
			host = defaultOllamaHost
		}
		u, err := url.Parse(host)
		if err != nil {
			return nil, "", fmt.Errorf("ollama: invalid OLLAMA_HOST %q: %w", host, err)
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return nil, "", fmt.Errorf("ollama: OLLAMA_HOST must use http or https scheme, got %q", u.Scheme)
		}
		m := cfg.Model
		if m == "" {
			m = defaultOllamaModel
		}
		bs := cfg.BatchSize
		if bs <= 0 {
			bs = defaultOllamaBatchSize
		}

		e := &OllamaEmbedder{
			baseURL:   host,
			model:     m,
			batchSize: bs,
			client:    &http.Client{Timeout: 60 * time.Second},
		}
		return e, fmt.Sprintf("ollama(model=%s,host=%s)", m, host), nil
	})
}

// OllamaEmbedder uses Ollama's local embedding API.
type OllamaEmbedder struct {
	baseURL   string
	model     string
	batchSize int
	client    *http.Client
}

type ollamaEmbedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type ollamaEmbedResponse struct {
	Embeddings [][]float64 `json:"embeddings"`
}

func (o *OllamaEmbedder) Embed(ctx context.Context, blocks []model.Block, opts EmbedOptions) ([]model.Embedding, error) {
	if opts.Dimensions > 0 {
		return nil, fmt.Errorf("ollama: custom dimensions not supported")
	}
	results := make([]model.Embedding, len(blocks))

	for i := 0; i < len(blocks); i += o.batchSize {
		end := i + o.batchSize
		if end > len(blocks) {
			end = len(blocks)
		}
		batch := blocks[i:end]

		embeddings, err := o.embedBatch(ctx, batch)
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

func (o *OllamaEmbedder) embedBatch(ctx context.Context, blocks []model.Block) ([][]float64, error) {
	texts := make([]string, len(blocks))
	for i, b := range blocks {
		text := b.Text
		if strings.TrimSpace(text) == "" {
			text = "(empty)"
		}
		texts[i] = text
	}

	reqBody := ollamaEmbedRequest{
		Model: o.model,
		Input: texts,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := o.baseURL + "/api/embed"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	httpClient := o.client
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("POST %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("POST %s: status %d: %s", url, resp.StatusCode, string(respBody))
	}

	var result ollamaEmbedResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 100<<20)).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(result.Embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}

	return result.Embeddings, nil
}
