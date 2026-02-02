package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/thirdlf03/kire/internal/boundary"
	"github.com/thirdlf03/kire/internal/embedding"
	"github.com/thirdlf03/kire/internal/eval"
	"github.com/thirdlf03/kire/internal/model"
	"github.com/thirdlf03/kire/internal/parser"
	"github.com/thirdlf03/kire/internal/pipeline"
	"github.com/thirdlf03/kire/internal/segment"
	"github.com/thirdlf03/kire/internal/tokenizer"
)

var benchCmd = &cobra.Command{
	Use:   "bench [flags] <gold.json> <input.md>",
	Short: "Evaluate segmentation quality against gold standard",
	Long: `Evaluate segmentation quality against a gold standard annotation.

Runs the kire pipeline on the input file and compares the resulting
boundaries against a manually annotated gold standard. Reports Pk,
WindowDiff, and boundary Precision/Recall/F1 metrics.

By default, also runs baseline methods (heading-split, fixed-split,
random-split) for comparison.

Examples:
  kire bench testdata/gold/simple.json testdata/simple.md
  kire bench --json testdata/gold/bench_long.json testdata/bench_long.md
  kire bench --embedder tfidf --tolerance 2 gold.json input.md
  kire bench --profile embedder testdata/gold/bench_long.json testdata/bench_long.md
  kire bench --profile output testdata/gold/bench_long.json testdata/bench_long.md`,
	Args: cobra.ExactArgs(2),
	RunE: runBench,
}

var (
	flBenchJSON               *bool
	flBenchProfile            *string
	flBenchEmbedder           *string
	flBenchEmbedModel         *string
	flBenchBatchSize          *int
	flBenchTolerance          *int
	flBenchK                  *int
	flBenchEvalStage          *string
	flBenchNoBaselines        *bool
	flBenchAllowBlockMismatch *bool
	flBenchBaselineMinGap     *bool

	flBenchMinTokens          *int
	flBenchMaxTokens          *int
	flBenchMaxLines           *int
	flBenchWindow             *int
	flBenchBlockK             *int
	flBenchThreshold          *float64
	flBenchMinGap             *int
	flBenchSplitCount         *int
	flBenchPackHeadingBarrier *int

	flBenchPseudoHeading        *bool
	flBenchPseudoHeadingPrefix  *string
	flBenchBoundaryHints        *bool
	flBenchSectionLock          *bool
	flBenchLockAfterHeading     *bool
	flBenchParaSplitPattern     *string
	flBenchForceBoundary        *string
	flBenchSuppressListBoundary *bool
	flBenchAtomicBoundary       *bool
	flBenchBoundaryMethod       *string
	flBenchBeta                 *float64
)

