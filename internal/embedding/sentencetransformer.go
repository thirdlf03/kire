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
	defaultSTHost      = "http://localhost:8080"
	defaultSTModel     = "all-MiniLM-L6-v2"
	defaultSTBatchSize = 32
)

func init() {
	Register("sentencetransformer", func(_ context.Context, cfg ProviderConfig) (Embedder, string, error) {
		host := os.Getenv("SENTENCETRANSFORMER_HOST")
		if host == "" {
			host = defaultSTHost
		}
		u, err := url.Parse(host)
		if err != nil {
			return nil, "", fmt.Errorf("sentencetransformer: invalid SENTENCETRANSFORMER_HOST %q: %w", host, err)
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return nil, "", fmt.Errorf("sentencetransformer: SENTENCETRANSFORMER_HOST must use http or https scheme, got %q", u.Scheme)
		}
		m := cfg.Model
		if m == "" {
			m = defaultSTModel
		}
		bs := cfg.BatchSize
		if bs <= 0 {
			bs = defaultSTBatchSize
		}

		e := &SentenceTransformerEmbedder{
			baseURL:   host,
			model:     m,
			batchSize: bs,
			client:    &http.Client{Timeout: 60 * time.Second},
		}
		return e, fmt.Sprintf("sentencetransformer(model=%s,host=%s)", m, host), nil
	})
}

// SentenceTransformerEmbedder uses a SentenceTransformer HTTP server for embeddings.
type SentenceTransformerEmbedder struct {
	baseURL   string
	model     string
	batchSize int
	client    *http.Client
}

type stEmbedRequest struct {
	Texts []string `json:"texts"`
	Model string   `json:"model"`
}

type stEmbedResponse struct {
	Embeddings [][]float64 `json:"embeddings"`
}

func (s *SentenceTransformerEmbedder) Embed(ctx context.Context, blocks []model.Block, opts EmbedOptions) ([]model.Embedding, error) {
	if opts.Dimensions > 0 {
		return nil, fmt.Errorf("sentencetransformer: custom dimensions not supported")
	}
	results := make([]model.Embedding, len(blocks))

	for i := 0; i < len(blocks); i += s.batchSize {
		end := i + s.batchSize
		if end > len(blocks) {
			end = len(blocks)
		}
		batch := blocks[i:end]

		embeddings, err := s.embedBatch(ctx, batch)
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

func (s *SentenceTransformerEmbedder) embedBatch(ctx context.Context, blocks []model.Block) ([][]float64, error) {
	texts := make([]string, len(blocks))
	for i, b := range blocks {
		text := b.Text
		if strings.TrimSpace(text) == "" {
			text = "(empty)"
		}
		texts[i] = text
	}

	reqBody := stEmbedRequest{
		Texts: texts,
		Model: s.model,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	endpoint := s.baseURL + "/embed"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	httpClient := s.client
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("POST %s: %w", endpoint, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("POST %s: status %d: %s", endpoint, resp.StatusCode, string(respBody))
	}

	var result stEmbedResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 100<<20)).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(result.Embeddings) != len(texts) {
		return nil, fmt.Errorf("expected %d embeddings, got %d", len(texts), len(result.Embeddings))
	}

	return result.Embeddings, nil
}
