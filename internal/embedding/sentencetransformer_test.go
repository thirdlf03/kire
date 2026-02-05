package embedding_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/thirdlf03/kire/internal/embedding"
	"github.com/thirdlf03/kire/internal/model"
)

func TestSentenceTransformerEmbedder_DimensionsUnsupported(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach server")
	}))
	defer server.Close()

	t.Setenv("SENTENCETRANSFORMER_HOST", server.URL)
	e, _, err := embedding.Create(context.Background(), "sentencetransformer", embedding.ProviderConfig{})
	if err != nil {
		t.Fatal(err)
	}

	blocks := []model.Block{{Kind: model.BlockParagraph, Text: "test"}}
	opts := embedding.EmbedOptions{Dimensions: 256}

	_, err = e.Embed(context.Background(), blocks, opts)
	if err == nil {
		t.Fatal("expected error for non-zero dimensions")
	}
	if !strings.Contains(err.Error(), "custom dimensions not supported") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSentenceTransformerEmbedder_HTTPMock_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/embed" {
			t.Errorf("expected path /embed, got %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		var req struct {
			Texts []string `json:"texts"`
			Model string   `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if req.Model != "test-model" {
			t.Errorf("expected model test-model, got %s", req.Model)
		}
		if len(req.Texts) != 2 {
			t.Errorf("expected 2 texts, got %d", len(req.Texts))
		}

		resp := map[string]any{
			"embeddings": [][]float64{{0.1, 0.2, 0.3}, {0.4, 0.5, 0.6}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	t.Setenv("SENTENCETRANSFORMER_HOST", server.URL)
	e, info, err := embedding.Create(context.Background(), "sentencetransformer", embedding.ProviderConfig{Model: "test-model"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(info, "test-model") {
		t.Errorf("expected info to contain model name, got %q", info)
	}
	if !strings.Contains(info, "sentencetransformer") {
		t.Errorf("expected info to contain provider name, got %q", info)
	}

	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "hello"},
		{Kind: model.BlockParagraph, Text: "world"},
	}
	results, err := e.Embed(context.Background(), blocks, embedding.EmbedOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].BlockIndex != 0 || results[1].BlockIndex != 1 {
		t.Errorf("unexpected block indices: %d, %d", results[0].BlockIndex, results[1].BlockIndex)
	}
	if len(results[0].Vector) != 3 {
		t.Errorf("expected vector length 3, got %d", len(results[0].Vector))
	}
}

func TestResolveSTModel(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"minilm", "all-MiniLM-L6-v2"},
		{"stella", "dunzhang/stella_en_400M_v5"},
		{"e5-large", "intfloat/e5-large-v2"},
		{"custom/my-model", "custom/my-model"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := embedding.ResolveSTModel(tt.input)
			if got != tt.want {
				t.Errorf("ResolveSTModel(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSentenceTransformerEmbedder_HTTPMock_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer server.Close()

	t.Setenv("SENTENCETRANSFORMER_HOST", server.URL)
	e, _, err := embedding.Create(context.Background(), "sentencetransformer", embedding.ProviderConfig{})
	if err != nil {
		t.Fatal(err)
	}

	blocks := []model.Block{{Kind: model.BlockParagraph, Text: "test"}}
	_, err = e.Embed(context.Background(), blocks, embedding.EmbedOptions{})
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "status 500") {
		t.Errorf("expected error to mention status 500, got %v", err)
	}
}

func TestSentenceTransformerEmbedder_HTTPMock_CountMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"embeddings": [][]float64{{0.1, 0.2}}, // only 1 embedding for 2 texts
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	t.Setenv("SENTENCETRANSFORMER_HOST", server.URL)
	e, _, err := embedding.Create(context.Background(), "sentencetransformer", embedding.ProviderConfig{})
	if err != nil {
		t.Fatal(err)
	}

	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "hello"},
		{Kind: model.BlockParagraph, Text: "world"},
	}
	_, err = e.Embed(context.Background(), blocks, embedding.EmbedOptions{})
	if err == nil {
		t.Fatal("expected error for count mismatch")
	}
	if !strings.Contains(err.Error(), "expected 2 embeddings, got 1") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSentenceTransformerEmbedder_HTTPMock_EmptyText(t *testing.T) {
	var receivedTexts []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Texts []string `json:"texts"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		receivedTexts = req.Texts
		resp := map[string]any{
			"embeddings": [][]float64{{0.1, 0.2}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	t.Setenv("SENTENCETRANSFORMER_HOST", server.URL)
	e, _, err := embedding.Create(context.Background(), "sentencetransformer", embedding.ProviderConfig{})
	if err != nil {
		t.Fatal(err)
	}

	blocks := []model.Block{{Kind: model.BlockParagraph, Text: "   "}}
	_, err = e.Embed(context.Background(), blocks, embedding.EmbedOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(receivedTexts) != 1 || receivedTexts[0] != "(empty)" {
		t.Errorf("expected empty text to be replaced with '(empty)', got %v", receivedTexts)
	}
}

func TestSentenceTransformerEmbedder_HTTPMock_Batching(t *testing.T) {
	batchCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		batchCount++
		var req struct {
			Texts []string `json:"texts"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		embeddings := make([][]float64, len(req.Texts))
		for i := range embeddings {
			embeddings[i] = []float64{float64(i) * 0.1, float64(i) * 0.2}
		}

		resp := map[string]any{"embeddings": embeddings}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	t.Setenv("SENTENCETRANSFORMER_HOST", server.URL)
	e, _, err := embedding.Create(context.Background(), "sentencetransformer", embedding.ProviderConfig{BatchSize: 2})
	if err != nil {
		t.Fatal(err)
	}

	blocks := make([]model.Block, 5)
	for i := range blocks {
		blocks[i] = model.Block{Kind: model.BlockParagraph, Text: "text"}
	}

	results, err := e.Embed(context.Background(), blocks, embedding.EmbedOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}
	if batchCount != 3 { // 2 + 2 + 1
		t.Errorf("expected 3 batches, got %d", batchCount)
	}
}

func TestSentenceTransformerEmbedder_InvalidHost(t *testing.T) {
	t.Setenv("SENTENCETRANSFORMER_HOST", "not-a-valid-url://host")
	_, _, err := embedding.Create(context.Background(), "sentencetransformer", embedding.ProviderConfig{})
	if err == nil {
		t.Fatal("expected error for invalid host")
	}
}

func TestSentenceTransformerEmbedder_InvalidScheme(t *testing.T) {
	t.Setenv("SENTENCETRANSFORMER_HOST", "ftp://localhost:8080")
	_, _, err := embedding.Create(context.Background(), "sentencetransformer", embedding.ProviderConfig{})
	if err == nil {
		t.Fatal("expected error for invalid scheme")
	}
	if !strings.Contains(err.Error(), "http or https") {
		t.Errorf("unexpected error: %v", err)
	}
}
