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
	Long: `Split long Markdown files at semantic boundaries.
Uses Gemini Embedding + TextTiling-style boundary detection.

Usage:
  kire [options] [<input-file>]
  kire --in a.md --in b.md [options]`,
	SilenceUsage:  true,
	SilenceErrors: true,
	Args:          cobra.ArbitraryArgs,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		switch len(args) {
		case 0:
			// First arg: .md file completion
			return []string{"md"}, cobra.ShellCompDirectiveFilterFileExt
		case 1:
			// Second arg: directory completion
			return nil, cobra.ShellCompDirectiveFilterDirs
		default:
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
	},
	RunE: runSplitter,
}

// Flag pointers — set in init(), consumed in runSplitter().
var (
	flInPaths              *[]string
	flOutDir               *string
	flPrefix               *string
	flDebug                *bool
	flMinTokens            *int
	flMaxTokens            *int
	flWindow               *int
	flThreshold            *float64
	flOverlapLines         *int
	flContextFormat        *string
	flContextMaxDepth      *int
	flMinGap               *int
	flBatchSize            *int
	flEmbedConcurrency     *int
	flEmbedQPS             *float64
	flCachePath            *string
	flEmbedModel           *string
	flPseudoHeading        *bool
	flPseudoHeadingPrefix  *string
	flDebugBoundary        *string
	flBoundaryHints        *bool
	flSectionLock          *bool
	flLockAfterHeading     *bool
	flParaSplitPattern     *string
	flForceBoundary        *string
	flSuppressListBoundary *bool
	flAtomicBoundary       *bool
	flSplitCount           *int
	flForce                *bool
	flDagJSON              *string
	flDagDOT               *string
	flNoIndex              *bool
	flShowVersion          *bool
	flQuiet                *bool
	flLogFormat            *string
	flEmbedder             *string
	flDryRun               *bool
	flJSON                 *bool
	flJSONL                *string
	flReport               *bool
	flMaxLines             *int
	flPackHeadingBarrier   *int
	flAgentMetadata        *bool
	flStateFile            *string
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
	flReport = f.Bool("report", false, "Write HTML boundary report to output directory")
	flEmbedder = f.String("embedder", "auto", "Embedder provider: auto|"+strings.Join(embedding.List(), "|"))
	flAgentMetadata = f.Bool("agent-metadata", false, "Include multi-agent metadata (segment IDs, coherence) in JSONL output")
	flStateFile = f.String("state-file", "", "Incremental processing state file path (enables change detection)")
	// Segmentation
	flMinTokens = f.Int("min-tokens", 300, "Minimum tokens per segment")
	flMaxTokens = f.Int("max-tokens", 3000, "Maximum tokens per segment (0 = unlimited; auto-disabled when --max-lines is active)")
	flMaxLines = f.Int("max-lines", -1, "Maximum lines per segment (-1 = auto, 0 = unlimited)")
	flPackHeadingBarrier = f.Int("pack-heading-barrier", 0, "Heading level barrier for segment packing (0=disabled)")
	flWindow = f.Int("window", 3, "Similarity smoothing window size")
	flThreshold = f.Float64("threshold", -1, "Boundary depth score threshold (-1 = auto)")
	flOverlapLines = f.Int("overlap", 0, "Overlap lines between segments")
	flContextFormat = f.String("context-format", "comment", "Context format: comment|front-matter|heading|none")
	flContextMaxDepth = f.Int("context-max-depth", 0, "Max heading depth for context (0 = unlimited)")
	flMinGap = f.Int("min-gap", 3, "Minimum gap between boundaries")
	flSplitCount = f.Int("split-count", 0, "Target number of segments (0 = auto)")

	// Embedding
	flBatchSize = f.Int("batch-size", 32, "Batch size for embedding API")
	flEmbedConcurrency = f.Int("embed-concurrency", 4, "Embedding API concurrency")
	flEmbedQPS = f.Float64("embed-qps", 0, "Embedding API QPS limit (0 = unlimited)")
	flCachePath = f.String("cache", "", "Embedding cache file path")
	flEmbedModel = f.String("embed-model", "", "Embedding model name (provider default if empty)")
	// Pseudo-heading
	flPseudoHeading = negatable(rootCmd, "pseudo-heading", true, "Enable pseudo-heading detection")
	flPseudoHeadingPrefix = f.String("pseudo-heading-prefix", "", "Comma-separated regex patterns for pseudo-heading prefix matching (bypasses MaxRuneLen)")

	// Boundary
	flDebugBoundary = f.String("debug-boundary", "", "Write boundary debug TSV to file (- for stdout)")
	flBoundaryHints = negatable(rootCmd, "boundary-hints", true, "Enable boundary prohibition/enforcement rules")
	flForceBoundary = f.String("force-boundary", "", "Comma-separated regex patterns for forced boundary (HintEnforce)")
	flSuppressListBoundary = negatable(rootCmd, "suppress-list-boundary", true, "Suppress boundaries between consecutive lists and heading→list")
	flAtomicBoundary = negatable(rootCmd, "atomic-boundary", true, "Prohibit boundaries before code/table/math blocks")

	// Section lock
	flSectionLock = negatable(rootCmd, "section-lock", true, "Enable section lock (keep lead+body together)")
	flLockAfterHeading = negatable(rootCmd, "lock-after-heading", true, "Also lock heading+following blocks in section lock")

	// Paragraph split
	flParaSplitPattern = f.String("para-split-pattern", "", "Comma-separated regex patterns to split long paragraphs")

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
	_ = rootCmd.RegisterFlagCompletionFunc("debug-boundary", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
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
