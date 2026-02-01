package pipeline_test

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/thirdlf03/kire/internal/boundary"
	"github.com/thirdlf03/kire/internal/embedding"
	"github.com/thirdlf03/kire/internal/model"
	"github.com/thirdlf03/kire/internal/parser"
	"github.com/thirdlf03/kire/internal/pipeline"
	"github.com/thirdlf03/kire/internal/segment"
	"github.com/thirdlf03/kire/internal/tokenizer"
)

func testdataPath(name string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", name)
}

func TestPipeline_SimpleFile(t *testing.T) {
	source, err := os.ReadFile(testdataPath("simple.md"))
	if err != nil {
		t.Fatal(err)
	}

	cfg := pipeline.Config{
		Source:         source,
		Embedder:       embedding.NewMockEmbedder(128),
		TokenEstimator: tokenizer.NewLocalEstimator(),
		MinTokens:      10,
		MaxTokens:      500,
		Window:         2,
		MinGap:         2,
		ContextFormat:  "none",
	}

	result, err := pipeline.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Blocks) == 0 {
		t.Error("expected blocks")
	}
	if len(result.Segments) == 0 {
		t.Error("expected segments")
	}
	if len(result.Rendered) == 0 {
		t.Error("expected rendered output")
	}

	// Verify all segments have reasonable token counts
	for i, seg := range result.Segments {
		if seg.TokenCount <= 0 {
			t.Errorf("segment %d has 0 tokens", i)
		}
	}
}

func TestPipeline_LosslessNoContext(t *testing.T) {
	source, err := os.ReadFile(testdataPath("simple.md"))
	if err != nil {
		t.Fatal(err)
	}

	cfg := pipeline.Config{
		Source:         source,
		Embedder:       embedding.NewMockEmbedder(128),
		TokenEstimator: tokenizer.NewLocalEstimator(),
		MinTokens:      1,
		MaxTokens:      10000,
		Window:         2,
		ContextFormat:  "none",
	}

	result, err := pipeline.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	// With no context and single segment (high max), rendered should match source
	if len(result.Segments) == 1 {
		if result.Rendered[0] != string(source) {
			t.Errorf("lossless round-trip failed for single segment")
		}
	}
}

func TestPipeline_EmbeddingsInResult(t *testing.T) {
	source, err := os.ReadFile(testdataPath("simple.md"))
	if err != nil {
		t.Fatal(err)
	}

	cfg := pipeline.Config{
		Source:         source,
		Embedder:       embedding.NewMockEmbedder(128),
		TokenEstimator: tokenizer.NewLocalEstimator(),
		MinTokens:      10,
		MaxTokens:      500,
		Window:         2,
		MinGap:         2,
		ContextFormat:  "none",
	}

	result, err := pipeline.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Embeddings) == 0 {
		t.Fatal("expected embeddings in result")
	}
	if len(result.Embeddings) != len(result.Blocks) {
		t.Errorf("embeddings count %d != blocks count %d", len(result.Embeddings), len(result.Blocks))
	}
}

func TestPipeline_TFIDFFallback(t *testing.T) {
	source, err := os.ReadFile(testdataPath("simple.md"))
	if err != nil {
		t.Fatal(err)
	}

	cfg := pipeline.Config{
		Source:         source,
		Embedder:       embedding.NewTFIDFEmbedder(256),
		TokenEstimator: tokenizer.NewLocalEstimator(),
		MinTokens:      10,
		MaxTokens:      500,
		Window:         2,
		MinGap:         2,
		ContextFormat:  "none",
	}

	result, err := pipeline.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Segments) == 0 {
		t.Error("expected segments with TF-IDF fallback")
	}
}

func baseCfg(source []byte) pipeline.Config {
	return pipeline.Config{
		Source:         source,
		SourceName:     "test.md",
		Embedder:       embedding.NewMockEmbedder(128),
		TokenEstimator: tokenizer.NewLocalEstimator(),
		MinTokens:      10,
		MaxTokens:      500,
		Window:         2,
		MinGap:         2,
		ContextFormat:  "none",
	}
}

func TestPipeline_EmptySource(t *testing.T) {
	cfg := baseCfg(nil)
	result, err := pipeline.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Blocks) != 0 || len(result.Segments) != 0 {
		t.Error("expected empty result for nil source")
	}
}

func TestPipeline_ParaSplitPatterns(t *testing.T) {
	source, _ := os.ReadFile(testdataPath("simple.md"))
	cfg := baseCfg(source)
	cfg.ParaSplitPatterns = []string{`^Some`}
	result, err := pipeline.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Blocks) == 0 {
		t.Error("expected blocks")
	}
}

func TestPipeline_ParaSplitPatterns_Invalid(t *testing.T) {
	source, _ := os.ReadFile(testdataPath("simple.md"))
	cfg := baseCfg(source)
	cfg.ParaSplitPatterns = []string{`[invalid`}
	var logBuf bytes.Buffer
	cfg.Logger = log.New(&logBuf, "", 0)
	_, err := pipeline.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
}

func TestPipeline_PseudoHeading(t *testing.T) {
	source := []byte("# Title\n\nIntro paragraph.\n\nSome Label:\nContent here.\n")
	cfg := baseCfg(source)
	cfg.PseudoHeading = parser.PseudoHeadingConfig{
		Enabled:       true,
		ColonSuffixes: true,
		MaxRuneLen:    40,
	}
	result, err := pipeline.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Blocks) == 0 {
		t.Error("expected blocks")
	}
}

