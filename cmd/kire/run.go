package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/thirdlf03/kire/internal/cache"
	"github.com/thirdlf03/kire/internal/dag"
	"github.com/thirdlf03/kire/internal/embedding"
	"github.com/thirdlf03/kire/internal/llmsplit"
	"github.com/thirdlf03/kire/internal/output"
	"github.com/thirdlf03/kire/internal/pipeline"
	"github.com/thirdlf03/kire/internal/tokenizer"
	"google.golang.org/genai"
)

// isTerminal reports whether stdin is a terminal (overridable for testing).
var isTerminal = func() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func runSplitter(cmd *cobra.Command, args []string) error {
	cfg := populateConfig(cmd)
	setupLogger(cfg.IO.Quiet, cfg.IO.LogFormat)

	if cfg.Output.ShowVersion {
		fmt.Printf("kire %s\n", version)
		return nil
	}

	// Positional arguments: [input-file] [output-dir]
	inPaths := cfg.IO.InPaths
	if len(args) >= 1 && len(inPaths) == 0 {
		inPaths = append(inPaths, args[0])
	}
	outDir := cfg.IO.OutDir
	if len(args) >= 2 && !cmd.Flags().Changed("out") {
		outDir = args[1]
	}

	if len(inPaths) == 0 {
		return fmt.Errorf("input file is required")
	}

	ctx := context.Background()

	// Build LLM splitter (always required)
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
		Model: cfg.Segment.LLMModel,
	})
	if cfg.IO.Debug {
		log.Printf("LLM splitter: model=%s", cfg.Segment.LLMModel)
	}

	// Build embedder only when --llm-refine is enabled
	var embedder embedding.Embedder
	var embedInfo, embedName string
	var closer io.Closer
	if cfg.Segment.LLMRefine {
		var err error
		embedder, embedInfo, embedName, closer, err = buildEmbedder(ctx, cfg.Embed.Embedder, cfg.Embed.EmbedModel, cfg.Embed.BatchSize, cfg.Embed.Concurrency, cfg.Embed.QPS, cfg.Embed.CachePath)
		if err != nil {
			return fmt.Errorf("build embedder: %w", err)
		}
		if closer != nil {
			defer func() { _ = closer.Close() }()
		}
		if cfg.IO.Debug {
			log.Printf("Embedder (for LLM-refine): %s", embedInfo)
		}
	}

	// Build token estimator (shared across all input files)
	estimator := tokenizer.NewLocalEstimator()

	opts := ProcessOptions{
		CLIConfig:   cfg,
		Embedder:    embedder,
		EmbedInfo:   embedInfo,
		EmbedName:   embedName,
		Estimator:   estimator,
		LLMDetector: llmDetector,
	}

	// Warn if --dag-json / --dag-dot specified with multiple files
	multiFile := len(inPaths) > 1
	if multiFile {
		if cfg.Output.DagJSON != "" {
			log.Println("WARNING: --dag-json is ignored with multiple input files; DAG JSON will be written per subdirectory")
		}
		if cfg.Output.DagDOT != "" {
			log.Println("WARNING: --dag-dot is ignored with multiple input files; DAG DOT will be written per subdirectory")
		}
	}

	// Process each input file
	for _, inFile := range inPaths {
		if err := processFile(ctx, inFile, outDir, multiFile, opts); err != nil {
			return err
		}
	}

	return nil
}

