package main

import (
	"github.com/spf13/cobra"

	"github.com/thirdlf03/kire/internal/embedding"
	"github.com/thirdlf03/kire/internal/parser"
	"github.com/thirdlf03/kire/internal/tokenizer"
)

// CLIConfig collects all flag values into a single structure.
type CLIConfig struct {
	IO       IOConfig
	Segment  SegmentConfig
	Embed    EmbedConfig
	Output   OutputConfig
	Boundary BoundaryConfig
}

// IOConfig holds I/O-related settings.
type IOConfig struct {
	InPaths   []string
	OutDir    string
	Prefix    string
	Debug     bool
	Force     bool
	DryRun    bool
	Quiet     bool
	LogFormat string
	StateFile string
}

// SegmentConfig holds segmentation parameters.
type SegmentConfig struct {
	MinTokens          int
	MaxTokens          int
	MaxLines           int
	Window             int
	BlockK             int
	Threshold          float64
	ThresholdChanged   bool // true if user explicitly set --threshold >= 0
	MinGap             int
	OverlapLines       int
	ContextFormat      string
	ContextMaxDepth    int
	SplitCount         int
	PackHeadingBarrier int
}

// EmbedConfig holds embedding parameters.
type EmbedConfig struct {
	Embedder    string
	EmbedModel  string
	BatchSize   int
	Concurrency int
	QPS         float64
	CachePath   string
}

// OutputConfig holds output-related settings.
type OutputConfig struct {
	JSON          bool
	JSONL         string
	Report        bool
	AgentMetadata bool
	NoIndex       bool
	DagJSON       string
	DagDOT        string
	ShowVersion   bool
}

// BoundaryConfig holds boundary detection settings.
type BoundaryConfig struct {
	DebugBoundary        string
	BoundaryHints        bool
	ForceBoundary        string
	SuppressListBoundary bool
	AtomicBoundary       bool
	SectionLock          bool
	LockAfterHeading     bool
	PseudoHeading        bool
	PseudoHeadingPrefix  string
	ParaSplitPattern     string
}

// ProcessOptions bundles shared objects for processFile.
type ProcessOptions struct {
	CLIConfig
	Embedder  embedding.Embedder
	EmbedInfo string
	EmbedName string
	Estimator tokenizer.TokenEstimator
	PHConfig  parser.PseudoHeadingConfig

	// Derived from CLIConfig
	ThresholdPtr     *float64
	SplitCountPtr    *int
	MaxTokensChanged bool // true if user explicitly set --max-tokens
}

// populateConfig reads all cobra flags into a CLIConfig.
func populateConfig(cmd *cobra.Command) CLIConfig {
	return CLIConfig{
		IO: IOConfig{
			InPaths:   *flInPaths,
			OutDir:    *flOutDir,
			Prefix:    *flPrefix,
			Debug:     *flDebug,
			Force:     *flForce,
			DryRun:    *flDryRun,
			Quiet:     *flQuiet,
			LogFormat: *flLogFormat,
			StateFile: *flStateFile,
		},
		Segment: SegmentConfig{
			MinTokens:          *flMinTokens,
			MaxTokens:          *flMaxTokens,
			MaxLines:           *flMaxLines,
			Window:             *flWindow,
			BlockK:             *flBlockK,
			Threshold:          *flThreshold,
			ThresholdChanged:   *flThreshold >= 0,
			MinGap:             *flMinGap,
			OverlapLines:       *flOverlapLines,
			ContextFormat:      *flContextFormat,
			ContextMaxDepth:    *flContextMaxDepth,
			SplitCount:         *flSplitCount,
			PackHeadingBarrier: *flPackHeadingBarrier,
		},
		Embed: EmbedConfig{
			Embedder:    *flEmbedder,
			EmbedModel:  *flEmbedModel,
			BatchSize:   *flBatchSize,
			Concurrency: *flEmbedConcurrency,
			QPS:         *flEmbedQPS,
			CachePath:   *flCachePath,
		},
		Output: OutputConfig{
			JSON:          *flJSON,
			JSONL:         *flJSONL,
			Report:        *flReport,
			AgentMetadata: *flAgentMetadata,
			NoIndex:       *flNoIndex,
			DagJSON:       *flDagJSON,
			DagDOT:        *flDagDOT,
			ShowVersion:   *flShowVersion,
		},
		Boundary: BoundaryConfig{
			DebugBoundary:        *flDebugBoundary,
			BoundaryHints:        *flBoundaryHints,
			ForceBoundary:        *flForceBoundary,
			SuppressListBoundary: *flSuppressListBoundary,
			AtomicBoundary:       *flAtomicBoundary,
			SectionLock:          *flSectionLock,
			LockAfterHeading:     *flLockAfterHeading,
			PseudoHeading:        *flPseudoHeading,
			PseudoHeadingPrefix:  *flPseudoHeadingPrefix,
			ParaSplitPattern:     *flParaSplitPattern,
		},
	}
}