func TestPipeline_EnableBoundaryHints(t *testing.T) {
	source, _ := os.ReadFile(testdataPath("simple.md"))
	cfg := baseCfg(source)
	cfg.EnableBoundaryHints = true
	result, err := pipeline.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Segments) == 0 {
		t.Error("expected segments")
	}
}

func TestPipeline_ForceEnforcePatterns(t *testing.T) {
	source, _ := os.ReadFile(testdataPath("simple.md"))
	cfg := baseCfg(source)
	cfg.ForceEnforcePatterns = []string{`^## `}
	result, err := pipeline.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Segments) == 0 {
		t.Error("expected segments")
	}
}

func TestPipeline_SuppressListBoundary(t *testing.T) {
	source, _ := os.ReadFile(testdataPath("simple.md"))
	cfg := baseCfg(source)
	cfg.SuppressListBoundary = true
	result, err := pipeline.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Segments) == 0 {
		t.Error("expected segments")
	}
}

func TestPipeline_AtomicBoundaryProtection(t *testing.T) {
	source, _ := os.ReadFile(testdataPath("simple.md"))
	cfg := baseCfg(source)
	cfg.AtomicBoundaryProtection = true
	result, err := pipeline.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Segments) == 0 {
		t.Error("expected segments")
	}
}

func TestPipeline_SectionLock(t *testing.T) {
	source, _ := os.ReadFile(testdataPath("simple.md"))
	cfg := baseCfg(source)
	cfg.SectionLock = segment.LockConfig{
		Enabled:          true,
		MaxLockBlocks:    10,
		LockAfterHeading: true,
	}
	result, err := pipeline.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Segments) == 0 {
		t.Error("expected segments")
	}
}

func TestPipeline_OverlapLines(t *testing.T) {
	source, _ := os.ReadFile(testdataPath("simple.md"))
	cfg := baseCfg(source)
	cfg.MinTokens = 1
	cfg.MaxTokens = 30
	cfg.OverlapLines = 2
	result, err := pipeline.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Segments) == 0 {
		t.Error("expected segments")
	}
}

func TestPipeline_DebugBoundaryWriter(t *testing.T) {
	source, _ := os.ReadFile(testdataPath("simple.md"))
	cfg := baseCfg(source)
	var buf bytes.Buffer
	cfg.DebugBoundaryWriter = &buf
	_, err := pipeline.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if buf.Len() == 0 {
		t.Error("expected debug output")
	}
}

func TestPipeline_AutoMaxLines(t *testing.T) {
	source, _ := os.ReadFile(testdataPath("simple.md"))
	cfg := baseCfg(source)
	cfg.MaxLines = -1
	result, err := pipeline.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Segments) == 0 {
		t.Error("expected segments")
	}
}

func TestPipeline_CustomLogger(t *testing.T) {
	source, _ := os.ReadFile(testdataPath("simple.md"))
	cfg := baseCfg(source)
	var buf bytes.Buffer
	cfg.Logger = log.New(&buf, "TEST: ", 0)
	_, err := pipeline.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if buf.Len() == 0 {
		t.Error("expected log output")
	}
}

func TestPipeline_HookOnEmbedError(t *testing.T) {
	source, _ := os.ReadFile(testdataPath("simple.md"))
	cfg := baseCfg(source)
	cfg.Hooks = &pipeline.Hooks{
		OnEmbed: func(_ []model.Embedding) error {
			return fmt.Errorf("embed hook error")
		},
	}
	_, err := pipeline.Run(context.Background(), cfg)
	if err == nil || err.Error() != "hook OnEmbed: embed hook error" {
		t.Errorf("expected embed hook error, got %v", err)
	}
}

func TestPipeline_HookOnBoundaryError(t *testing.T) {
	source, _ := os.ReadFile(testdataPath("simple.md"))
	cfg := baseCfg(source)
	cfg.Hooks = &pipeline.Hooks{
		OnBoundary: func(_ boundary.BoundaryResult) error {
			return fmt.Errorf("boundary hook error")
		},
	}
	_, err := pipeline.Run(context.Background(), cfg)
	if err == nil || err.Error() != "hook OnBoundary: boundary hook error" {
		t.Errorf("expected boundary hook error, got %v", err)
	}
}

func TestPipeline_HookOnSegmentError(t *testing.T) {
	source, _ := os.ReadFile(testdataPath("simple.md"))
	cfg := baseCfg(source)
	cfg.Hooks = &pipeline.Hooks{
		OnSegment: func(_ []model.Segment) error {
			return fmt.Errorf("segment hook error")
		},
	}
	_, err := pipeline.Run(context.Background(), cfg)
	if err == nil || err.Error() != "hook OnSegment: segment hook error" {
		t.Errorf("expected segment hook error, got %v", err)
	}
}

func TestPipeline_HookOnRenderError(t *testing.T) {
	source, _ := os.ReadFile(testdataPath("simple.md"))
	cfg := baseCfg(source)
	cfg.Hooks = &pipeline.Hooks{
		OnRender: func(_ int, _ string) error {
			return fmt.Errorf("render hook error")
		},
	}
	_, err := pipeline.Run(context.Background(), cfg)
	if err == nil || err.Error() != "hook OnRender[0]: render hook error" {
		t.Errorf("expected render hook error, got %v", err)
	}
}

func TestPipeline_SplitCount(t *testing.T) {
	source, _ := os.ReadFile(testdataPath("simple.md"))
	cfg := baseCfg(source)
	count := 3
	cfg.SplitCount = &count
	result, err := pipeline.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Segments) == 0 {
		t.Error("expected segments")
	}
}