func init() {
	f := benchCmd.Flags()
	flBenchJSON = f.Bool("json", false, "Output JSON report to stdout")
	flBenchProfile = f.String("profile", "", "Preset for bench parameters: embedder|output")
	flBenchEmbedder = f.String("embedder", "tfidf", "Embedder provider: "+strings.Join(embedding.List(), "|"))
	flBenchEmbedModel = f.String("embed-model", "", "Embedding model name (provider default if empty)")
	flBenchBatchSize = f.Int("batch-size", 32, "Batch size for embedding API")
	flBenchTolerance = f.Int("tolerance", 1, "Boundary matching tolerance (blocks)")
	flBenchK = f.Int("k", 0, "Pk/WindowDiff window half-width (0 = auto)")
	flBenchEvalStage = f.String("eval-stage", "final", "Evaluation stage: raw|final|both")
	flBenchNoBaselines = f.Bool("no-baselines", false, "Skip baseline comparisons")
	flBenchAllowBlockMismatch = f.Bool("allow-block-mismatch", false, "Allow evaluation even if block count changes after preprocessing")
	flBenchBaselineMinGap = f.Bool("baseline-min-gap", true, "Apply min-gap constraint to baseline boundaries")

	// Segmentation config (align with CLI defaults)
	flBenchMinTokens = f.Int("min-tokens", 300, "Minimum tokens per segment")
	flBenchMaxTokens = f.Int("max-tokens", 3000, "Maximum tokens per segment (0 = unlimited; auto-disabled when --max-lines is active)")
	flBenchMaxLines = f.Int("max-lines", -1, "Maximum lines per segment (-1 = auto, 0 = unlimited)")
	flBenchWindow = f.Int("window", 3, "Similarity smoothing window size")
	flBenchBlockK = f.Int("block-k", 3, "Block comparison window (k embeddings per side, 1=adjacent)")
	flBenchThreshold = f.Float64("threshold", -1, "Boundary depth score threshold (-1 = auto)")
	flBenchMinGap = f.Int("min-gap", 3, "Minimum gap between boundaries")
	flBenchSplitCount = f.Int("split-count", 0, "Target number of segments (0 = auto)")
	flBenchPackHeadingBarrier = f.Int("pack-heading-barrier", 0, "Heading level barrier for segment packing (0=disabled)")
	flBenchBoundaryMethod = f.String("boundary-method", "texttiling", "Boundary detection method: texttiling|kcpd")
	flBenchBeta = f.Float64("beta", -1, "KCPD penalty parameter (-1 = auto)")

	// Pseudo-heading
	flBenchPseudoHeading = negatable(benchCmd, "pseudo-heading", true, "Enable pseudo-heading detection")
	flBenchPseudoHeadingPrefix = f.String("pseudo-heading-prefix", "", "Comma-separated regex patterns for pseudo-heading prefix matching (bypasses MaxRuneLen)")

	// Boundary
	flBenchBoundaryHints = negatable(benchCmd, "boundary-hints", true, "Enable boundary prohibition/enforcement rules")
	flBenchParaSplitPattern = f.String("para-split-pattern", "", "Comma-separated regex patterns to split long paragraphs")
	flBenchForceBoundary = f.String("force-boundary", "", "Comma-separated regex patterns for forced boundary (HintEnforce)")
	flBenchSuppressListBoundary = negatable(benchCmd, "suppress-list-boundary", true, "Suppress boundaries between consecutive lists and heading→list")
	flBenchAtomicBoundary = negatable(benchCmd, "atomic-boundary", true, "Prohibit boundaries before code/table/math blocks")

	// Section lock
	flBenchSectionLock = negatable(benchCmd, "section-lock", true, "Enable section lock (keep lead+body together)")
	flBenchLockAfterHeading = negatable(benchCmd, "lock-after-heading", true, "Also lock heading+following blocks in section lock")

	_ = benchCmd.RegisterFlagCompletionFunc("embedder", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return embedding.List(), cobra.ShellCompDirectiveNoFileComp
	})
	_ = benchCmd.RegisterFlagCompletionFunc("eval-stage", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"raw", "final", "both"}, cobra.ShellCompDirectiveNoFileComp
	})
	_ = benchCmd.RegisterFlagCompletionFunc("profile", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"embedder", "output"}, cobra.ShellCompDirectiveNoFileComp
	})

	rootCmd.AddCommand(benchCmd)
}

