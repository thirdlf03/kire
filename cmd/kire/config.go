package main

import (
	"github.com/spf13/cobra"

	"github.com/thirdlf03/kire/internal/embedding"
	"github.com/thirdlf03/kire/internal/pipeline"
	"github.com/thirdlf03/kire/internal/tokenizer"
)

// CLIConfig collects all flag values into a single structure.
type CLIConfig struct {
	IO      IOConfig
	Segment SegmentConfig
	Embed   EmbedConfig
	Output  OutputConfig
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
	OverlapLines    int
	ContextFormat   string
	ContextMaxDepth int
	LLMModel        string
	LLMRefine       bool
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
	AgentMetadata bool
	NoIndex       bool
	DagJSON       string
	DagDOT        string
	ShowVersion   bool
}

// ProcessOptions bundles shared objects for processFile.
type ProcessOptions struct {
	CLIConfig
	Embedder  embedding.Embedder
	EmbedInfo string
	EmbedName string
	Estimator tokenizer.TokenEstimator

	// LLM boundary detector (always non-nil in normal operation)
	LLMDetector pipeline.BoundaryDetector
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
			OverlapLines:    *flOverlapLines,
			ContextFormat:   *flContextFormat,
			ContextMaxDepth: *flContextMaxDepth,
			LLMModel:        *flLLMModel,
			LLMRefine:       *flLLMRefine,
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
			AgentMetadata: *flAgentMetadata,
			NoIndex:       *flNoIndex,
			DagJSON:       *flDagJSON,
			DagDOT:        *flDagDOT,
			ShowVersion:   *flShowVersion,
		},
	}
}
