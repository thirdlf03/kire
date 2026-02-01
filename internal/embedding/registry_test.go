package embedding

import (
	"context"
	"strings"
	"testing"

	"github.com/thirdlf03/kire/internal/model"
)

func TestCreate_UnregisteredProvider(t *testing.T) {
	_, _, err := Create(context.Background(), "nonexistent", ProviderConfig{})
	if err == nil {
		t.Fatal("expected error for unregistered provider, got nil")
	}
}

func TestCreate_Mock(t *testing.T) {
	embedder, info, err := Create(context.Background(), "mock", ProviderConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if embedder == nil {
		t.Fatal("expected non-nil embedder")
	}
	if info == "" {
		t.Fatal("expected non-empty info string")
	}
}

func TestCreate_TFIDF(t *testing.T) {
	embedder, info, err := Create(context.Background(), "tfidf", ProviderConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if embedder == nil {
		t.Fatal("expected non-nil embedder")
	}
	if info == "" {
		t.Fatal("expected non-empty info string")
	}
}

func TestCreate_Gemini_NoAPIKey(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	// gemini must be a registered provider
	found := false
	for _, name := range List() {
		if name == "gemini" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected gemini to be registered")
	}
	_, _, err := Create(context.Background(), "gemini", ProviderConfig{})
	if err == nil {
		t.Fatal("expected error when GEMINI_API_KEY is not set")
	}
}

func TestList_Sorted(t *testing.T) {
	names := List()
	if len(names) < 5 {
		t.Fatalf("expected at least 5 providers, got %d: %v", len(names), names)
	}
	// Check sorted order
	for i := 1; i < len(names); i++ {
		if names[i-1] >= names[i] {
			t.Fatalf("List() not sorted: %v", names)
		}
	}
	// Check that known providers are present
	expected := map[string]bool{"gemini": false, "mock": false, "ollama": false, "openai": false, "tfidf": false}
	for _, name := range names {
		if _, ok := expected[name]; ok {
			expected[name] = true
		}
	}
	for name, found := range expected {
		if !found {
			t.Errorf("expected provider %q in List()", name)
		}
	}
}

func TestCreate_OpenAI_NoAPIKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	_, _, err := Create(context.Background(), "openai", ProviderConfig{})
	if err == nil {
		t.Fatal("expected error when OPENAI_API_KEY is not set")
	}
}

func TestCreate_OpenAI_WithAPIKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key-for-unit-test")
	embedder, info, err := Create(context.Background(), "openai", ProviderConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if embedder == nil {
		t.Fatal("expected non-nil embedder")
	}
	if info == "" {
		t.Fatal("expected non-empty info string")
	}
	if !strings.Contains(info, "openai") {
		t.Errorf("info should contain 'openai', got: %s", info)
	}
	if !strings.Contains(info, "text-embedding-3-large") {
		t.Errorf("info should contain default model name, got: %s", info)
	}
}

func TestCreate_OpenAI_CustomModel(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key-for-unit-test")
	embedder, info, err := Create(context.Background(), "openai", ProviderConfig{
		Model:     "text-embedding-3-large",
		BatchSize: 512,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if embedder == nil {
		t.Fatal("expected non-nil embedder")
	}
	if !strings.Contains(info, "text-embedding-3-large") {
		t.Errorf("info should contain custom model name, got: %s", info)
	}
	if !strings.Contains(info, "512") {
		t.Errorf("info should contain custom batch size, got: %s", info)
	}
}

func TestCreate_Ollama_AlwaysSucceeds(t *testing.T) {
	embedder, info, err := Create(context.Background(), "ollama", ProviderConfig{})
	if err != nil {
		t.Fatalf("ollama factory should always succeed, got error: %v", err)
	}
	if embedder == nil {
		t.Fatal("expected non-nil embedder")
	}
	if info == "" {
		t.Fatal("expected non-empty info string")
	}
	if !strings.Contains(info, "ollama") {
		t.Errorf("info should contain 'ollama', got: %s", info)
	}
	if !strings.Contains(info, "nomic-embed-text") {
		t.Errorf("info should contain default model name, got: %s", info)
	}
}

func TestCreate_Ollama_CustomConfig(t *testing.T) {
	t.Setenv("OLLAMA_HOST", "http://custom-host:11434")
	embedder, info, err := Create(context.Background(), "ollama", ProviderConfig{
		Model:     "mxbai-embed-large",
		BatchSize: 64,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if embedder == nil {
		t.Fatal("expected non-nil embedder")
	}
	if !strings.Contains(info, "mxbai-embed-large") {
		t.Errorf("info should contain custom model name, got: %s", info)
	}
	if !strings.Contains(info, "custom-host") {
		t.Errorf("info should contain custom host, got: %s", info)
	}
}

func TestOllamaEmbedder_InvalidHost(t *testing.T) {
	e := &OllamaEmbedder{
		baseURL:   "http://invalid-host-that-does-not-exist:99999",
		model:     "nomic-embed-text",
		batchSize: 32,
	}
	blocks := []model.Block{{Text: "test"}}
	_, err := e.Embed(context.Background(), blocks, EmbedOptions{})
	if err == nil {
		t.Fatal("expected error when connecting to invalid host")
	}
}

func TestRegister_Duplicate_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on duplicate Register")
		}
	}()
	Register("mock", func(_ context.Context, _ ProviderConfig) (Embedder, string, error) {
		return nil, "", nil
	})
}