func runBench(cmd *cobra.Command, args []string) error {
	if err := applyBenchProfile(cmd, *flBenchProfile); err != nil {
		return err
	}
	goldPath := args[0]
	inputPath := args[1]

	// Load gold annotation
	gold, err := eval.LoadGold(goldPath)
	if err != nil {
		return fmt.Errorf("load gold: %w", err)
	}
	if err := applyGoldParams(cmd, gold.Params); err != nil {
		return err
	}
	resolveNegatables()

	stage, err := parseEvalStage(*flBenchEvalStage)
	if err != nil {
		return err
	}

	// Read input
	source, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read input: %w", err)
	}

	// Parse for block-count validation (pipeline may change block count)
	baseBlocks, err := parser.Parse(source)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}

	ctx := cmd.Context()

	embedder, _, err := embedding.Create(ctx, *flBenchEmbedder, embedding.ProviderConfig{
		Model:     *flBenchEmbedModel,
		BatchSize: *flBenchBatchSize,
	})
	if err != nil {
		return fmt.Errorf("create embedder %s: %w", *flBenchEmbedder, err)
	}

	estimator := tokenizer.NewLocalEstimator()

	phCfg := parser.DefaultPseudoHeadingConfig()
	phCfg.Enabled = *flBenchPseudoHeading
	if *flBenchPseudoHeadingPrefix != "" {
		phCfg.PrefixPatterns = splitCSV(*flBenchPseudoHeadingPrefix)
	}

	var thresholdPtr *float64
	if *flBenchThreshold >= 0 {
		t := *flBenchThreshold
		thresholdPtr = &t
	}

	var splitCountPtr *int
	if *flBenchSplitCount > 0 {
		sc := *flBenchSplitCount
		splitCountPtr = &sc
	}

	// Determine effective MaxTokens if max-lines is active and max-tokens unchanged.
	effectiveMaxTokens := *flBenchMaxTokens
	if *flBenchMaxLines != 0 && !cmd.Flags().Changed("max-tokens") {
		effectiveMaxTokens = 0
	}

	// Beta pointer for KCPD
	var benchBetaPtr *float64
	if *flBenchBeta >= 0 {
		b := *flBenchBeta
		benchBetaPtr = &b
	}

	pipeCfg := pipeline.Config{
		Source:                   source,
		SourceName:               filepath.Base(inputPath),
		Embedder:                 embedder,
		TokenEstimator:           estimator,
		MinTokens:                *flBenchMinTokens,
		MaxTokens:                effectiveMaxTokens,
		MaxLines:                 *flBenchMaxLines,
		Window:                   *flBenchWindow,
		BlockK:                   *flBenchBlockK,
		Threshold:                thresholdPtr,
		MinGap:                   *flBenchMinGap,
		EmbedTaskType:            "SEMANTIC_SIMILARITY",
		PseudoHeading:            phCfg,
		EnableBoundaryHints:      *flBenchBoundaryHints,
		SectionLock:              buildLockConfig(*flBenchSectionLock, *flBenchLockAfterHeading),
		ParaSplitPatterns:        splitCSV(*flBenchParaSplitPattern),
		ForceEnforcePatterns:     splitCSV(*flBenchForceBoundary),
		SuppressListBoundary:     *flBenchSuppressListBoundary,
		AtomicBoundaryProtection: *flBenchAtomicBoundary,
		SplitCount:               splitCountPtr,
		PackHeadingBarrier:       *flBenchPackHeadingBarrier,
		BoundaryMethod:           *flBenchBoundaryMethod,
		Beta:                     benchBetaPtr,
	}

	result, err := pipeline.Run(ctx, pipeCfg)
	if err != nil {
		return fmt.Errorf("kire pipeline: %w", err)
	}

	blocks := result.Blocks
	numBlocks := len(blocks)
	if len(baseBlocks) != numBlocks {
		msg := fmt.Sprintf("block count changed after preprocessing: parse=%d pipeline=%d (gold boundaries may no longer align)", len(baseBlocks), numBlocks)
		if !*flBenchAllowBlockMismatch {
			return fmt.Errorf("%s; disable pseudo-heading/para-split or use --allow-block-mismatch", msg)
		}
		cmd.PrintErrf("WARNING: %s\n", msg)
	}

	if err := validateGold(gold, numBlocks); err != nil {
		return err
	}

	warnSmallDoc(cmd, numBlocks)
	warnMinGapCeiling(cmd, gold.Boundaries, *flBenchMinGap)
	warnAutoK(cmd, gold.Boundaries, numBlocks, *flBenchK)

	report := eval.EvalReport{
		DocID:     gold.DocID,
		NumBlocks: numBlocks,
		Gold:      *gold,
	}

	includeStage := !(stage.raw && !stage.final)

	if stage.raw {
		if err := validateBoundaries("kire raw", result.Boundary.Boundaries, numBlocks); err != nil {
			return err
		}
		name := methodName("kire", *flBenchEmbedder, "raw", includeStage)
		report.Methods = append(report.Methods,
			eval.Evaluate(gold, numBlocks, result.Boundary.Boundaries, name, *flBenchK, *flBenchTolerance),
		)
	}

	if stage.final {
		finalBoundaries, err := boundariesFromSegments(result.Segments, blocks)
		if err != nil {
			return fmt.Errorf("kire final boundaries: %w", err)
		}
		if err := validateBoundaries("kire final", finalBoundaries, numBlocks); err != nil {
			return err
		}
		name := methodName("kire", *flBenchEmbedder, "final", includeStage)
		report.Methods = append(report.Methods,
			eval.Evaluate(gold, numBlocks, finalBoundaries, name, *flBenchK, *flBenchTolerance),
		)
	}

	// Baselines
	if !*flBenchNoBaselines {
		headingBoundaries := eval.HeadingSplit(blocks)

		avgGoldSegLen := numBlocks / (len(gold.Boundaries) + 1)
		if avgGoldSegLen < 2 {
			avgGoldSegLen = 2
		}
		fixedBoundaries := eval.FixedSplit(numBlocks, avgGoldSegLen)
		randomBoundaries := eval.RandomSplit(numBlocks, len(gold.Boundaries), 42)

		if *flBenchBaselineMinGap {
			headingBoundaries = applyMinGap(headingBoundaries, *flBenchMinGap)
			fixedBoundaries = applyMinGap(fixedBoundaries, *flBenchMinGap)
			randomBoundaries = applyMinGap(randomBoundaries, *flBenchMinGap)
		}

		if stage.raw {
			if err := validateBoundaries("heading-split", headingBoundaries, numBlocks); err != nil {
				return err
			}
			report.Methods = append(report.Methods,
				eval.Evaluate(gold, numBlocks, headingBoundaries, methodName("heading-split", "", "raw", includeStage), *flBenchK, *flBenchTolerance))

			if err := validateBoundaries("fixed-split", fixedBoundaries, numBlocks); err != nil {
				return err
			}
			report.Methods = append(report.Methods,
				eval.Evaluate(gold, numBlocks, fixedBoundaries, methodName(fmt.Sprintf("fixed-%d", avgGoldSegLen), "", "raw", includeStage), *flBenchK, *flBenchTolerance))

			if err := validateBoundaries("random-split", randomBoundaries, numBlocks); err != nil {
				return err
			}
			report.Methods = append(report.Methods,
				eval.Evaluate(gold, numBlocks, randomBoundaries, methodName(fmt.Sprintf("random (n=%d)", len(gold.Boundaries)), "", "raw", includeStage), *flBenchK, *flBenchTolerance))
		}

		if stage.final {
			depthScores := result.Boundary.DepthScores
			if len(depthScores) != numBlocks-1 {
				depthScores = computeDepthScores(result.Embeddings, *flBenchWindow, *flBenchBlockK)
			}

			effectiveMaxLines := *flBenchMaxLines
			if effectiveMaxLines < 0 {
				totalLines := 0
				for _, b := range blocks {
					totalLines += b.LinesApprox
				}
				effectiveMaxLines = segment.AutoMaxLines(totalLines)
			}

			lockRanges := computeLockRanges(blocks, *flBenchSectionLock, *flBenchLockAfterHeading, *flBenchAtomicBoundary)
			optCfg := segment.OptConfig{
				MinTokens:          *flBenchMinTokens,
				MaxTokens:          effectiveMaxTokens,
				MaxLines:           effectiveMaxLines,
				LockRanges:         lockRanges,
				PackHeadingBarrier: *flBenchPackHeadingBarrier,
			}

			headingFinal, err := optimizeBoundaries(blocks, depthScores, headingBoundaries, optCfg)
			if err != nil {
				return fmt.Errorf("heading-split final: %w", err)
			}
			if err := validateBoundaries("heading-split final", headingFinal, numBlocks); err != nil {
				return err
			}
			report.Methods = append(report.Methods,
				eval.Evaluate(gold, numBlocks, headingFinal, methodName("heading-split", "", "final", includeStage), *flBenchK, *flBenchTolerance))

			fixedFinal, err := optimizeBoundaries(blocks, depthScores, fixedBoundaries, optCfg)
			if err != nil {
				return fmt.Errorf("fixed-split final: %w", err)
			}
			if err := validateBoundaries("fixed-split final", fixedFinal, numBlocks); err != nil {
				return err
			}
			report.Methods = append(report.Methods,
				eval.Evaluate(gold, numBlocks, fixedFinal, methodName(fmt.Sprintf("fixed-%d", avgGoldSegLen), "", "final", includeStage), *flBenchK, *flBenchTolerance))

			randomFinal, err := optimizeBoundaries(blocks, depthScores, randomBoundaries, optCfg)
			if err != nil {
				return fmt.Errorf("random-split final: %w", err)
			}
			if err := validateBoundaries("random-split final", randomFinal, numBlocks); err != nil {
				return err
			}
			report.Methods = append(report.Methods,
				eval.Evaluate(gold, numBlocks, randomFinal, methodName(fmt.Sprintf("random (n=%d)", len(gold.Boundaries)), "", "final", includeStage), *flBenchK, *flBenchTolerance))
		}
	}

	// Output
	if *flBenchJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}

	fmt.Print(eval.FormatTable(report))
	return nil
}

