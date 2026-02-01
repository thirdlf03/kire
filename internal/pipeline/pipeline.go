package pipeline

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"regexp"

	"github.com/thirdlf03/kire/internal/boundary"
	"github.com/thirdlf03/kire/internal/embedding"
	"github.com/thirdlf03/kire/internal/model"
	"github.com/thirdlf03/kire/internal/output"
	"github.com/thirdlf03/kire/internal/parser"
	"github.com/thirdlf03/kire/internal/segment"
	"github.com/thirdlf03/kire/internal/tokenizer"
)

// Config holds all pipeline configuration.
type Config struct {
	Source         []byte
	SourceName     string // Original filename (for segment ID generation)
	Embedder       embedding.Embedder
	TokenEstimator tokenizer.TokenEstimator

	MinTokens       int
	MaxTokens       int
	Window          int
	Threshold       *float64
	MinGap          int
	OverlapLines    int
	ContextFormat   string
	ContextMaxDepth int
	EmbedTaskType   string
	EmbedDims       int

	ContextExcludePatterns []string
	PseudoHeading          parser.PseudoHeadingConfig
	DebugBoundaryWriter    io.Writer
	EnableBoundaryHints    bool
	SectionLock            segment.LockConfig

	// Phase 1: Paragraph split patterns (split long paragraphs at matching lines)
	ParaSplitPatterns []string
	// Phase 3: User-defined forced boundary patterns (HintEnforce before matching blocks)
	ForceEnforcePatterns []string
	// Phase 5: Suppress list boundaries (prohibit list→list and heading→list splits)
	SuppressListBoundary bool
	// Phase 6: Atomic block protection (prohibit boundary before code/table/math)
	AtomicBoundaryProtection bool
	// Phase 7: Target split count (nil = auto)
	SplitCount *int
	// Maximum lines per segment (0 = unlimited)
	MaxLines int
	// Heading level barrier for segment packing (0 = disabled)
	PackHeadingBarrier int
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

	// Step 1.2: Split long paragraphs at pattern-matching lines
	if len(cfg.ParaSplitPatterns) > 0 {
		var patterns []*regexp.Regexp
		for _, p := range cfg.ParaSplitPatterns {
			re, err := regexp.Compile(p)
			if err != nil {
				cfg.logf("WARNING: invalid para-split-pattern %q: %v", p, err)
				continue
			}
			patterns = append(patterns, re)
		}
		if len(patterns) > 0 {
			blocks = parser.SplitLongParagraphs(blocks, patterns, cfg.Source)
			cfg.logf("Split long paragraphs: %d blocks after split", len(blocks))
		}
	}

	// Step 1.3: Auto-split paragraphs at pseudo-heading candidates
	if cfg.PseudoHeading.Enabled {
		before := len(blocks)
		blocks = parser.AutoSplitParagraphs(blocks, cfg.PseudoHeading, cfg.Source)
		if len(blocks) != before {
			cfg.logf("Auto-split paragraphs at pseudo-heading candidates: %d → %d blocks", before, len(blocks))
		}
	}

	// Step 1.5: Pseudo-heading detection
	if cfg.PseudoHeading.Enabled {
		blocks = parser.DetectPseudoHeadings(blocks, cfg.PseudoHeading)
		cfg.logf("Pseudo-heading detection applied")
	}

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
	totalLines := 0
	for i := range blocks {
		r := blocks[i].Range
		if r.Start < r.End && r.End <= len(cfg.Source) {
			blocks[i].LinesApprox = bytes.Count(cfg.Source[r.Start:r.End], []byte("\n")) + 1
		} else {
			blocks[i].LinesApprox = 1
		}
		totalLines += blocks[i].LinesApprox
	}

	// Auto-compute MaxLines if requested (-1)
	if cfg.MaxLines < 0 {
		cfg.MaxLines = segment.AutoMaxLines(totalLines)
		cfg.logf("Auto max-lines: %d (total %d lines)", cfg.MaxLines, totalLines)
	}

	// Step 3: Embed
	opts := embedding.EmbedOptions{
		TaskType:   cfg.EmbedTaskType,
		Dimensions: cfg.EmbedDims,
	}
	embeddings, err := cfg.Embedder.Embed(ctx, blocks, opts)
	if err != nil {
		return nil, fmt.Errorf("embed: %w", err)
	}
	cfg.logf("Generated %d embeddings", len(embeddings))

	// Hook: OnEmbed
	if cfg.Hooks != nil && cfg.Hooks.OnEmbed != nil {
		if err := cfg.Hooks.OnEmbed(embeddings); err != nil {
			return nil, fmt.Errorf("hook OnEmbed: %w", err)
		}
	}

	// Step 4: Detect boundaries
	scoringCfg := boundary.ScoringConfig{
		Window:    cfg.Window,
		Threshold: cfg.Threshold,
		MinGap:    cfg.MinGap,
	}
	if cfg.EnableBoundaryHints {
		scoringCfg.Hints = boundary.GenerateHints(blocks, boundary.DefaultHintRules())
		cfg.logf("Generated %d boundary hints", len(scoringCfg.Hints))
	}
	// User-defined force-boundary patterns (HintEnforce)
	if len(cfg.ForceEnforcePatterns) > 0 {
		enforceRules := boundary.UserEnforceRules(cfg.ForceEnforcePatterns)
		enforceHints := boundary.GenerateHints(blocks, enforceRules)
		scoringCfg.Hints = append(scoringCfg.Hints, enforceHints...)
		cfg.logf("Added %d user enforce hints", len(enforceHints))
	}
	// Suppress list boundaries (list→list and heading→list)
	if cfg.SuppressListBoundary {
		listHints := boundary.GenerateListHints(blocks)
		scoringCfg.Hints = append(scoringCfg.Hints, listHints...)
		cfg.logf("Added %d list boundary suppression hints", len(listHints))
	}
	// Atomic block protection (code/table/math)
	if cfg.AtomicBoundaryProtection {
		atomicHints := boundary.GenerateAtomicHints(blocks)
		scoringCfg.Hints = append(scoringCfg.Hints, atomicHints...)
		cfg.logf("Added %d atomic block protection hints", len(atomicHints))
	}
	// Target split count
	scoringCfg.TargetCount = cfg.SplitCount
	br := boundary.DetectBoundaries(embeddings, scoringCfg)
	cfg.logf("Detected %d boundaries", len(br.Boundaries))

	// Debug boundary output
	if cfg.DebugBoundaryWriter != nil {
		if err := boundary.WriteDebugTSV(cfg.DebugBoundaryWriter, blocks, br, 40); err != nil {
			return nil, fmt.Errorf("write debug boundary: %w", err)
		}
	}

	// Step 4.5: Collect atomic gaps for oversized re-split protection
	atomicGaps := make(map[int]bool)
	if cfg.AtomicBoundaryProtection {
		for i := 1; i < len(blocks); i++ {
			k := blocks[i].Kind
			if k == model.BlockCodeBlock || k == model.BlockTable || k == model.BlockMathBlock {
				atomicGaps[i-1] = true
			}
		}
	}

	// Step 5: Section lock detection
	var lockRanges []segment.LockRange
	if cfg.SectionLock.Enabled {
		lockRanges = segment.DetectLockRanges(blocks, cfg.SectionLock)
		cfg.logf("Detected %d section lock ranges", len(lockRanges))
		// Convert lock ranges to prohibit hints and add to boundary result
		lockedGaps := segment.LockedGaps(lockRanges)
		for gapIdx := range lockedGaps {
			scoringCfg.Hints = append(scoringCfg.Hints, boundary.BoundaryHint{
				GapIndex: gapIdx,
				Kind:     boundary.HintProhibit,
			})
		}
		// Re-run boundary detection with lock hints if any were added
		if len(lockedGaps) > 0 {
			br = boundary.DetectBoundaries(embeddings, scoringCfg)
			cfg.logf("Re-detected %d boundaries after section lock", len(br.Boundaries))
		}
	}

	// Hook: OnBoundary
	if cfg.Hooks != nil && cfg.Hooks.OnBoundary != nil {
		if err := cfg.Hooks.OnBoundary(br); err != nil {
			return nil, fmt.Errorf("hook OnBoundary: %w", err)
		}
	}

	// Add atomic gaps as lock ranges for oversized re-split protection
	for gapIdx := range atomicGaps {
		lockRanges = append(lockRanges, segment.LockRange{Start: gapIdx, End: gapIdx + 2})
	}

	// Step 6: Optimize segments
	optCfg := segment.OptConfig{
		MinTokens:          cfg.MinTokens,
		MaxTokens:          cfg.MaxTokens,
		MaxLines:           cfg.MaxLines,
		LockRanges:         lockRanges,
		PackHeadingBarrier: cfg.PackHeadingBarrier,
	}
	segments := segment.Optimize(blocks, br, optCfg)
	cfg.logf("Created %d segments", len(segments))

	// Step 6.5: Assign content-addressable IDs and prev/next links
	segment.AssignIDs(cfg.SourceName, segments, cfg.Source)

	// Step 6.7: Compute quality metrics (also assigns per-segment coherence)
	quality := segment.ComputeQualityMetrics(segments, blocks, embeddings, br)

	// Hook: OnSegment
	if cfg.Hooks != nil && cfg.Hooks.OnSegment != nil {
		if err := cfg.Hooks.OnSegment(segments); err != nil {
			return nil, fmt.Errorf("hook OnSegment: %w", err)
		}
	}

	// Warn if segment count is extreme
	if len(segments) > 20 {
		cfg.logf("WARNING: high segment count (%d), consider adjusting min/max tokens", len(segments))
	}
	if len(segments) == 1 && len(blocks) > 10 {
		cfg.logf("WARNING: single segment from %d blocks, consider lowering max tokens", len(blocks))
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
