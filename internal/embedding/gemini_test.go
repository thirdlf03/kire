package embedding_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/thirdlf03/kire/internal/embedding"
	"github.com/thirdlf03/kire/internal/model"
	"google.golang.org/genai"
)

func TestGeminiEmbedder_MockServer_Single(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Logf("Mock server received: %s %s", r.Method, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"embeddings": []map[string]interface{}{
				{"values": []float64{0.1, 0.2, 0.3}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  "test-key",
		Backend: genai.BackendGeminiAPI,
		HTTPOptions: genai.HTTPOptions{
			BaseURL: server.URL,
		},
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	embedder := embedding.NewGeminiEmbedder(client, embedding.GeminiConfig{
		Model:     "gemini-embedding-001",
		BatchSize: 32,
	})

	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "Hello world"},
	}

	results, err := embedder.Embed(ctx, blocks, embedding.EmbedOptions{
		TaskType: "SEMANTIC_SIMILARITY",
	})
	if err != nil {
		t.Fatalf("embed error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if len(results[0].Vector) != 3 {
		t.Errorf("expected 3-dim vector, got %d", len(results[0].Vector))
	}
}

func TestGeminiEmbedder_MockServer_Batch(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		t.Logf("Mock server received: %s %s", r.Method, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]interface{}{
			"embeddings": []map[string]interface{}{
				{"values": []float64{0.1, 0.2, 0.3}},
				{"values": []float64{0.4, 0.5, 0.6}},
				{"values": []float64{0.7, 0.8, 0.9}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  "test-key",
		Backend: genai.BackendGeminiAPI,
		HTTPOptions: genai.HTTPOptions{
			BaseURL: server.URL,
		},
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	embedder := embedding.NewGeminiEmbedder(client, embedding.GeminiConfig{
		Model:     "gemini-embedding-001",
		BatchSize: 32,
	})

	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "Hello"},
		{Kind: model.BlockParagraph, Text: "World"},
		{Kind: model.BlockParagraph, Text: "Test"},
	}

	results, err := embedder.Embed(ctx, blocks, embedding.EmbedOptions{})
	if err != nil {
		t.Fatalf("embed error: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if callCount != 1 {
		t.Errorf("expected 1 API call for batch, got %d", callCount)
	}
}

func TestGeminiEmbedder_RealAPI(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY not set, skipping real API test")
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	embedder := embedding.NewGeminiEmbedder(client, embedding.GeminiConfig{})
	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "Go is a programming language"},
	}

	results, err := embedder.Embed(ctx, blocks, embedding.EmbedOptions{
		Dimensions: 128,
	})
	if err != nil {
		t.Fatalf("embed error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if len(results[0].Vector) == 0 {
		t.Error("expected non-empty vector")
	}
}