type benchEvalStage struct {
	raw   bool
	final bool
}

func parseEvalStage(value string) (benchEvalStage, error) {
	switch strings.ToLower(value) {
	case "raw":
		return benchEvalStage{raw: true}, nil
	case "final":
		return benchEvalStage{final: true}, nil
	case "both":
		return benchEvalStage{raw: true, final: true}, nil
	default:
		return benchEvalStage{}, fmt.Errorf("invalid eval-stage %q (expected raw|final|both)", value)
	}
}

func methodName(base, embedder, stage string, includeStage bool) string {
	if includeStage {
		if embedder != "" {
			return fmt.Sprintf("%s (%s,%s)", base, embedder, stage)
		}
		return fmt.Sprintf("%s (%s)", base, stage)
	}
	if embedder != "" {
		return fmt.Sprintf("%s (%s)", base, embedder)
	}
	return base
}

func applyBenchProfile(cmd *cobra.Command, profile string) error {
	if profile == "" {
		return nil
	}
	switch strings.ToLower(profile) {
	case "embedder":
		return applyEmbedderProfile(cmd)
	case "output":
		return applyOutputProfile(cmd)
	default:
		return fmt.Errorf("invalid profile %q (expected embedder|output)", profile)
	}
}

func applyEmbedderProfile(cmd *cobra.Command) error {
	if err := setIfNotChanged(cmd, "eval-stage", "raw"); err != nil {
		return err
	}
	if err := setIfNotChanged(cmd, "tolerance", "0"); err != nil {
		return err
	}
	if err := setIfNotChanged(cmd, "min-gap", "2"); err != nil {
		return err
	}
	if err := setIfNotChanged(cmd, "window", "5"); err != nil {
		return err
	}
	if err := setIfNotChanged(cmd, "k", "3"); err != nil {
		return err
	}
	if err := setIfNotChanged(cmd, "boundary-hints", "false"); err != nil {
		return err
	}
	if err := setIfNotChanged(cmd, "section-lock", "false"); err != nil {
		return err
	}
	if err := setIfNotChanged(cmd, "suppress-list-boundary", "false"); err != nil {
		return err
	}
	if err := setIfNotChanged(cmd, "atomic-boundary", "false"); err != nil {
		return err
	}
	if err := setIfNotChanged(cmd, "pseudo-heading", "false"); err != nil {
		return err
	}
	if err := setIfNotChanged(cmd, "baseline-min-gap", "false"); err != nil {
		return err
	}
	if err := setIfNotChanged(cmd, "block-k", "3"); err != nil {
		return err
	}
	return nil
}