func processFile(ctx context.Context, inFile, outDir string, multiFile bool, opts ProcessOptions) error {
	source, err := os.ReadFile(inFile)
	if err != nil {
		return fmt.Errorf("read input %s: %w", inFile, err)
	}

	cfg := opts.CLIConfig

	// Determine output subdirectory: outDir/basename(without ext)
	base := strings.TrimSuffix(filepath.Base(inFile), filepath.Ext(inFile))
	fileOutDir := filepath.Join(outDir, base)

	// Check if output directory already exists (skip entirely in dry-run mode)
	if !cfg.IO.DryRun {
		if info, err := os.Stat(fileOutDir); err == nil && info.IsDir() {
			if !cfg.IO.Force {
				if !isTerminal() {
					return fmt.Errorf("output directory %s already exists; use --force to overwrite (non-interactive mode)", fileOutDir)
				}
				fmt.Fprintf(os.Stderr, "Output directory %s already exists. Overwrite? [y/N] ", fileOutDir)
				scanner := bufio.NewScanner(os.Stdin)
				scanner.Scan()
				answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
				if answer != "y" && answer != "yes" {
					log.Printf("Skipped %s", inFile)
					return nil
				}
			}
			absOut, absErr := filepath.Abs(outDir)
			absFile, fileErr := filepath.Abs(fileOutDir)
			if absErr != nil || fileErr != nil || !strings.HasPrefix(absFile, absOut+string(filepath.Separator)) {
				return fmt.Errorf("refusing to remove %s: not inside output dir %s", fileOutDir, outDir)
			}
			if err := os.RemoveAll(fileOutDir); err != nil {
				return fmt.Errorf("remove existing output directory %s: %w", fileOutDir, err)
			}
		}
	}

	// Run pipeline
	pipeCfg := pipeline.Config{
		Source:           source,
		SourceName:       filepath.Base(inFile),
		Embedder:         opts.Embedder,
		TokenEstimator:   opts.Estimator,
		BoundaryDetector: opts.LLMDetector,
		LLMRefine:        cfg.Segment.LLMRefine,
		OverlapLines:     cfg.Segment.OverlapLines,
		ContextFormat:    cfg.Segment.ContextFormat,
		ContextMaxDepth:  cfg.Segment.ContextMaxDepth,
		EmbedTaskType:    "SEMANTIC_SIMILARITY",
	}

	result, err := pipeline.Run(ctx, pipeCfg)
	if err != nil {
		return fmt.Errorf("pipeline (%s): %w", inFile, err)
	}

	// Incremental processing: detect changes and save state
	if cfg.IO.StateFile != "" {
		prevState, err := pipeline.LoadState(cfg.IO.StateFile)
		if err != nil {
			log.Printf("WARNING: failed to load state file: %v", err)
		}
		if prevState != nil && prevState.SourceHash == pipeline.SourceHash(source) {
			log.Printf("Source unchanged (hash match), skipping %s", inFile)
			return nil
		}
		changes := pipeline.DetectChanges(prevState, result.Segments, source)
		if prevState != nil {
			log.Printf("Changes: %d added, %d removed, %d modified, %d unchanged",
				len(changes.Added), len(changes.Removed), len(changes.Modified), len(changes.Unchanged))
		}
		newState := pipeline.BuildState(source, result.Segments)
		if err := pipeline.SaveState(cfg.IO.StateFile, newState); err != nil {
			return fmt.Errorf("save state (%s): %w", inFile, err)
		}
	}

	// Generate filenames (always needed for --json summary)
	var filenames []string
	if cfg.IO.Prefix != "" {
		for i := range result.Rendered {
			filenames = append(filenames, fmt.Sprintf("%s%d.md", cfg.IO.Prefix, i+1))
		}
	} else {
		filenames = output.GenerateFilenames(result.Segments, source)
	}

	// JSONL output (works in both normal and dry-run mode)
	if cfg.Output.JSONL != "" {
		if err := writeJSONLOutput(inFile, fileOutDir, result, filenames, source, cfg.Output); err != nil {
			return fmt.Errorf("write JSONL (%s): %w", inFile, err)
		}
	}

	// Write files (skip in dry-run mode)
	if !cfg.IO.DryRun {
		if cfg.IO.Prefix != "" {
			if err := output.WriteFiles(fileOutDir, cfg.IO.Prefix, result.Rendered); err != nil {
				return fmt.Errorf("write output (%s): %w", inFile, err)
			}
		} else {
			if err := output.WriteNamedFiles(fileOutDir, filenames, result.Rendered); err != nil {
				return fmt.Errorf("write output (%s): %w", inFile, err)
			}
		}

		log.Printf("Wrote %d segments to %s/", len(result.Rendered), fileOutDir)

		// DAG export + index.md
		d := dag.Build(result.Segments, source)

		if !cfg.Output.NoIndex {
			indexPath := filepath.Join(fileOutDir, "index.md")
			if err := os.WriteFile(indexPath, []byte(d.ExportIndexMarkdown(filenames)), 0644); err != nil {
				return fmt.Errorf("write index.md (%s): %w", inFile, err)
			}
			log.Printf("Wrote %s", indexPath)
		}

		// DAG JSON/DOT: use explicit flag path for single file, auto-place in subdirectory for multiple
		dagJSONPath := cfg.Output.DagJSON
		dagDOTPath := cfg.Output.DagDOT
		if multiFile {
			dagJSONPath = filepath.Join(fileOutDir, "dag.json")
			dagDOTPath = filepath.Join(fileOutDir, "dag.dot")
		}

		if dagJSONPath != "" {
			data, err := d.ExportJSON()
			if err != nil {
				return fmt.Errorf("export DAG JSON (%s): %w", inFile, err)
			}
			if err := os.WriteFile(dagJSONPath, data, 0644); err != nil {
				return fmt.Errorf("write DAG JSON (%s): %w", inFile, err)
			}
			log.Printf("Wrote DAG JSON to %s", dagJSONPath)
		}
		if dagDOTPath != "" {
			if err := os.WriteFile(dagDOTPath, []byte(d.ExportDOT()), 0644); err != nil {
				return fmt.Errorf("write DAG DOT (%s): %w", inFile, err)
			}
			log.Printf("Wrote DAG DOT to %s", dagDOTPath)
		}
	} else {
		// dry-run: print summary to stderr (unless --json is used)
		if !cfg.Output.JSON {
			fmt.Fprintf(os.Stderr, "[dry-run] %s: %d segments → %s/\n", inFile, len(result.Rendered), fileOutDir)
			for _, fn := range filenames {
				fmt.Fprintf(os.Stderr, "  %s\n", fn)
			}
		}
	}

	// JSON summary output
	if cfg.Output.JSON {
		summary := buildSummary(inFile, fileOutDir, result, filenames, source, opts.EmbedInfo, opts.EmbedName, cfg)
		if err := writeSummaryJSON(os.Stdout, summary); err != nil {
			return fmt.Errorf("write JSON summary (%s): %w", inFile, err)
		}
	}

	return nil
}

