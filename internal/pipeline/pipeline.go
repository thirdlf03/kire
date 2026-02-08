package pipeline

import (
	"bytes"
	"context"
	"fmt"
	"log"

	"github.com/thirdlf03/kire/internal/boundary"
	"github.com/thirdlf03/kire/internal/embedding"
	"github.com/thirdlf03/kire/internal/model"
	"github.com/thirdlf03/kire/internal/output"
	"github.com/thirdlf03/kire/internal/parser"
	"github.com/thirdlf03/kire/internal/segment"
	"github.com/thirdlf03/kire/internal/tokenizer"
)

// BoundaryDetector detects boundaries directly from blocks without embeddings.
type BoundaryDetector interface {
	DetectBoundaries(ctx context.Context, blocks []model.Block) (boundary.BoundaryResult, error)
}

// SimilaritySetter is an optional interface for boundary detectors that accept
// cosine similarity data (used by --llm-refine mode).
type SimilaritySetter interface {
	SetSimilarities(sims []float64)
}

// Config holds all pipeline configuration.
type Config struct {
	Source         []byte
	SourceName     string // Original filename (for segment ID generation)
	Embedder       embedding.Embedder
	TokenEstimator tokenizer.TokenEstimator
	// BoundaryDetector uses LLM (or other) for boundary detection.
	BoundaryDetector BoundaryDetector

	// LLMRefine enables embedding + cosine similarity → LLM refine mode.
	// When true, embeddings are computed and similarities are passed to the
	// BoundaryDetector via SimilaritySetter before calling DetectBoundaries.
	LLMRefine bool

	OverlapLines    int
	ContextFormat   string
	ContextMaxDepth int
	EmbedTaskType   string
	EmbedDims       int

	ContextExcludePatterns []string

	// Pipeline event hooks (nil = no hooks)
	Hooks *Hooks
	// Logger for pipeline messages (nil = default log.Printf)
	Logger *log.Logger
}

// logf logs a message using the configured logger or the default logger.
func (c Config) logf(format string, args ...any) {
	if c.Logger != nil {
		c.Logger.Printf(format, args...)
	} else {
		log.Printf(format, args...)
	}
}

// Result holds the pipeline output.
type Result struct {
	Blocks     []model.Block
	Segments   []model.Segment
	Rendered   []string
	Boundary   boundary.BoundaryResult
	Embeddings []model.Embedding
	Quality    segment.QualityMetrics
}

// Run executes the full splitting pipeline.
func Run(ctx context.Context, cfg Config) (*Result, error) {
	// Step 1: Parse
	blocks, err := parser.Parse(cfg.Source)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	if len(blocks) == 0 {
		return &Result{}, nil
	}
	cfg.logf("Parsed %d blocks", len(blocks))

	// Hook: OnParse
	if cfg.Hooks != nil && cfg.Hooks.OnParse != nil {
		if err := cfg.Hooks.OnParse(blocks); err != nil {
			return nil, fmt.Errorf("hook OnParse: %w", err)
		}
	}

	// Step 2: Estimate tokens
	for i := range blocks {
		tokens, err := cfg.TokenEstimator.Estimate(blocks[i].Text)
		if err != nil {
			return nil, fmt.Errorf("estimate tokens for block %d: %w", i, err)
		}
		blocks[i].TokensApprox = tokens
	}

	// Step 2.5: Estimate lines per block
	for i := range blocks {
		r := blocks[i].Range
		if r.Start < r.End && r.End <= len(cfg.Source) {
			blocks[i].LinesApprox = bytes.Count(cfg.Source[r.Start:r.End], []byte("\n")) + 1
		} else {
			blocks[i].LinesApprox = 1
		}
	}

	var embeddings []model.Embedding

	// Step 3: LLM-refine mode — compute embeddings and pass similarities
	if cfg.LLMRefine && cfg.Embedder != nil {
		opts := embedding.EmbedOptions{
			TaskType:   cfg.EmbedTaskType,
			Dimensions: cfg.EmbedDims,
		}
		embeddings, err = cfg.Embedder.Embed(ctx, blocks, opts)
		if err != nil {
			return nil, fmt.Errorf("embed: %w", err)
		}
		cfg.logf("Generated %d embeddings for LLM-refine", len(embeddings))

		// Hook: OnEmbed
		if cfg.Hooks != nil && cfg.Hooks.OnEmbed != nil {
			if err := cfg.Hooks.OnEmbed(embeddings); err != nil {
				return nil, fmt.Errorf("hook OnEmbed: %w", err)
			}
		}

		// Compute cosine similarities and pass to detector
		sims := boundary.BlockSimilarities(embeddings, 1)
		if setter, ok := cfg.BoundaryDetector.(SimilaritySetter); ok {
			setter.SetSimilarities(sims)
		}
	} else {
		// Placeholder embeddings for downstream compatibility (quality metrics)
		embeddings = make([]model.Embedding, len(blocks))
		for i := range blocks {
			embeddings[i] = model.Embedding{BlockIndex: i}
		}
	}

	// Step 4: Detect boundaries via LLM
	br, err := cfg.BoundaryDetector.DetectBoundaries(ctx, blocks)
	if err != nil {
		return nil, fmt.Errorf("boundary detection: %w", err)
	}
	cfg.logf("Detected %d boundaries", len(br.Boundaries))

	// Hook: OnBoundary
	if cfg.Hooks != nil && cfg.Hooks.OnBoundary != nil {
		if err := cfg.Hooks.OnBoundary(br); err != nil {
			return nil, fmt.Errorf("hook OnBoundary: %w", err)
		}
	}

	// Step 5: Optimize segments (empty config — LLM boundaries used as-is)
	segments := segment.Optimize(blocks, br, segment.OptConfig{})
	cfg.logf("Created %d segments", len(segments))

	// Step 5.5: Assign content-addressable IDs and prev/next links
	segment.AssignIDs(cfg.SourceName, segments, cfg.Source)

	// Step 5.7: Compute quality metrics (also assigns per-segment coherence)
	quality := segment.ComputeQualityMetrics(segments, blocks, embeddings, br)

	// Hook: OnSegment
	if cfg.Hooks != nil && cfg.Hooks.OnSegment != nil {
		if err := cfg.Hooks.OnSegment(segments); err != nil {
			return nil, fmt.Errorf("hook OnSegment: %w", err)
		}
	}

	// Step 6: Render segments
	rendered := make([]string, len(segments))
	for i, seg := range segments {
		rcfg := output.RenderConfig{
			ContextFormat:          cfg.ContextFormat,
			ContextMaxDepth:        cfg.ContextMaxDepth,
			ContextExcludePatterns: cfg.ContextExcludePatterns,
		}
		// Add overlap from previous segment
		if i > 0 && cfg.OverlapLines > 0 {
			rcfg.OverlapBlocks = output.ComputeOverlapBlocks(
				segments[i-1].Blocks, cfg.Source, cfg.OverlapLines,
			)
		}
		rendered[i] = output.Render(seg, cfg.Source, rcfg)

		// Hook: OnRender
		if cfg.Hooks != nil && cfg.Hooks.OnRender != nil {
			if err := cfg.Hooks.OnRender(i, rendered[i]); err != nil {
				return nil, fmt.Errorf("hook OnRender[%d]: %w", i, err)
			}
		}
	}

	return &Result{
		Blocks:     blocks,
		Segments:   segments,
		Rendered:   rendered,
		Boundary:   br,
		Embeddings: embeddings,
		Quality:    quality,
	}, nil
}