func applyOutputProfile(cmd *cobra.Command) error {
	if err := setIfNotChanged(cmd, "eval-stage", "final"); err != nil {
		return err
	}
	if err := setIfNotChanged(cmd, "max-tokens", "800"); err != nil {
		return err
	}
	return nil
}

func applyGoldParams(cmd *cobra.Command, params *eval.GoldParams) error {
	if params == nil {
		return nil
	}
	if params.EvalStage != "" {
		if err := setIfNotChanged(cmd, "eval-stage", params.EvalStage); err != nil {
			return err
		}
	}
	if params.Tolerance != nil {
		if err := setIfNotChanged(cmd, "tolerance", fmt.Sprintf("%d", *params.Tolerance)); err != nil {
			return err
		}
	}
	if params.K != nil {
		if err := setIfNotChanged(cmd, "k", fmt.Sprintf("%d", *params.K)); err != nil {
			return err
		}
	}
	if params.MinGap != nil {
		if err := setIfNotChanged(cmd, "min-gap", fmt.Sprintf("%d", *params.MinGap)); err != nil {
			return err
		}
	}
	if params.MinTokens != nil {
		if err := setIfNotChanged(cmd, "min-tokens", fmt.Sprintf("%d", *params.MinTokens)); err != nil {
			return err
		}
	}
	if params.MaxTokens != nil {
		if err := setIfNotChanged(cmd, "max-tokens", fmt.Sprintf("%d", *params.MaxTokens)); err != nil {
			return err
		}
	}
	if params.MaxLines != nil {
		if err := setIfNotChanged(cmd, "max-lines", fmt.Sprintf("%d", *params.MaxLines)); err != nil {
			return err
		}
	}
	if params.Window != nil {
		if err := setIfNotChanged(cmd, "window", fmt.Sprintf("%d", *params.Window)); err != nil {
			return err
		}
	}
	if params.BlockK != nil {
		if err := setIfNotChanged(cmd, "block-k", fmt.Sprintf("%d", *params.BlockK)); err != nil {
			return err
		}
	}
	if params.Threshold != nil {
		if err := setIfNotChanged(cmd, "threshold", fmt.Sprintf("%f", *params.Threshold)); err != nil {
			return err
		}
	}
	if params.SplitCount != nil {
		if err := setIfNotChanged(cmd, "split-count", fmt.Sprintf("%d", *params.SplitCount)); err != nil {
			return err
		}
	}
	if params.PackHeadingBarrier != nil {
		if err := setIfNotChanged(cmd, "pack-heading-barrier", fmt.Sprintf("%d", *params.PackHeadingBarrier)); err != nil {
			return err
		}
	}
	if params.PseudoHeading != nil {
		if err := setIfNotChanged(cmd, "pseudo-heading", fmt.Sprintf("%t", *params.PseudoHeading)); err != nil {
			return err
		}
	}
	if params.BoundaryHints != nil {
		if err := setIfNotChanged(cmd, "boundary-hints", fmt.Sprintf("%t", *params.BoundaryHints)); err != nil {
			return err
		}
	}
	if params.SectionLock != nil {
		if err := setIfNotChanged(cmd, "section-lock", fmt.Sprintf("%t", *params.SectionLock)); err != nil {
			return err
		}
	}
	if params.LockAfterHeading != nil {
		if err := setIfNotChanged(cmd, "lock-after-heading", fmt.Sprintf("%t", *params.LockAfterHeading)); err != nil {
			return err
		}
	}
	if params.ParaSplitPattern != nil {
		if err := setIfNotChanged(cmd, "para-split-pattern", *params.ParaSplitPattern); err != nil {
			return err
		}
	}
	if params.ForceBoundary != nil {
		if err := setIfNotChanged(cmd, "force-boundary", *params.ForceBoundary); err != nil {
			return err
		}
	}
	if params.SuppressListBoundary != nil {
		if err := setIfNotChanged(cmd, "suppress-list-boundary", fmt.Sprintf("%t", *params.SuppressListBoundary)); err != nil {
			return err
		}
	}
	if params.AtomicBoundary != nil {
		if err := setIfNotChanged(cmd, "atomic-boundary", fmt.Sprintf("%t", *params.AtomicBoundary)); err != nil {
			return err
		}
	}
	if params.BaselineMinGap != nil {
		if err := setIfNotChanged(cmd, "baseline-min-gap", fmt.Sprintf("%t", *params.BaselineMinGap)); err != nil {
			return err
		}
	}
	if params.AllowBlockMismatch != nil {
		if err := setIfNotChanged(cmd, "allow-block-mismatch", fmt.Sprintf("%t", *params.AllowBlockMismatch)); err != nil {
			return err
		}
	}
	return nil
}