func writeJSONLOutput(inputPath, outDir string, result *pipeline.Result, filenames []string, source []byte, outCfg OutputConfig) error {
	records := make([]output.JSONLRecord, len(result.Segments))
	for i, seg := range result.Segments {
		content := string(source[seg.Range.Start:seg.Range.End])
		headingPath := seg.HeadingPath
		if headingPath == nil {
			headingPath = []string{}
		}

		rec := output.JSONLRecord{
			Content: content,
			Metadata: output.JSONLMetadata{
				Source:       filepath.Base(inputPath),
				SegmentIndex: i,
				Filename:     filenames[i],
				ByteRange:    [2]int{seg.Range.Start, seg.Range.End},
				HeadingPath:  headingPath,
				TokenCount:   seg.TokenCount,
				BlockCount:   len(seg.Blocks),
			},
		}
		if outCfg.AgentMetadata {
			rec.Metadata.SegmentID = seg.ID
			rec.Metadata.PrevSegmentID = seg.PrevSegmentID
			rec.Metadata.NextSegmentID = seg.NextSegmentID
			coh := seg.Coherence
			rec.Metadata.Coherence = &coh
			// Boundary confidence: use the confidence for boundary that starts this segment
			if i > 0 && i-1 < len(result.Boundary.Confidences) {
				conf := result.Boundary.Confidences[i-1]
				rec.Metadata.BoundaryConfidence = &conf
			}
		}
		records[i] = rec
	}

	if outCfg.JSONL == "-" {
		return output.WriteJSONL(os.Stdout, records)
	}

	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	jsonlPath := filepath.Join(outDir, "metadata.jsonl")
	if outCfg.JSONL != "auto" && outCfg.JSONL != "" {
		jsonlPath = outCfg.JSONL
	}
	f, err := os.Create(jsonlPath)
	if err != nil {
		return fmt.Errorf("create JSONL file: %w", err)
	}
	defer func() { _ = f.Close() }()
	if err := output.WriteJSONL(f, records); err != nil {
		return err
	}
	log.Printf("Wrote JSONL to %s", jsonlPath)
	return nil
}

func buildEmbedder(ctx context.Context, embedderType, model string, batchSize, concurrency int, qps float64, cachePath string) (embedding.Embedder, string, string, io.Closer, error) {
	cfg := embedding.ProviderConfig{
		Model:     model,
		BatchSize: batchSize,
	}

	var base embedding.Embedder
	baseInfo := ""
	resolvedName := ""

	if embedderType == "auto" {
		// auto: try gemini first, fall back to tfidf
		var err error
		base, baseInfo, err = embedding.Create(ctx, "gemini", cfg)
		if err != nil {
			log.Println("No GEMINI_API_KEY set, using TF-IDF embedder")
			base, baseInfo, err = embedding.Create(ctx, "tfidf", cfg)
			if err != nil {
				return nil, "", "", nil, fmt.Errorf("create tfidf embedder: %w", err)
			}
			resolvedName = "tfidf"
		} else {
			resolvedName = "gemini"
		}
	} else {
		var err error
		base, baseInfo, err = embedding.Create(ctx, embedderType, cfg)
		if err != nil {
			return nil, "", "", nil, fmt.Errorf("--embedder=%s: %w", embedderType, err)
		}
		resolvedName = embedderType
	}

	infoParts := []string{baseInfo}

	// Wrap with cache if configured
	var closer io.Closer
	if cachePath != "" {
		c, err := cache.NewJSONCache(cachePath)
		if err != nil {
			return nil, "", "", nil, fmt.Errorf("create cache: %w", err)
		}
		cached := embedding.NewCachedEmbedder(base, c, model)
		base = cached
		closer = cached
		infoParts = append(infoParts, fmt.Sprintf("cache=%s", cachePath))
	}

	// Wrap with concurrency if > 1
	if concurrency > 1 {
		base = embedding.NewConcurrentEmbedder(base, concurrency, qps, batchSize)
		if qps > 0 {
			infoParts = append(infoParts, fmt.Sprintf("concurrency=%d,qps=%.2f", concurrency, qps))
		} else {
			infoParts = append(infoParts, fmt.Sprintf("concurrency=%d", concurrency))
		}
	}

	return base, strings.Join(infoParts, " | "), resolvedName, closer, nil
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
