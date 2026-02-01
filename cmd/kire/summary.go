package main

import (
	"encoding/json"
	"io"

	"github.com/thirdlf03/kire/internal/pipeline"
)

// Summary is the top-level JSON output structure.
type Summary struct {
	Version       string          `json:"version"`
	Input         string          `json:"input"`
	OutputDir     string          `json:"output_dir"`
	Segments      []SegmentInfo   `json:"segments"`
	Boundaries    []int           `json:"boundaries"`
	TotalSegments int             `json:"total_segments"`
	TotalTokens   int             `json:"total_tokens"`
	Config        SummaryConfig   `json:"config"`
	Quality       *SummaryQuality `json:"quality,omitempty"`
}

// SegmentInfo describes a single segment in the JSON output.
type SegmentInfo struct {
	Filename    string       `json:"filename"`
	TokenCount  int          `json:"token_count"`
	BlockCount  int          `json:"block_count"`
	HeadingPath []string     `json:"heading_path"`
	SourceRange SummaryRange `json:"source_range"`
}

// SummaryRange is a byte range in the original source.
type SummaryRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// SummaryQuality holds quality metrics in the JSON output.
type SummaryQuality struct {
	MeanCoherence     float64 `json:"mean_coherence"`
	MeanConfidence    float64 `json:"mean_confidence"`
	MinCoherence      float64 `json:"min_coherence"`
	SegmentCountScore float64 `json:"segment_count_score"`
}

// SummaryConfig captures the pipeline configuration for reproducibility.
type SummaryConfig struct {
	MinTokens     int     `json:"min_tokens"`
	MaxTokens     int     `json:"max_tokens"`
	Window        int     `json:"window"`
	Threshold     float64 `json:"threshold"`
	MinGap        int     `json:"min_gap"`
	EmbedderType  string  `json:"embedder_type"`
	ContextFormat string  `json:"context_format"`
	OverlapLines  int     `json:"overlap_lines"`
	SplitCount    int     `json:"split_count"`
	DryRun        bool    `json:"dry_run"`
}

func buildSummary(inputPath, outputDir string, result *pipeline.Result, filenames []string, source []byte, embedderInfo string, cfg CLIConfig) Summary {
	segments := make([]SegmentInfo, len(result.Segments))
	totalTokens := 0

	for i, seg := range result.Segments {
		info := SegmentInfo{
			BlockCount:  len(seg.Blocks),
			TokenCount:  seg.TokenCount,
			HeadingPath: seg.HeadingPath,
			SourceRange: SummaryRange{
				Start: seg.Range.Start,
				End:   seg.Range.End,
			},
		}
		if info.HeadingPath == nil {
			info.HeadingPath = []string{}
		}
		if i < len(filenames) {
			info.Filename = filenames[i]
		}
		segments[i] = info
		totalTokens += seg.TokenCount
	}

	threshold := float64(-1)
	if cfg.Segment.ThresholdChanged {
		threshold = cfg.Segment.Threshold
	}

	var quality *SummaryQuality
	if cfg.Output.AgentMetadata {
		quality = &SummaryQuality{
			MeanCoherence:     result.Quality.MeanCoherence,
			MeanConfidence:    result.Quality.MeanConfidence,
			MinCoherence:      result.Quality.MinCoherence,
			SegmentCountScore: result.Quality.SegmentCountScore,
		}
	}

	return Summary{
		Version:       version,
		Input:         inputPath,
		OutputDir:     outputDir,
		Segments:      segments,
		Boundaries:    normalizeBoundaries(result.Boundary.Boundaries),
		TotalSegments: len(result.Segments),
		TotalTokens:   totalTokens,
		Quality:       quality,
		Config: SummaryConfig{
			MinTokens:     cfg.Segment.MinTokens,
			MaxTokens:     cfg.Segment.MaxTokens,
			Window:        cfg.Segment.Window,
			Threshold:     threshold,
			MinGap:        cfg.Segment.MinGap,
			EmbedderType:  embedderInfo,
			ContextFormat: cfg.Segment.ContextFormat,
			OverlapLines:  cfg.Segment.OverlapLines,
			SplitCount:    cfg.Segment.SplitCount,
			DryRun:        cfg.IO.DryRun,
		},
	}
}

// normalizeBoundaries ensures a nil slice is emitted as [] in JSON.
func normalizeBoundaries(b []int) []int {
	if b == nil {
		return []int{}
	}
	return b
}

func writeSummaryJSON(w io.Writer, s Summary) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(s)
}
