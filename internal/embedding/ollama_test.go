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

func TestOllamaEmbedder_DimensionsUnsupported(t *testing.T) {
	e := &embedding.OllamaEmbedder{}
	blocks := []model.Block{{Kind: model.BlockParagraph, Text: "test"}}
	opts := embedding.EmbedOptions{Dimensions: 256}

	_, err := e.Embed(context.Background(), blocks, opts)
	if err == nil {
		t.Fatal("expected error for non-zero dimensions")
	}
	if err.Error() != "ollama: custom dimensions not supported" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestOllamaEmbedder_HTTPMock_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"embeddings": [][]float64{{0.1, 0.2, 0.3}, {0.4, 0.5, 0.6}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	t.Setenv("OLLAMA_HOST", server.URL)
	e, info, err := embedding.Create(context.Background(), "ollama", embedding.ProviderConfig{Model: "test-model"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(info, "test-model") {
		t.Errorf("expected info to contain model name, got %q", info)
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
}

func TestOllamaEmbedder_HTTPMock_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer server.Close()

	t.Setenv("OLLAMA_HOST", server.URL)
	e, _, err := embedding.Create(context.Background(), "ollama", embedding.ProviderConfig{})
	if err != nil {
		t.Fatal(err)
	}

	blocks := []model.Block{{Kind: model.BlockParagraph, Text: "test"}}
	_, err = e.Embed(context.Background(), blocks, embedding.EmbedOptions{})
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestOllamaEmbedder_HTTPMock_EmptyEmbeddings(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{"embeddings": [][]float64{}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	t.Setenv("OLLAMA_HOST", server.URL)
	e, _, err := embedding.Create(context.Background(), "ollama", embedding.ProviderConfig{})
	if err != nil {
		t.Fatal(err)
	}

	blocks := []model.Block{{Kind: model.BlockParagraph, Text: "test"}}
	_, err = e.Embed(context.Background(), blocks, embedding.EmbedOptions{})
	if err == nil {
		t.Fatal("expected error for empty embeddings")
	}
}

func TestOllamaEmbedder_HTTPMock_EmptyText(t *testing.T) {
	var receivedTexts []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Input []string `json:"input"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		receivedTexts = req.Input
		resp := map[string]any{
			"embeddings": [][]float64{{0.1, 0.2}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	t.Setenv("OLLAMA_HOST", server.URL)
	e, _, err := embedding.Create(context.Background(), "ollama", embedding.ProviderConfig{})
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
