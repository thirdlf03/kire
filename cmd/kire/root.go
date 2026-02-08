package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/thirdlf03/kire/internal/embedding"
)

// version is set via ldflags at build time:
//
//	go build -ldflags "-X main.version=1.2.3"
var version = "0.0.1-dev"

var rootCmd = &cobra.Command{
	Use:   "kire [options] [<input-file>] [<output-dir>]",
	Short: "Split long Markdown files at semantic boundaries",
	Long: `Split long Markdown files at semantic boundaries using LLM.

Usage:
  kire [options] [<input-file>]
  kire --in a.md --in b.md [options]`,
	SilenceUsage:  true,
	SilenceErrors: true,
	Args:          cobra.ArbitraryArgs,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		switch len(args) {
		case 0:
			return []string{"md"}, cobra.ShellCompDirectiveFilterFileExt
		case 1:
			return nil, cobra.ShellCompDirectiveFilterDirs
		default:
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
	},
	RunE: runSplitter,
}

// Flag pointers — set in init(), consumed in runSplitter().
var (
	flInPaths          *[]string
	flOutDir           *string
	flPrefix           *string
	flDebug            *bool
	flOverlapLines     *int
	flContextFormat    *string
	flContextMaxDepth  *int
	flBatchSize        *int
	flEmbedConcurrency *int
	flEmbedQPS         *float64
	flCachePath        *string
	flEmbedModel       *string
	flForce            *bool
	flDagJSON          *string
	flDagDOT           *string
	flNoIndex          *bool
	flShowVersion      *bool
	flQuiet            *bool
	flLogFormat        *string
	flEmbedder         *string
	flDryRun           *bool
	flJSON             *bool
	flJSONL            *string
	flAgentMetadata    *bool
	flStateFile        *string
	flLLMModel         *string
	flLLMRefine        *bool
)

func init() {
	f := rootCmd.Flags()

	// I/O
	flInPaths = f.StringArray("in", nil, "Input Markdown file(s) (required, repeatable)")
	flOutDir = f.StringP("out", "o", "docs", "Output directory")
	flPrefix = f.String("prefix", "", "Output file prefix (empty = semantic naming)")
	flDebug = f.BoolP("debug", "d", false, "Enable debug logging")
	flForce = f.BoolP("force", "f", false, "Overwrite existing output directories without confirmation")
	flShowVersion = f.BoolP("version", "V", false, "Show version")
	flQuiet = f.BoolP("quiet", "q", false, "Suppress all log output")
	flLogFormat = f.String("log-format", "text", "Log format: text|json")
	flDryRun = f.BoolP("dry-run", "n", false, "Run pipeline without writing files")
	flJSON = f.Bool("json", false, "Output JSON summary to stdout")
	flJSONL = f.String("jsonl", "", "Write JSONL metadata (default: to output directory, --jsonl=- for stdout)")
	rootCmd.Flags().Lookup("jsonl").NoOptDefVal = "auto"
	flAgentMetadata = f.Bool("agent-metadata", false, "Include multi-agent metadata (segment IDs, coherence) in JSONL output")
	flStateFile = f.String("state-file", "", "Incremental processing state file path (enables change detection)")

	// LLM
	flLLMModel = f.String("llm-model", "gemini-2.5-flash-lite", "LLM model for boundary detection")
	flLLMRefine = f.Bool("llm-refine", false, "Enable embedding + cosine similarity refinement for LLM boundary detection")

	// Segmentation
	flOverlapLines = f.Int("overlap", 0, "Overlap lines between segments")
	flContextFormat = f.String("context-format", "comment", "Context format: comment|front-matter|heading|none")
	flContextMaxDepth = f.Int("context-max-depth", 0, "Max heading depth for context (0 = unlimited)")

	// Embedding (for --llm-refine)
	flEmbedder = f.String("embedder", "auto", "Embedder provider (for --llm-refine): auto|"+strings.Join(embedding.List(), "|"))
	flBatchSize = f.Int("batch-size", 32, "Batch size for embedding API")
	flEmbedConcurrency = f.Int("embed-concurrency", 4, "Embedding API concurrency")
	flEmbedQPS = f.Float64("embed-qps", 0, "Embedding API QPS limit (0 = unlimited)")
	flCachePath = f.String("cache", "", "Embedding cache file path")
	flEmbedModel = f.String("embed-model", "", "Embedding model name (provider default if empty)")

	// DAG
	flDagJSON = f.String("dag-json", "", "Output DAG as JSON to file")
	flDagDOT = f.String("dag-dot", "", "Output DAG as DOT to file")
	flNoIndex = f.Bool("no-index", false, "Disable index.md generation")

	// --- Flag completion ---
	_ = rootCmd.RegisterFlagCompletionFunc("context-format", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"comment", "front-matter", "heading", "none"}, cobra.ShellCompDirectiveNoFileComp
	})
	_ = rootCmd.RegisterFlagCompletionFunc("in", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"md"}, cobra.ShellCompDirectiveFilterFileExt
	})
	_ = rootCmd.RegisterFlagCompletionFunc("cache", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveDefault
	})
	_ = rootCmd.RegisterFlagCompletionFunc("dag-json", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveDefault
	})
	_ = rootCmd.RegisterFlagCompletionFunc("dag-dot", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveDefault
	})
	_ = rootCmd.RegisterFlagCompletionFunc("log-format", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"text", "json"}, cobra.ShellCompDirectiveNoFileComp
	})
	_ = rootCmd.RegisterFlagCompletionFunc("embedder", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return append([]string{"auto"}, embedding.List()...), cobra.ShellCompDirectiveNoFileComp
	})
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "kire:", err)
		os.Exit(1)
	}
}
