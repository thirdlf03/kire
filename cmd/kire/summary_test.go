package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/thirdlf03/kire/internal/boundary"
	"github.com/thirdlf03/kire/internal/model"
	"github.com/thirdlf03/kire/internal/pipeline"
)

func testCLIConfig() CLIConfig {
	return CLIConfig{
		IO: IOConfig{
			DryRun: false,
		},
		Segment: SegmentConfig{
			MinTokens:     300,
			MaxTokens:     3000,
			Window:        3,
			Threshold:     -1,
			MinGap:        3,
			ContextFormat: "comment",
			OverlapLines:  0,
			SplitCount:    0,
		},
		Output: OutputConfig{},
	}
}

func TestBuildSummary(t *testing.T) {
	result := &pipeline.Result{
		Segments: []model.Segment{
			{
				Blocks: []model.Block{
					{Kind: model.BlockHeading, Text: "# Intro", TokensApprox: 10},
					{Kind: model.BlockParagraph, Text: "Hello world", TokensApprox: 20},
				},
				HeadingPath: []string{"Intro"},
				Range:       model.SourceRange{Start: 0, End: 100},
				TokenCount:  30,
			},
			{
				Blocks: []model.Block{
					{Kind: model.BlockParagraph, Text: "Another section", TokensApprox: 15},
				},
				HeadingPath: nil,
				Range:       model.SourceRange{Start: 100, End: 200},
				TokenCount:  15,
			},
		},
		Boundary: boundary.BoundaryResult{
			Boundaries: []int{1},
		},
	}

	filenames := []string{"01-intro.md", "02-another.md"}
	source := []byte("# Intro\nHello world\nAnother section")

	s := buildSummary("test.md", "docs/test", result, filenames, source, "mock(dim=128)", "mock", testCLIConfig())

	if s.Version != version {
		t.Errorf("expected version=%s, got %s", version, s.Version)
	}
	if s.Input != "test.md" {
		t.Errorf("expected input=test.md, got %s", s.Input)
	}
	if s.OutputDir != "docs/test" {
		t.Errorf("expected output_dir=docs/test, got %s", s.OutputDir)
	}
	if s.TotalSegments != 2 {
		t.Errorf("expected total_segments=2, got %d", s.TotalSegments)
	}
	if s.TotalTokens != 45 {
		t.Errorf("expected total_tokens=45, got %d", s.TotalTokens)
	}
	if len(s.Segments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(s.Segments))
	}

	seg0 := s.Segments[0]
	if seg0.Filename != "01-intro.md" {
		t.Errorf("seg[0] filename=%s, want 01-intro.md", seg0.Filename)
	}
	if seg0.BlockCount != 2 {
		t.Errorf("seg[0] block_count=%d, want 2", seg0.BlockCount)
	}
	if seg0.TokenCount != 30 {
		t.Errorf("seg[0] token_count=%d, want 30", seg0.TokenCount)
	}
	if seg0.SourceRange.Start != 0 || seg0.SourceRange.End != 100 {
		t.Errorf("seg[0] source_range={%d,%d}, want {0,100}", seg0.SourceRange.Start, seg0.SourceRange.End)
	}

	// nil HeadingPath should become empty slice
	seg1 := s.Segments[1]
	if seg1.HeadingPath == nil {
		t.Error("seg[1] heading_path should not be nil")
	}
	if len(seg1.HeadingPath) != 0 {
		t.Errorf("seg[1] heading_path len=%d, want 0", len(seg1.HeadingPath))
	}

	if len(s.Boundaries) != 1 || s.Boundaries[0] != 1 {
		t.Errorf("boundaries=%v, want [1]", s.Boundaries)
	}

	if s.Config.Embedder != "mock" {
		t.Errorf("config.embedder=%s, want mock", s.Config.Embedder)
	}
	if s.Config.EmbedderType != "mock(dim=128)" {
		t.Errorf("config.embedder_type=%s, want mock(dim=128)", s.Config.EmbedderType)
	}

	if s.InputHash != pipeline.SourceHash(source) {
		t.Errorf("input_hash=%s, want %s", s.InputHash, pipeline.SourceHash(source))
	}
}

func TestBuildSummary_NilBoundaries(t *testing.T) {
	result := &pipeline.Result{
		Segments: []model.Segment{},
		Boundary: boundary.BoundaryResult{
			Boundaries: nil,
		},
	}

	s := buildSummary("test.md", "docs/test", result, nil, nil, "mock(dim=128)", "mock", testCLIConfig())

	if s.Boundaries == nil {
		t.Error("boundaries should be empty slice, not nil")
	}
	if len(s.Boundaries) != 0 {
		t.Errorf("boundaries len=%d, want 0", len(s.Boundaries))
	}
	if s.InputHash == "" {
		t.Error("input_hash should not be empty")
	}
}

func TestWriteSummaryJSON(t *testing.T) {
	cfg := testCLIConfig()
	cfg.IO.DryRun = true

	result := &pipeline.Result{
		Segments: []model.Segment{
			{
				Blocks:      []model.Block{{Kind: model.BlockParagraph, Text: "test", TokensApprox: 5}},
				HeadingPath: []string{"Test"},
				Range:       model.SourceRange{Start: 0, End: 50},
				TokenCount:  5,
			},
		},
		Boundary: boundary.BoundaryResult{
			Boundaries: []int{},
		},
	}

	s := buildSummary("input.md", "docs/input", result, []string{"01-test.md"}, []byte("test"), "mock(dim=128)", "mock", cfg)

	var buf bytes.Buffer
	if err := writeSummaryJSON(&buf, s); err != nil {
		t.Fatalf("writeSummaryJSON error: %v", err)
	}

	// Verify valid JSON
	var parsed Summary
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON output: %v\nraw: %s", err, buf.String())
	}

	if parsed.Version != version {
		t.Errorf("version=%s, want %s", parsed.Version, version)
	}
	if parsed.Config.DryRun != true {
		t.Error("expected config.dry_run=true")
	}
	if len(parsed.Segments) != 1 {
		t.Errorf("segments len=%d, want 1", len(parsed.Segments))
	}
	if parsed.InputHash == "" {
		t.Error("expected non-empty input_hash in JSON")
	}
	if parsed.Config.Embedder != "mock" {
		t.Errorf("config.embedder=%s, want mock", parsed.Config.Embedder)
	}
}
