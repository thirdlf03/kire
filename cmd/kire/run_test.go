package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// resetFlags resets cobra flag values that may persist between tests.
func resetFlags() {
	// Reset key boolean flags to defaults
	*flShowVersion = false
	*flDryRun = false
	*flJSON = false
	*flForce = false
	*flDebug = false
	*flQuiet = false
	*flNoIndex = false
	*flAgentMetadata = false
	*flLLMRefine = false
	// Reset string flags
	*flOutDir = "docs"
	*flPrefix = ""
	*flEmbedder = "auto"
	*flJSONL = ""
	*flDagJSON = ""
	*flDagDOT = ""
	*flStateFile = ""
	*flLogFormat = "text"
	*flContextFormat = "comment"
	// Reset numeric flags
	*flOverlapLines = 0
	*flEmbedConcurrency = 4
	*flBatchSize = 32
	*flEmbedQPS = 0
	// Reset InPaths
	*flInPaths = nil
}

func TestSplitCSV(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{"a,b,c", []string{"a", "b", "c"}},
		{" a , b , c ", []string{"a", "b", "c"}},
		{",,,", nil},
		{"single", []string{"single"}},
		{" , a , , b , ", []string{"a", "b"}},
	}
	for _, tt := range tests {
		got := splitCSV(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("splitCSV(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("splitCSV(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestBuildEmbedder_Mock(t *testing.T) {
	embedder, info, name, closer, err := buildEmbedder(context.Background(), "mock", "", 32, 1, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if embedder == nil {
		t.Fatal("expected non-nil embedder")
	}
	if !strings.Contains(info, "mock") {
		t.Errorf("expected info to contain 'mock', got %q", info)
	}
	if name != "mock" {
		t.Errorf("expected name='mock', got %q", name)
	}
	if closer != nil {
		t.Error("expected nil closer for mock without cache")
	}
}

func TestBuildEmbedder_TFIDF(t *testing.T) {
	embedder, info, name, _, err := buildEmbedder(context.Background(), "tfidf", "", 32, 1, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if embedder == nil {
		t.Fatal("expected non-nil embedder")
	}
	if !strings.Contains(info, "tfidf") {
		t.Errorf("expected info to contain 'tfidf', got %q", info)
	}
	if name != "tfidf" {
		t.Errorf("expected name='tfidf', got %q", name)
	}
}

func TestBuildEmbedder_Auto_NoAPIKey(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	embedder, info, name, _, err := buildEmbedder(context.Background(), "auto", "", 32, 1, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if embedder == nil {
		t.Fatal("expected non-nil embedder")
	}
	if !strings.Contains(info, "tfidf") {
		t.Errorf("expected tfidf fallback, got %q", info)
	}
	if name != "tfidf" {
		t.Errorf("expected name='tfidf', got %q", name)
	}
}

func TestBuildEmbedder_Unknown(t *testing.T) {
	_, _, _, _, err := buildEmbedder(context.Background(), "nonexistent", "", 32, 1, 0, "")
	if err == nil {
		t.Fatal("expected error for unknown embedder")
	}
}

func TestBuildEmbedder_WithCache(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "test.cache.json")
	embedder, info, name, closer, err := buildEmbedder(context.Background(), "mock", "", 32, 1, 0, cachePath)
	if err != nil {
		t.Fatal(err)
	}
	if embedder == nil {
		t.Fatal("expected non-nil embedder")
	}
	if closer == nil {
		t.Fatal("expected non-nil closer with cache")
	}
	if !strings.Contains(info, "cache=") {
		t.Errorf("expected info to contain 'cache=', got %q", info)
	}
	if name != "mock" {
		t.Errorf("expected name='mock', got %q", name)
	}
	_ = closer.Close()
}

func TestBuildEmbedder_WithConcurrency(t *testing.T) {
	embedder, info, name, _, err := buildEmbedder(context.Background(), "mock", "", 32, 4, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if embedder == nil {
		t.Fatal("expected non-nil embedder")
	}
	if !strings.Contains(info, "concurrency=4") {
		t.Errorf("expected info to contain 'concurrency=4', got %q", info)
	}
	if name != "mock" {
		t.Errorf("expected name='mock', got %q", name)
	}
}

func TestBuildEmbedder_WithConcurrencyAndQPS(t *testing.T) {
	_, info, name, _, err := buildEmbedder(context.Background(), "mock", "", 32, 4, 10.0, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(info, "qps=") {
		t.Errorf("expected info to contain 'qps=', got %q", info)
	}
	if name != "mock" {
		t.Errorf("expected name='mock', got %q", name)
	}
}

func TestRunSplitter_NoInput(t *testing.T) {
	resetFlags()
	rootCmd.SetArgs([]string{})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for no input")
	}
	// Either "input file is required" or "GEMINI_API_KEY is required"
	errStr := err.Error()
	if !strings.Contains(errStr, "input file is required") && !strings.Contains(errStr, "GEMINI_API_KEY") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunSplitter_Version(t *testing.T) {
	resetFlags()
	rootCmd.SetArgs([]string{"--version"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSplitter_NoAPIKey(t *testing.T) {
	dir := t.TempDir()
	inFile := filepath.Join(dir, "test.md")
	if err := os.WriteFile(inFile, []byte("# Title\n\nContent.\n"), 0644); err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(dir, "output")

	t.Setenv("GEMINI_API_KEY", "")
	resetFlags()
	rootCmd.SetArgs([]string{
		"--dry-run",
		"--out", outDir,
		inFile,
	})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
	if !strings.Contains(err.Error(), "GEMINI_API_KEY") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunSplitter_NonexistentInput(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "test-key")
	resetFlags()
	rootCmd.SetArgs([]string{"/nonexistent/file.md"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent input")
	}
}

func TestRunSplitter_ExistingOutputNoForce_NonInteractive(t *testing.T) {
	dir := t.TempDir()
	inFile := filepath.Join(dir, "test.md")
	if err := os.WriteFile(inFile, []byte("# Title\n\nContent.\n"), 0644); err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(dir, "output")
	// Create the output subdir ahead of time
	if err := os.MkdirAll(filepath.Join(outDir, "test"), 0755); err != nil {
		t.Fatal(err)
	}

	// Force non-terminal
	origIsTerminal := isTerminal
	isTerminal = func() bool { return false }
	defer func() { isTerminal = origIsTerminal }()

	t.Setenv("GEMINI_API_KEY", "test-key")
	resetFlags()
	rootCmd.SetArgs([]string{
		"--out", outDir,
		inFile,
	})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for existing output without --force")
	}
}
