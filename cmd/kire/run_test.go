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
	*flReport = false
	*flNoIndex = false
	*flAgentMetadata = false
	// Reset string flags
	*flOutDir = "docs"
	*flPrefix = ""
	*flEmbedder = "auto"
	*flJSONL = ""
	*flDagJSON = ""
	*flDagDOT = ""
	*flStateFile = ""
	*flDebugBoundary = ""
	*flForceBoundary = ""
	*flParaSplitPattern = ""
	*flLogFormat = "text"
	*flContextFormat = "comment"
	// Reset numeric flags
	*flMinTokens = 300
	*flMaxTokens = 3000
	*flMaxLines = -1
	*flMinGap = 3
	*flWindow = 3
	*flOverlapLines = 0
	*flSplitCount = 0
	*flThreshold = -1
	*flEmbedConcurrency = 4
	*flBatchSize = 32
	*flEmbedQPS = 0
	// Reset negatable flags
	*flBoundaryHints = true
	*flSectionLock = true
	*flLockAfterHeading = true
	*flSuppressListBoundary = true
	*flAtomicBoundary = true
	*flPseudoHeading = true
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

func TestBuildLockConfig(t *testing.T) {
	cfg := buildLockConfig(true, true)
	if !cfg.Enabled {
		t.Error("expected Enabled=true")
	}
	if !cfg.LockAfterHeading {
		t.Error("expected LockAfterHeading=true")
	}

	cfg2 := buildLockConfig(false, false)
	if cfg2.Enabled {
		t.Error("expected Enabled=false")
	}
	if cfg2.LockAfterHeading {
		t.Error("expected LockAfterHeading=false")
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
	// Reset global flag state
	resetFlags()
	rootCmd.SetArgs([]string{})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for no input")
	}
	if !strings.Contains(err.Error(), "input file is required") {
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

func TestRunSplitter_DryRun(t *testing.T) {
	// Create a temp input file
	dir := t.TempDir()
	inFile := filepath.Join(dir, "test.md")
	if err := os.WriteFile(inFile, []byte("# Title\n\nSome content.\n"), 0644); err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(dir, "output")

	resetFlags()
	rootCmd.SetArgs([]string{
		"--dry-run",
		"--embedder", "mock",
		"--out", outDir,
		"--no-boundary-hints",
		"--no-section-lock",
		"--no-suppress-list-boundary",
		"--no-atomic-boundary",
		"--no-pseudo-heading",
		inFile,
	})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSplitter_DryRunJSON(t *testing.T) {
	dir := t.TempDir()
	inFile := filepath.Join(dir, "test.md")
	if err := os.WriteFile(inFile, []byte("# Title\n\nSome content.\n"), 0644); err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(dir, "output")

	resetFlags()
	rootCmd.SetArgs([]string{
		"--dry-run",
		"--json",
		"--embedder", "mock",
		"--out", outDir,
		inFile,
	})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSplitter_WriteFiles(t *testing.T) {
	dir := t.TempDir()
	inFile := filepath.Join(dir, "test.md")
	if err := os.WriteFile(inFile, []byte("# Title\n\nSome content here.\n\n## Section Two\n\nMore content.\n"), 0644); err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(dir, "output")

	resetFlags()
	rootCmd.SetArgs([]string{
		"--embedder", "mock",
		"--out", outDir,
		"--force",
		"--context-format", "none",
		inFile,
	})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check output directory was created
	if _, err := os.Stat(filepath.Join(outDir, "test")); os.IsNotExist(err) {
		t.Error("expected output directory to be created")
	}
}

func TestRunSplitter_WithPrefix(t *testing.T) {
	dir := t.TempDir()
	inFile := filepath.Join(dir, "test.md")
	if err := os.WriteFile(inFile, []byte("# Title\n\nContent.\n"), 0644); err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(dir, "output")

	resetFlags()
	rootCmd.SetArgs([]string{
		"--embedder", "mock",
		"--out", outDir,
		"--prefix", "part-",
		"--force",
		inFile,
	})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSplitter_WithJSONL(t *testing.T) {
	dir := t.TempDir()
	inFile := filepath.Join(dir, "test.md")
	if err := os.WriteFile(inFile, []byte("# Title\n\nContent.\n"), 0644); err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(dir, "output")

	resetFlags()
	rootCmd.SetArgs([]string{
		"--embedder", "mock",
		"--out", outDir,
		"--force",
		"--jsonl",
		inFile,
	})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSplitter_WithReport(t *testing.T) {
	dir := t.TempDir()
	inFile := filepath.Join(dir, "test.md")
	if err := os.WriteFile(inFile, []byte("# Title\n\nContent.\n"), 0644); err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(dir, "output")

	resetFlags()
	rootCmd.SetArgs([]string{
		"--embedder", "mock",
		"--out", outDir,
		"--force",
		"--report",
		inFile,
	})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSplitter_WithDAG(t *testing.T) {
	dir := t.TempDir()
	inFile := filepath.Join(dir, "test.md")
	if err := os.WriteFile(inFile, []byte("# Title\n\nContent.\n"), 0644); err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(dir, "output")
	dagJSON := filepath.Join(dir, "dag.json")
	dagDOT := filepath.Join(dir, "dag.dot")

	resetFlags()
	rootCmd.SetArgs([]string{
		"--embedder", "mock",
		"--out", outDir,
		"--force",
		"--dag-json", dagJSON,
		"--dag-dot", dagDOT,
		inFile,
	})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSplitter_WithStateFile(t *testing.T) {
	dir := t.TempDir()
	inFile := filepath.Join(dir, "test.md")
	if err := os.WriteFile(inFile, []byte("# Title\n\nContent.\n"), 0644); err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(dir, "output")
	stateFile := filepath.Join(dir, "state.json")

	// First run: creates state file
	resetFlags()
	rootCmd.SetArgs([]string{
		"--embedder", "mock",
		"--out", outDir,
		"--force",
		"--state-file", stateFile,
		inFile,
	})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("first run error: %v", err)
	}

	// Second run: should detect no changes
	resetFlags()
	rootCmd.SetArgs([]string{
		"--embedder", "mock",
		"--out", outDir,
		"--force",
		"--state-file", stateFile,
		inFile,
	})
	err = rootCmd.Execute()
	if err != nil {
		t.Fatalf("second run error: %v", err)
	}
}

func TestRunSplitter_WithAgentMetadata(t *testing.T) {
	dir := t.TempDir()
	inFile := filepath.Join(dir, "test.md")
	if err := os.WriteFile(inFile, []byte("# Title\n\nContent.\n"), 0644); err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(dir, "output")

	resetFlags()
	rootCmd.SetArgs([]string{
		"--embedder", "mock",
		"--out", outDir,
		"--force",
		"--jsonl",
		"--agent-metadata",
		"--json",
		inFile,
	})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSplitter_PositionalArgs(t *testing.T) {
	dir := t.TempDir()
	inFile := filepath.Join(dir, "test.md")
	if err := os.WriteFile(inFile, []byte("# Title\n\nContent.\n"), 0644); err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(dir, "output")

	resetFlags()
	rootCmd.SetArgs([]string{
		"--embedder", "mock",
		"--dry-run",
		inFile,
		outDir,
	})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSplitter_NonexistentInput(t *testing.T) {
	resetFlags()
	rootCmd.SetArgs([]string{"/nonexistent/file.md"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent input")
	}
}

func TestRunSplitter_NoIndex(t *testing.T) {
	dir := t.TempDir()
	inFile := filepath.Join(dir, "test.md")
	if err := os.WriteFile(inFile, []byte("# Title\n\nContent.\n"), 0644); err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(dir, "output")

	resetFlags()
	rootCmd.SetArgs([]string{
		"--embedder", "mock",
		"--out", outDir,
		"--force",
		"--no-index",
		inFile,
	})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSplitter_MaxLines(t *testing.T) {
	dir := t.TempDir()
	inFile := filepath.Join(dir, "test.md")
	if err := os.WriteFile(inFile, []byte("# Title\n\nContent.\n"), 0644); err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(dir, "output")

	resetFlags()
	rootCmd.SetArgs([]string{
		"--embedder", "mock",
		"--out", outDir,
		"--force",
		"--dry-run",
		"--max-lines", "50",
		inFile,
	})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSplitter_WithInFlag(t *testing.T) {
	dir := t.TempDir()
	inFile := filepath.Join(dir, "test.md")
	if err := os.WriteFile(inFile, []byte("# Title\n\nContent.\n"), 0644); err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(dir, "output")

	resetFlags()
	rootCmd.SetArgs([]string{
		"--in", inFile,
		"--embedder", "mock",
		"--out", outDir,
		"--force",
		"--dry-run",
		"--json",
	})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSplitter_MultipleInFiles(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "a.md")
	f2 := filepath.Join(dir, "b.md")
	os.WriteFile(f1, []byte("# A\n\nContent A.\n"), 0644)
	os.WriteFile(f2, []byte("# B\n\nContent B.\n"), 0644)
	outDir := filepath.Join(dir, "output")

	resetFlags()
	rootCmd.SetArgs([]string{
		"--in", f1,
		"--in", f2,
		"--embedder", "mock",
		"--out", outDir,
		"--force",
		"--dag-json", filepath.Join(dir, "dag.json"),
		"--dag-dot", filepath.Join(dir, "dag.dot"),
	})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSplitter_DebugBoundaryFile(t *testing.T) {
	dir := t.TempDir()
	inFile := filepath.Join(dir, "test.md")
	if err := os.WriteFile(inFile, []byte("# Title\n\nContent.\n"), 0644); err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(dir, "output")
	debugFile := filepath.Join(dir, "debug.tsv")

	resetFlags()
	rootCmd.SetArgs([]string{
		"--embedder", "mock",
		"--out", outDir,
		"--force",
		"--dry-run",
		"--debug-boundary", debugFile,
		inFile,
	})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSplitter_ThresholdExplicit(t *testing.T) {
	dir := t.TempDir()
	inFile := filepath.Join(dir, "test.md")
	if err := os.WriteFile(inFile, []byte("# Title\n\nContent.\n"), 0644); err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(dir, "output")

	resetFlags()
	rootCmd.SetArgs([]string{
		"--embedder", "mock",
		"--out", outDir,
		"--force",
		"--dry-run",
		"--threshold", "0.5",
		inFile,
	})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSplitter_SplitCount(t *testing.T) {
	dir := t.TempDir()
	inFile := filepath.Join(dir, "test.md")
	if err := os.WriteFile(inFile, []byte("# Title\n\nContent.\n\n## Section\n\nMore.\n"), 0644); err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(dir, "output")

	resetFlags()
	rootCmd.SetArgs([]string{
		"--embedder", "mock",
		"--out", outDir,
		"--force",
		"--dry-run",
		"--split-count", "2",
		inFile,
	})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSplitter_JSONLCustomPath(t *testing.T) {
	dir := t.TempDir()
	inFile := filepath.Join(dir, "test.md")
	if err := os.WriteFile(inFile, []byte("# Title\n\nContent.\n"), 0644); err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(dir, "output")
	jsonlPath := filepath.Join(dir, "custom.jsonl")

	resetFlags()
	rootCmd.SetArgs([]string{
		"--embedder", "mock",
		"--out", outDir,
		"--force",
		"--jsonl=" + jsonlPath,
		"--in", inFile,
	})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(jsonlPath); os.IsNotExist(err) {
		t.Error("expected custom JSONL file to be created")
	}
}

func TestRunSplitter_PseudoHeadingPrefix(t *testing.T) {
	dir := t.TempDir()
	inFile := filepath.Join(dir, "test.md")
	if err := os.WriteFile(inFile, []byte("# Title\n\nStep 1:\nDo something.\n"), 0644); err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(dir, "output")

	resetFlags()
	rootCmd.SetArgs([]string{
		"--embedder", "mock",
		"--out", outDir,
		"--force",
		"--dry-run",
		"--pseudo-heading-prefix", "^Step",
		inFile,
	})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
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
	os.MkdirAll(filepath.Join(outDir, "test"), 0755)

	// Force non-terminal
	origIsTerminal := isTerminal
	isTerminal = func() bool { return false }
	defer func() { isTerminal = origIsTerminal }()

	resetFlags()
	rootCmd.SetArgs([]string{
		"--embedder", "mock",
		"--out", outDir,
		inFile,
	})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for existing output without --force")
	}
}
