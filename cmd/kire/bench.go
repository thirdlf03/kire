package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/thirdlf03/kire/internal/embedding"
	"github.com/thirdlf03/kire/internal/eval"
	"github.com/thirdlf03/kire/internal/llmsplit"
	"github.com/thirdlf03/kire/internal/model"
	"github.com/thirdlf03/kire/internal/parser"
	"github.com/thirdlf03/kire/internal/pipeline"
	"github.com/thirdlf03/kire/internal/tokenizer"
	"google.golang.org/genai"
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
  kire bench --llm-refine --embedder gemini testdata/gold/bench_xl.json testdata/bench_xl.md`,
	Args: cobra.ExactArgs(2),
	RunE: runBench,
}

var (
	flBenchJSON        *bool
	flBenchEmbedder    *string
	flBenchEmbedModel  *string
	flBenchBatchSize   *int
	flBenchTolerance   *int
	flBenchK           *int
	flBenchEvalStage   *string
	flBenchNoBaselines *bool
	flBenchLLMModel    *string
	flBenchLLMRefine   *bool
)

func init() {
	f := benchCmd.Flags()
	flBenchJSON = f.Bool("json", false, "Output JSON report to stdout")
	flBenchEmbedder = f.String("embedder", "tfidf", "Embedder provider (for --llm-refine): "+strings.Join(embedding.List(), "|"))
	flBenchEmbedModel = f.String("embed-model", "", "Embedding model name (provider default if empty)")
	flBenchBatchSize = f.Int("batch-size", 32, "Batch size for embedding API")
	flBenchTolerance = f.Int("tolerance", 1, "Boundary matching tolerance (blocks)")
	flBenchK = f.Int("k", 0, "Pk/WindowDiff window half-width (0 = auto)")
	flBenchEvalStage = f.String("eval-stage", "final", "Evaluation stage: raw|final|both")
	flBenchNoBaselines = f.Bool("no-baselines", false, "Skip baseline comparisons")
	flBenchLLMModel = f.String("llm-model", "gemini-2.5-flash-lite", "LLM model for boundary detection")
	flBenchLLMRefine = f.Bool("llm-refine", false, "Enable embedding + cosine similarity refinement for LLM boundary detection")

	_ = benchCmd.RegisterFlagCompletionFunc("embedder", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return embedding.List(), cobra.ShellCompDirectiveNoFileComp
	})
	_ = benchCmd.RegisterFlagCompletionFunc("eval-stage", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"raw", "final", "both"}, cobra.ShellCompDirectiveNoFileComp
	})

	rootCmd.AddCommand(benchCmd)
}

func runBench(cmd *cobra.Command, args []string) error {
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

	// Build LLM splitter
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("GEMINI_API_KEY is required for LLM boundary detection")
	}
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return fmt.Errorf("create LLM client: %w", err)
	}
	llmDetector := llmsplit.New(client, llmsplit.Config{
		Model: *flBenchLLMModel,
	})

	// Build embedder only when --llm-refine is enabled
	var embedder embedding.Embedder
	if *flBenchLLMRefine {
		var err error
		embedder, _, err = embedding.Create(ctx, *flBenchEmbedder, embedding.ProviderConfig{
			Model:     *flBenchEmbedModel,
			BatchSize: *flBenchBatchSize,
		})
		if err != nil {
			return fmt.Errorf("create embedder %s: %w", *flBenchEmbedder, err)
		}
	}

	estimator := tokenizer.NewLocalEstimator()

	pipeCfg := pipeline.Config{
		Source:           source,
		SourceName:       filepath.Base(inputPath),
		Embedder:         embedder,
		TokenEstimator:   estimator,
		BoundaryDetector: llmDetector,
		LLMRefine:        *flBenchLLMRefine,
		EmbedTaskType:    "SEMANTIC_SIMILARITY",
	}

	result, err := pipeline.Run(ctx, pipeCfg)
	if err != nil {
		return fmt.Errorf("kire pipeline: %w", err)
	}

	blocks := result.Blocks
	numBlocks := len(blocks)
	if len(baseBlocks) != numBlocks {
		cmd.PrintErrf("WARNING: block count changed after preprocessing: parse=%d pipeline=%d\n", len(baseBlocks), numBlocks)
	}

	if err := validateGold(gold, numBlocks); err != nil {
		return err
	}

	warnSmallDoc(cmd, numBlocks)
	warnAutoK(cmd, gold.Boundaries, numBlocks, *flBenchK)

	report := eval.EvalReport{
		DocID:     gold.DocID,
		NumBlocks: numBlocks,
		Gold:      *gold,
	}

	includeStage := !stage.raw || stage.final

	benchLabel := "llm"
	if *flBenchLLMRefine {
		benchLabel = fmt.Sprintf("llm-refine(%s)", *flBenchEmbedder)
	}

	if stage.raw {
		if err := validateBoundaries("kire raw", result.Boundary.Boundaries, numBlocks); err != nil {
			return err
		}
		name := methodName("kire", benchLabel, "raw", includeStage)
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
		name := methodName("kire", benchLabel, "final", includeStage)
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
			// For baselines, raw=final since there's no optimizer step
			if err := validateBoundaries("heading-split final", headingBoundaries, numBlocks); err != nil {
				return err
			}
			report.Methods = append(report.Methods,
				eval.Evaluate(gold, numBlocks, headingBoundaries, methodName("heading-split", "", "final", includeStage), *flBenchK, *flBenchTolerance))

			if err := validateBoundaries("fixed-split final", fixedBoundaries, numBlocks); err != nil {
				return err
			}
			report.Methods = append(report.Methods,
				eval.Evaluate(gold, numBlocks, fixedBoundaries, methodName(fmt.Sprintf("fixed-%d", avgGoldSegLen), "", "final", includeStage), *flBenchK, *flBenchTolerance))

			if err := validateBoundaries("random-split final", randomBoundaries, numBlocks); err != nil {
				return err
			}
			report.Methods = append(report.Methods,
				eval.Evaluate(gold, numBlocks, randomBoundaries, methodName(fmt.Sprintf("random (n=%d)", len(gold.Boundaries)), "", "final", includeStage), *flBenchK, *flBenchTolerance))
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
	return nil
}

func setIfNotChanged(cmd *cobra.Command, name, value string) error {
	if cmd.Flags().Changed(name) {
		return nil
	}
	return cmd.Flags().Set(name, value)
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