func setIfNotChanged(cmd *cobra.Command, name, value string) error {
	if flagChanged(cmd, name) {
		return nil
	}
	return cmd.Flags().Set(name, value)
}

func flagChanged(cmd *cobra.Command, name string) bool {
	if cmd.Flags().Changed(name) {
		return true
	}
	noName := "no-" + name
	if cmd.Flags().Lookup(noName) != nil && cmd.Flags().Changed(noName) {
		return true
	}
	return false
}

func validateGold(ann *eval.GoldAnnotation, numBlocks int) error {
	if ann.Unit != "" && ann.Unit != "block" {
		return fmt.Errorf("unsupported gold unit %q (expected \"block\")", ann.Unit)
	}
	return validateBoundaries("gold", ann.Boundaries, numBlocks)
}

func validateBoundaries(label string, boundaries []int, numBlocks int) error {
	if len(boundaries) == 0 {
		return nil
	}
	if numBlocks < 2 {
		return fmt.Errorf("%s has boundaries but document has %d blocks", label, numBlocks)
	}
	max := numBlocks - 2
	prev := -1
	for i, b := range boundaries {
		if b < 0 || b > max {
			return fmt.Errorf("%s boundary %d out of range [0,%d]", label, b, max)
		}
		if b <= prev {
			return fmt.Errorf("%s boundaries not strictly increasing at index %d (%d <= %d)", label, i, b, prev)
		}
		prev = b
	}
	return nil
}

