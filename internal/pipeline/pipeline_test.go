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
	"github.com/thirdlf03/kire/internal/pipeline"
	"github.com/thirdlf03/kire/internal/tokenizer"
)

func testdataPath(name string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", name)
}

// stubBoundaryDetector returns fixed boundaries for testing.
type stubBoundaryDetector struct {
	boundaries []int
}

func (s *stubBoundaryDetector) DetectBoundaries(_ context.Context, blocks []model.Block) (boundary.BoundaryResult, error) {
	return boundary.BoundaryResult{
		Boundaries:  s.boundaries,
		DepthScores: make([]float64, max(len(blocks)-1, 0)),
	}, nil
}

func baseCfg(source []byte) pipeline.Config {
	return pipeline.Config{
		Source:         source,
		SourceName:     "test.md",
		TokenEstimator: tokenizer.NewLocalEstimator(),
		BoundaryDetector: &stubBoundaryDetector{
			boundaries: nil, // no boundaries → single segment
		},
		ContextFormat: "none",
	}
}

func TestPipeline_SimpleFile(t *testing.T) {
	source, err := os.ReadFile(testdataPath("simple.md"))
	if err != nil {
		t.Fatal(err)
	}

	cfg := baseCfg(source)
	cfg.BoundaryDetector = &stubBoundaryDetector{boundaries: []int{2}}

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

	cfg := baseCfg(source)

	result, err := pipeline.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	// With no context and single segment, rendered should match source
	if len(result.Segments) == 1 {
		if result.Rendered[0] != string(source) {
			t.Errorf("lossless round-trip failed for single segment")
		}
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

func TestPipeline_BoundaryDetector_Respected(t *testing.T) {
	// Build source with many small paragraphs
	var src []byte
	for i := 0; i < 10; i++ {
		src = append(src, []byte(fmt.Sprintf("# Section %d\n\nParagraph %d content here.\n\n", i, i))...)
	}

	cfg := pipeline.Config{
		Source:         src,
		SourceName:     "test.md",
		TokenEstimator: tokenizer.NewLocalEstimator(),
		ContextFormat:  "none",
		BoundaryDetector: &stubBoundaryDetector{
			boundaries: []int{3, 7}, // Two boundaries → 3 segments
		},
	}

	result, err := pipeline.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	// Exactly 3 segments
	if len(result.Segments) != 3 {
		t.Errorf("expected 3 segments (boundaries respected), got %d", len(result.Segments))
	}
}

func TestPipeline_LLMRefine(t *testing.T) {
	source, err := os.ReadFile(testdataPath("simple.md"))
	if err != nil {
		t.Fatal(err)
	}

	// stubDetectorWithSims records if SetSimilarities was called
	detector := &stubDetectorWithSims{boundaries: []int{2}}

	cfg := pipeline.Config{
		Source:           source,
		SourceName:       "test.md",
		TokenEstimator:   tokenizer.NewLocalEstimator(),
		BoundaryDetector: detector,
		LLMRefine:        true,
		Embedder:         embedding.NewMockEmbedder(128),
		ContextFormat:    "none",
		EmbedTaskType:    "SEMANTIC_SIMILARITY",
	}

	result, err := pipeline.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	if !detector.simsSet {
		t.Error("expected SetSimilarities to be called in LLM-refine mode")
	}
	if len(result.Segments) == 0 {
		t.Error("expected segments")
	}
	// Embeddings should be populated (not placeholder)
	if len(result.Embeddings) == 0 {
		t.Error("expected embeddings in LLM-refine mode")
	}
	for _, e := range result.Embeddings {
		if len(e.Vector) == 0 {
			t.Error("expected non-empty embedding vectors in LLM-refine mode")
			break
		}
	}
}

// stubDetectorWithSims is a boundary detector that also implements SimilaritySetter.
type stubDetectorWithSims struct {
	boundaries []int
	simsSet    bool
	sims       []float64
}

func (s *stubDetectorWithSims) DetectBoundaries(_ context.Context, blocks []model.Block) (boundary.BoundaryResult, error) {
	return boundary.BoundaryResult{
		Boundaries:  s.boundaries,
		DepthScores: make([]float64, max(len(blocks)-1, 0)),
	}, nil
}

func (s *stubDetectorWithSims) SetSimilarities(sims []float64) {
	s.simsSet = true
	s.sims = sims
}

func TestPipeline_OverlapLines(t *testing.T) {
	source, err := os.ReadFile(testdataPath("simple.md"))
	if err != nil {
		t.Fatal(err)
	}

	cfg := baseCfg(source)
	cfg.BoundaryDetector = &stubBoundaryDetector{boundaries: []int{1}}
	cfg.OverlapLines = 2
	result, err := pipeline.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Segments) == 0 {
		t.Error("expected segments")
	}
}

func TestPipeline_CustomLogger(t *testing.T) {
	source, err := os.ReadFile(testdataPath("simple.md"))
	if err != nil {
		t.Fatal(err)
	}

	cfg := baseCfg(source)
	var buf bytes.Buffer
	cfg.Logger = log.New(&buf, "TEST: ", 0)
	_, err = pipeline.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if buf.Len() == 0 {
		t.Error("expected log output")
	}
}

func TestPipeline_HookOnBoundaryError(t *testing.T) {
	source, err := os.ReadFile(testdataPath("simple.md"))
	if err != nil {
		t.Fatal(err)
	}

	cfg := baseCfg(source)
	cfg.Hooks = &pipeline.Hooks{
		OnBoundary: func(_ boundary.BoundaryResult) error {
			return fmt.Errorf("boundary hook error")
		},
	}
	_, err = pipeline.Run(context.Background(), cfg)
	if err == nil || err.Error() != "hook OnBoundary: boundary hook error" {
		t.Errorf("expected boundary hook error, got %v", err)
	}
}

func TestPipeline_HookOnSegmentError(t *testing.T) {
	source, err := os.ReadFile(testdataPath("simple.md"))
	if err != nil {
		t.Fatal(err)
	}

	cfg := baseCfg(source)
	cfg.Hooks = &pipeline.Hooks{
		OnSegment: func(_ []model.Segment) error {
			return fmt.Errorf("segment hook error")
		},
	}
	_, err = pipeline.Run(context.Background(), cfg)
	if err == nil || err.Error() != "hook OnSegment: segment hook error" {
		t.Errorf("expected segment hook error, got %v", err)
	}
}

func TestPipeline_HookOnRenderError(t *testing.T) {
	source, err := os.ReadFile(testdataPath("simple.md"))
	if err != nil {
		t.Fatal(err)
	}

	cfg := baseCfg(source)
	cfg.Hooks = &pipeline.Hooks{
		OnRender: func(_ int, _ string) error {
			return fmt.Errorf("render hook error")
		},
	}
	_, err = pipeline.Run(context.Background(), cfg)
	if err == nil || err.Error() != "hook OnRender[0]: render hook error" {
		t.Errorf("expected render hook error, got %v", err)
	}
}

func TestPipeline_HookOnEmbedError(t *testing.T) {
	source, err := os.ReadFile(testdataPath("simple.md"))
	if err != nil {
		t.Fatal(err)
	}

	// OnEmbed hook only fires in LLM-refine mode
	cfg := baseCfg(source)
	cfg.LLMRefine = true
	cfg.Embedder = embedding.NewMockEmbedder(128)
	cfg.EmbedTaskType = "SEMANTIC_SIMILARITY"
	cfg.Hooks = &pipeline.Hooks{
		OnEmbed: func(_ []model.Embedding) error {
			return fmt.Errorf("embed hook error")
		},
	}
	_, err = pipeline.Run(context.Background(), cfg)
	if err == nil || err.Error() != "hook OnEmbed: embed hook error" {
		t.Errorf("expected embed hook error, got %v", err)
	}
}