func boundariesFromSegments(segments []model.Segment, blocks []model.Block) ([]int, error) {
	if len(segments) == 0 {
		return nil, nil
	}
	if len(blocks) == 0 {
		return nil, fmt.Errorf("no blocks to map segments")
	}

	idxByStart := model.BlockStartIndex(blocks)
	expectedStart := 0
	boundaries := make([]int, 0, len(segments)-1)

	for i, seg := range segments {
		if len(seg.Blocks) == 0 {
			return nil, fmt.Errorf("segment %d has no blocks", i)
		}
		startIdx, ok := idxByStart[seg.Blocks[0].Range.Start]
		if !ok {
			return nil, fmt.Errorf("segment %d start block not found in block list", i)
		}
		endIdx, ok := idxByStart[seg.Blocks[len(seg.Blocks)-1].Range.Start]
		if !ok {
			return nil, fmt.Errorf("segment %d end block not found in block list", i)
		}
		if startIdx != expectedStart {
			return nil, fmt.Errorf("segment %d starts at block %d; expected %d", i, startIdx, expectedStart)
		}
		if endIdx < startIdx {
			return nil, fmt.Errorf("segment %d ends before it starts", i)
		}
		if i < len(segments)-1 {
			boundaries = append(boundaries, endIdx)
		}
		expectedStart = endIdx + 1
	}
	if expectedStart != len(blocks) {
		return nil, fmt.Errorf("segments cover %d/%d blocks", expectedStart, len(blocks))
	}
	return boundaries, nil
}

func applyMinGap(boundaries []int, minGap int) []int {
	if minGap <= 1 || len(boundaries) <= 1 {
		return boundaries
	}
	filtered := make([]int, 0, len(boundaries))
	last := -minGap
	for _, b := range boundaries {
		if len(filtered) == 0 || b-last >= minGap {
			filtered = append(filtered, b)
			last = b
		}
	}
	return filtered
}

func maxBoundariesWithMinGap(boundaries []int, minGap int) int {
	if minGap <= 1 || len(boundaries) == 0 {
		return len(boundaries)
	}
	count := 0
	last := -minGap
	for _, b := range boundaries {
		if count == 0 || b-last >= minGap {
			count++
			last = b
		}
	}
	return count
}

func warnMinGapCeiling(cmd *cobra.Command, boundaries []int, minGap int) {
	if minGap <= 1 || len(boundaries) == 0 {
		return
	}
	maxCount := maxBoundariesWithMinGap(boundaries, minGap)
	if maxCount >= len(boundaries) {
		return
	}
	maxRecall := float64(maxCount) / float64(len(boundaries))
	maxF1 := 0.0
	if maxRecall > 0 {
		maxF1 = 2 * maxRecall / (1 + maxRecall)
	}
	cmd.PrintErrf("WARNING: min-gap=%d prevents matching all gold boundaries (%d/%d). Max recall=%.2f, max F1≈%.2f when precision=1.\n",
		minGap, maxCount, len(boundaries), maxRecall, maxF1)
}

func warnAutoK(cmd *cobra.Command, boundaries []int, numBlocks, k int) {
	if numBlocks < 2 || k != 0 {
		return
	}
	numSegs := len(boundaries) + 1
	avgLen := float64(numBlocks) / float64(numSegs)
	kAuto := int(avgLen / 2)
	if kAuto < 1 {
		kAuto = 1
	}
	if kAuto <= 1 {
		cmd.PrintErrf("WARNING: auto-k=%d (avg segment length %.1f). Pk/WindowDiff may be weakly discriminative; consider --k.\n", kAuto, avgLen)
	}
}

func warnSmallDoc(cmd *cobra.Command, numBlocks int) {
	if numBlocks > 0 && numBlocks < 50 {
		cmd.PrintErrf("WARNING: only %d blocks; results may be noisy. Consider longer or multiple documents.\n", numBlocks)
	}
}

func computeDepthScores(embeddings []model.Embedding, window, blockK int) []float64 {
	if len(embeddings) < 2 {
		return nil
	}
	sims := boundary.BlockSimilarities(embeddings, blockK)
	smoothed := boundary.Smooth(sims, window)
	return boundary.DepthScore(smoothed)
}

func computeLockRanges(blocks []model.Block, sectionLock, lockAfterHeading, atomicBoundary bool) []segment.LockRange {
	var ranges []segment.LockRange
	if sectionLock {
		ranges = segment.DetectLockRanges(blocks, buildLockConfig(true, lockAfterHeading))
	}
	if atomicBoundary {
		for i := 1; i < len(blocks); i++ {
			k := blocks[i].Kind
			if k == model.BlockCodeBlock || k == model.BlockTable || k == model.BlockMathBlock {
				gapIdx := i - 1
				ranges = append(ranges, segment.LockRange{Start: gapIdx, End: gapIdx + 2})
			}
		}
	}
	return ranges
}

func optimizeBoundaries(blocks []model.Block, depthScores []float64, boundaries []int, cfg segment.OptConfig) ([]int, error) {
	br := boundary.BoundaryResult{
		Boundaries:  boundaries,
		DepthScores: depthScores,
	}
	segments := segment.Optimize(blocks, br, cfg)
	return boundariesFromSegments(segments, blocks)
}
