package main

import (
	"bytes"
	"slices"
	"testing"

	"github.com/spf13/cobra"

	"github.com/thirdlf03/kire/internal/eval"
	"github.com/thirdlf03/kire/internal/model"
	"github.com/thirdlf03/kire/internal/segment"
)

func makeBlock(start int) model.Block {
	return model.Block{
		Range: model.SourceRange{Start: start, End: start + 1},
	}
}

// ---------------------------------------------------------------------------
// boundariesFromSegments
// ---------------------------------------------------------------------------

func TestBoundariesFromSegments(t *testing.T) {
	blocks := []model.Block{
		makeBlock(0),
		makeBlock(1),
		makeBlock(2),
		makeBlock(3),
	}
	segments := []model.Segment{
		{Blocks: []model.Block{blocks[0], blocks[1]}},
		{Blocks: []model.Block{blocks[2], blocks[3]}},
	}
	got, err := boundariesFromSegments(segments, blocks)
	if err != nil {
		t.Fatalf("boundariesFromSegments error: %v", err)
	}
	want := []int{1}
	if !slices.Equal(got, want) {
		t.Fatalf("boundariesFromSegments: got %v, want %v", got, want)
	}
}

func TestBoundariesFromSegments_Mismatch(t *testing.T) {
	blocks := []model.Block{
		makeBlock(0),
		makeBlock(1),
		makeBlock(2),
	}
	segments := []model.Segment{
		{Blocks: []model.Block{blocks[1], blocks[2]}},
	}
	if _, err := boundariesFromSegments(segments, blocks); err == nil {
		t.Fatal("expected error for mismatched segment start")
	}
}

// ---------------------------------------------------------------------------
// applyMinGap / maxBoundariesWithMinGap
// ---------------------------------------------------------------------------

func TestApplyMinGap(t *testing.T) {
	boundaries := []int{1, 2, 4, 7}
	got := applyMinGap(boundaries, 3)
	want := []int{1, 4, 7}
	if !slices.Equal(got, want) {
		t.Fatalf("applyMinGap: got %v, want %v", got, want)
	}
}

func TestMaxBoundariesWithMinGap(t *testing.T) {
	boundaries := []int{1, 2, 4, 7}
	got := maxBoundariesWithMinGap(boundaries, 3)
	if got != 3 {
		t.Fatalf("maxBoundariesWithMinGap: got %d, want 3", got)
	}
}

// ---------------------------------------------------------------------------
// parseEvalStage
// ---------------------------------------------------------------------------

func TestParseEvalStage(t *testing.T) {
	tests := []struct {
		input   string
		wantRaw bool
		wantFin bool
		wantErr bool
	}{
		{"raw", true, false, false},
		{"Raw", true, false, false},
		{"final", false, true, false},
		{"Final", false, true, false},
		{"both", true, true, false},
		{"Both", true, true, false},
		{"invalid", false, false, true},
		{"", false, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseEvalStage(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseEvalStage(%q) error=%v, wantErr=%v", tt.input, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if got.raw != tt.wantRaw || got.final != tt.wantFin {
				t.Fatalf("parseEvalStage(%q) = {raw:%v, final:%v}, want {raw:%v, final:%v}",
					tt.input, got.raw, got.final, tt.wantRaw, tt.wantFin)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// methodName
// ---------------------------------------------------------------------------

func TestMethodName(t *testing.T) {
	tests := []struct {
		base, embedder, stage string
		includeStage          bool
		want                  string
	}{
		{"kire", "gemini", "raw", true, "kire (gemini,raw)"},
		{"kire", "gemini", "raw", false, "kire (gemini)"},
		{"kire", "", "final", true, "kire (final)"},
		{"kire", "", "final", false, "kire"},
		{"heading-split", "", "raw", true, "heading-split (raw)"},
		{"heading-split", "", "raw", false, "heading-split"},
	}
	for _, tt := range tests {
		got := methodName(tt.base, tt.embedder, tt.stage, tt.includeStage)
		if got != tt.want {
			t.Errorf("methodName(%q,%q,%q,%v) = %q, want %q",
				tt.base, tt.embedder, tt.stage, tt.includeStage, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// validateBoundaries
// ---------------------------------------------------------------------------

func TestValidateBoundaries(t *testing.T) {
	tests := []struct {
		name       string
		boundaries []int
		numBlocks  int
		wantErr    bool
	}{
		{"empty boundaries", nil, 10, false},
		{"valid single", []int{3}, 10, false},
		{"valid multiple", []int{1, 4, 7}, 10, false},
		{"max boundary", []int{8}, 10, false},
		{"out of range high", []int{9}, 10, true},
		{"out of range negative", []int{-1}, 10, true},
		{"not increasing", []int{3, 3}, 10, true},
		{"decreasing", []int{5, 3}, 10, true},
		{"boundaries with 1 block", []int{0}, 1, true},
		{"boundaries with 0 blocks", []int{0}, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBoundaries("test", tt.boundaries, tt.numBlocks)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateBoundaries(%v, %d) error=%v, wantErr=%v",
					tt.boundaries, tt.numBlocks, err, tt.wantErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// validateGold
// ---------------------------------------------------------------------------

func TestValidateGold(t *testing.T) {
	tests := []struct {
		name    string
		ann     *eval.GoldAnnotation
		blocks  int
		wantErr bool
	}{
		{
			"valid block unit",
			&eval.GoldAnnotation{Unit: "block", Boundaries: []int{2, 5}},
			10,
			false,
		},
		{
			"empty unit (default)",
			&eval.GoldAnnotation{Unit: "", Boundaries: []int{2}},
			10,
			false,
		},
		{
			"unsupported unit",
			&eval.GoldAnnotation{Unit: "sentence", Boundaries: []int{2}},
			10,
			true,
		},
		{
			"invalid boundary in gold",
			&eval.GoldAnnotation{Unit: "block", Boundaries: []int{20}},
			10,
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGold(tt.ann, tt.blocks)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateGold error=%v, wantErr=%v", err, tt.wantErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// cobra helper: newTestBenchCmd creates a fresh cobra.Command with bench flags.
// ---------------------------------------------------------------------------

func newTestBenchCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "test-bench"}
	f := cmd.Flags()

	f.String("eval-stage", "final", "")
	f.Int("tolerance", 1, "")
	f.Int("k", 0, "")
	f.Int("min-gap", 3, "")
	f.Int("min-tokens", 300, "")
	f.Int("max-tokens", 3000, "")
	f.Int("max-lines", -1, "")
	f.Int("window", 3, "")
	f.Int("block-k", 3, "")
	f.Float64("threshold", -1, "")
	f.Int("split-count", 0, "")
	f.Int("pack-heading-barrier", 0, "")
	f.String("profile", "", "")
	f.Bool("baseline-min-gap", true, "")
	f.Bool("allow-block-mismatch", false, "")

	// negatable flags (register both --flag and --no-flag)
	f.Bool("pseudo-heading", true, "")
	f.Bool("no-pseudo-heading", false, "")
	f.Bool("boundary-hints", true, "")
	f.Bool("no-boundary-hints", false, "")
	f.Bool("section-lock", true, "")
	f.Bool("no-section-lock", false, "")
	f.Bool("lock-after-heading", true, "")
	f.Bool("no-lock-after-heading", false, "")
	f.Bool("suppress-list-boundary", true, "")
	f.Bool("no-suppress-list-boundary", false, "")
	f.Bool("atomic-boundary", true, "")
	f.Bool("no-atomic-boundary", false, "")

	f.String("para-split-pattern", "", "")
	f.String("force-boundary", "", "")

	return cmd
}

// ---------------------------------------------------------------------------
// setIfNotChanged
// ---------------------------------------------------------------------------

func TestSetIfNotChanged(t *testing.T) {
	t.Run("sets when unchanged", func(t *testing.T) {
		cmd := newTestBenchCmd()
		if err := setIfNotChanged(cmd, "eval-stage", "raw"); err != nil {
			t.Fatal(err)
		}
		got, _ := cmd.Flags().GetString("eval-stage")
		if got != "raw" {
			t.Fatalf("expected raw, got %q", got)
		}
	})

	t.Run("skips when already changed", func(t *testing.T) {
		cmd := newTestBenchCmd()
		_ = cmd.Flags().Set("eval-stage", "both")
		if err := setIfNotChanged(cmd, "eval-stage", "raw"); err != nil {
			t.Fatal(err)
		}
		got, _ := cmd.Flags().GetString("eval-stage")
		if got != "both" {
			t.Fatalf("expected both (unchanged), got %q", got)
		}
	})
}

// ---------------------------------------------------------------------------
// flagChanged
// ---------------------------------------------------------------------------

func TestFlagChanged(t *testing.T) {
	t.Run("unchanged flag", func(t *testing.T) {
		cmd := newTestBenchCmd()
		if flagChanged(cmd, "eval-stage") {
			t.Fatal("expected false for unchanged flag")
		}
	})

	t.Run("changed flag", func(t *testing.T) {
		cmd := newTestBenchCmd()
		_ = cmd.Flags().Set("eval-stage", "raw")
		if !flagChanged(cmd, "eval-stage") {
			t.Fatal("expected true for changed flag")
		}
	})

	t.Run("negatable no- flag changed", func(t *testing.T) {
		cmd := newTestBenchCmd()
		_ = cmd.Flags().Set("no-section-lock", "true")
		if !flagChanged(cmd, "section-lock") {
			t.Fatal("expected true when no-section-lock is changed")
		}
	})

	t.Run("negatable neither changed", func(t *testing.T) {
		cmd := newTestBenchCmd()
		if flagChanged(cmd, "section-lock") {
			t.Fatal("expected false when neither flag changed")
		}
	})
}

// ---------------------------------------------------------------------------
// applyBenchProfile
// ---------------------------------------------------------------------------

func TestApplyBenchProfile(t *testing.T) {
	t.Run("empty profile is noop", func(t *testing.T) {
		cmd := newTestBenchCmd()
		if err := applyBenchProfile(cmd, ""); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("invalid profile", func(t *testing.T) {
		cmd := newTestBenchCmd()
		if err := applyBenchProfile(cmd, "bad"); err == nil {
			t.Fatal("expected error for invalid profile")
		}
	})

	t.Run("embedder profile", func(t *testing.T) {
		cmd := newTestBenchCmd()
		if err := applyBenchProfile(cmd, "embedder"); err != nil {
			t.Fatal(err)
		}
		got, _ := cmd.Flags().GetString("eval-stage")
		if got != "raw" {
			t.Fatalf("expected eval-stage=raw, got %q", got)
		}
		tol, _ := cmd.Flags().GetInt("tolerance")
		if tol != 0 {
			t.Fatalf("expected tolerance=0, got %d", tol)
		}
		k, _ := cmd.Flags().GetInt("k")
		if k != 3 {
			t.Fatalf("expected k=3, got %d", k)
		}
		minGap, _ := cmd.Flags().GetInt("min-gap")
		if minGap != 2 {
			t.Fatalf("expected min-gap=2, got %d", minGap)
		}
		win, _ := cmd.Flags().GetInt("window")
		if win != 5 {
			t.Fatalf("expected window=5, got %d", win)
		}
		bh, _ := cmd.Flags().GetBool("boundary-hints")
		if bh {
			t.Fatal("expected boundary-hints=false")
		}
		sl, _ := cmd.Flags().GetBool("section-lock")
		if sl {
			t.Fatal("expected section-lock=false")
		}
	})

	t.Run("output profile", func(t *testing.T) {
		cmd := newTestBenchCmd()
		if err := applyBenchProfile(cmd, "output"); err != nil {
			t.Fatal(err)
		}
		got, _ := cmd.Flags().GetString("eval-stage")
		if got != "final" {
			t.Fatalf("expected eval-stage=final, got %q", got)
		}
		mt, _ := cmd.Flags().GetInt("max-tokens")
		if mt != 800 {
			t.Fatalf("expected max-tokens=800, got %d", mt)
		}
	})

	t.Run("profile does not override explicit flags", func(t *testing.T) {
		cmd := newTestBenchCmd()
		_ = cmd.Flags().Set("eval-stage", "both")
		if err := applyBenchProfile(cmd, "embedder"); err != nil {
			t.Fatal(err)
		}
		got, _ := cmd.Flags().GetString("eval-stage")
		if got != "both" {
			t.Fatalf("expected eval-stage=both (explicit), got %q", got)
		}
	})
}

// ---------------------------------------------------------------------------
// applyGoldParams
// ---------------------------------------------------------------------------

func intPtr(v int) *int       { return &v }
func boolPtr(v bool) *bool    { return &v }
func strPtr(v string) *string { return &v }
func f64Ptr(v float64) *float64 { return &v }

func TestApplyGoldParams(t *testing.T) {
	t.Run("nil params is noop", func(t *testing.T) {
		cmd := newTestBenchCmd()
		if err := applyGoldParams(cmd, nil); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("applies eval_stage", func(t *testing.T) {
		cmd := newTestBenchCmd()
		params := &eval.GoldParams{EvalStage: "raw"}
		if err := applyGoldParams(cmd, params); err != nil {
			t.Fatal(err)
		}
		got, _ := cmd.Flags().GetString("eval-stage")
		if got != "raw" {
			t.Fatalf("expected raw, got %q", got)
		}
	})

	t.Run("applies integer params", func(t *testing.T) {
		cmd := newTestBenchCmd()
		params := &eval.GoldParams{
			Tolerance:  intPtr(2),
			K:          intPtr(5),
			MinGap:     intPtr(4),
			MinTokens:  intPtr(100),
			MaxTokens:  intPtr(500),
			MaxLines:   intPtr(200),
			Window:     intPtr(7),
			SplitCount: intPtr(10),
			PackHeadingBarrier: intPtr(2),
		}
		if err := applyGoldParams(cmd, params); err != nil {
			t.Fatal(err)
		}
		cases := []struct {
			flag string
			want int
		}{
			{"tolerance", 2},
			{"k", 5},
			{"min-gap", 4},
			{"min-tokens", 100},
			{"max-tokens", 500},
			{"max-lines", 200},
			{"window", 7},
			{"split-count", 10},
			{"pack-heading-barrier", 2},
		}
		for _, c := range cases {
			got, _ := cmd.Flags().GetInt(c.flag)
			if got != c.want {
				t.Errorf("flag %s: got %d, want %d", c.flag, got, c.want)
			}
		}
	})

	t.Run("applies threshold", func(t *testing.T) {
		cmd := newTestBenchCmd()
		params := &eval.GoldParams{Threshold: f64Ptr(0.5)}
		if err := applyGoldParams(cmd, params); err != nil {
			t.Fatal(err)
		}
		got, _ := cmd.Flags().GetFloat64("threshold")
		if got < 0.49 || got > 0.51 {
			t.Fatalf("expected threshold ~0.5, got %f", got)
		}
	})

	t.Run("applies bool params", func(t *testing.T) {
		cmd := newTestBenchCmd()
		params := &eval.GoldParams{
			PseudoHeading:        boolPtr(false),
			BoundaryHints:        boolPtr(false),
			SectionLock:          boolPtr(false),
			LockAfterHeading:     boolPtr(false),
			SuppressListBoundary: boolPtr(false),
			AtomicBoundary:       boolPtr(false),
			BaselineMinGap:       boolPtr(false),
			AllowBlockMismatch:   boolPtr(true),
		}
		if err := applyGoldParams(cmd, params); err != nil {
			t.Fatal(err)
		}
		boolCases := []struct {
			flag string
			want bool
		}{
			{"pseudo-heading", false},
			{"boundary-hints", false},
			{"section-lock", false},
			{"lock-after-heading", false},
			{"suppress-list-boundary", false},
			{"atomic-boundary", false},
			{"baseline-min-gap", false},
			{"allow-block-mismatch", true},
		}
		for _, c := range boolCases {
			got, _ := cmd.Flags().GetBool(c.flag)
			if got != c.want {
				t.Errorf("flag %s: got %v, want %v", c.flag, got, c.want)
			}
		}
	})

	t.Run("applies string params", func(t *testing.T) {
		cmd := newTestBenchCmd()
		params := &eval.GoldParams{
			ParaSplitPattern: strPtr("^\\d+\\."),
			ForceBoundary:    strPtr("^---$"),
		}
		if err := applyGoldParams(cmd, params); err != nil {
			t.Fatal(err)
		}
		psp, _ := cmd.Flags().GetString("para-split-pattern")
		if psp != "^\\d+\\." {
			t.Errorf("para-split-pattern: got %q", psp)
		}
		fb, _ := cmd.Flags().GetString("force-boundary")
		if fb != "^---$" {
			t.Errorf("force-boundary: got %q", fb)
		}
	})

	t.Run("does not override explicit flags", func(t *testing.T) {
		cmd := newTestBenchCmd()
		_ = cmd.Flags().Set("tolerance", "5")
		params := &eval.GoldParams{Tolerance: intPtr(2)}
		if err := applyGoldParams(cmd, params); err != nil {
			t.Fatal(err)
		}
		got, _ := cmd.Flags().GetInt("tolerance")
		if got != 5 {
			t.Fatalf("expected tolerance=5 (explicit), got %d", got)
		}
	})
}

// ---------------------------------------------------------------------------
// warnSmallDoc
// ---------------------------------------------------------------------------

func TestWarnSmallDoc(t *testing.T) {
	t.Run("warns for small doc", func(t *testing.T) {
		cmd := &cobra.Command{}
		var buf bytes.Buffer
		cmd.SetErr(&buf)
		warnSmallDoc(cmd, 25)
		if buf.Len() == 0 {
			t.Fatal("expected warning for 25 blocks")
		}
	})

	t.Run("no warning for large doc", func(t *testing.T) {
		cmd := &cobra.Command{}
		var buf bytes.Buffer
		cmd.SetErr(&buf)
		warnSmallDoc(cmd, 50)
		if buf.Len() != 0 {
			t.Fatalf("unexpected warning: %s", buf.String())
		}
	})

	t.Run("no warning for zero blocks", func(t *testing.T) {
		cmd := &cobra.Command{}
		var buf bytes.Buffer
		cmd.SetErr(&buf)
		warnSmallDoc(cmd, 0)
		if buf.Len() != 0 {
			t.Fatalf("unexpected warning for 0 blocks: %s", buf.String())
		}
	})
}

// ---------------------------------------------------------------------------
// warnMinGapCeiling
// ---------------------------------------------------------------------------

func TestWarnMinGapCeiling(t *testing.T) {
	t.Run("warns when min-gap prevents full match", func(t *testing.T) {
		cmd := &cobra.Command{}
		var buf bytes.Buffer
		cmd.SetErr(&buf)
		// boundaries [1,2,4] with minGap=3: only 1 and 4 can survive
		warnMinGapCeiling(cmd, []int{1, 2, 4}, 3)
		if buf.Len() == 0 {
			t.Fatal("expected warning")
		}
	})

	t.Run("no warning when all boundaries fit", func(t *testing.T) {
		cmd := &cobra.Command{}
		var buf bytes.Buffer
		cmd.SetErr(&buf)
		warnMinGapCeiling(cmd, []int{1, 5, 9}, 3)
		if buf.Len() != 0 {
			t.Fatalf("unexpected warning: %s", buf.String())
		}
	})

	t.Run("no warning for minGap <= 1", func(t *testing.T) {
		cmd := &cobra.Command{}
		var buf bytes.Buffer
		cmd.SetErr(&buf)
		warnMinGapCeiling(cmd, []int{1, 2, 3}, 1)
		if buf.Len() != 0 {
			t.Fatalf("unexpected warning: %s", buf.String())
		}
	})

	t.Run("no warning for empty boundaries", func(t *testing.T) {
		cmd := &cobra.Command{}
		var buf bytes.Buffer
		cmd.SetErr(&buf)
		warnMinGapCeiling(cmd, nil, 3)
		if buf.Len() != 0 {
			t.Fatalf("unexpected warning: %s", buf.String())
		}
	})
}

// ---------------------------------------------------------------------------
// warnAutoK
// ---------------------------------------------------------------------------

func TestWarnAutoK(t *testing.T) {
	t.Run("warns for tiny avg segment", func(t *testing.T) {
		cmd := &cobra.Command{}
		var buf bytes.Buffer
		cmd.SetErr(&buf)
		// 4 blocks, 3 boundaries => 4 segments, avgLen=1.0, kAuto=max(1,0)=1
		warnAutoK(cmd, []int{0, 1, 2}, 4, 0)
		if buf.Len() == 0 {
			t.Fatal("expected warning for kAuto=1")
		}
	})

	t.Run("no warning when k is explicit", func(t *testing.T) {
		cmd := &cobra.Command{}
		var buf bytes.Buffer
		cmd.SetErr(&buf)
		warnAutoK(cmd, []int{0, 1, 2}, 4, 3)
		if buf.Len() != 0 {
			t.Fatalf("unexpected warning: %s", buf.String())
		}
	})

	t.Run("no warning for large segments", func(t *testing.T) {
		cmd := &cobra.Command{}
		var buf bytes.Buffer
		cmd.SetErr(&buf)
		// 100 blocks, 4 boundaries => 5 segments, avgLen=20, kAuto=10
		warnAutoK(cmd, []int{20, 40, 60, 80}, 100, 0)
		if buf.Len() != 0 {
			t.Fatalf("unexpected warning: %s", buf.String())
		}
	})

	t.Run("no warning for < 2 blocks", func(t *testing.T) {
		cmd := &cobra.Command{}
		var buf bytes.Buffer
		cmd.SetErr(&buf)
		warnAutoK(cmd, nil, 1, 0)
		if buf.Len() != 0 {
			t.Fatalf("unexpected warning: %s", buf.String())
		}
	})
}

// ---------------------------------------------------------------------------
// computeDepthScores
// ---------------------------------------------------------------------------

func TestComputeDepthScores(t *testing.T) {
	t.Run("returns nil for < 2 embeddings", func(t *testing.T) {
		got := computeDepthScores(nil, 3, 0)
		if got != nil {
			t.Fatalf("expected nil, got %v", got)
		}
		got = computeDepthScores([]model.Embedding{{Vector: []float64{1, 0}}}, 3, 0)
		if got != nil {
			t.Fatalf("expected nil for single embedding, got %v", got)
		}
	})

	t.Run("computes scores for multiple embeddings", func(t *testing.T) {
		embeddings := []model.Embedding{
			{Vector: []float64{1, 0, 0}},
			{Vector: []float64{0.9, 0.1, 0}},
			{Vector: []float64{0, 0, 1}},
			{Vector: []float64{0, 0.1, 0.9}},
			{Vector: []float64{0, 0.1, 0.9}},
		}
		got := computeDepthScores(embeddings, 1, 0)
		if len(got) != 4 {
			t.Fatalf("expected 4 depth scores, got %d", len(got))
		}
		// Depth scores should be non-negative
		for i, v := range got {
			if v < 0 {
				t.Errorf("depth score[%d] = %f, expected >= 0", i, v)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// computeLockRanges
// ---------------------------------------------------------------------------

func TestComputeLockRanges(t *testing.T) {
	t.Run("no locks when both disabled", func(t *testing.T) {
		blocks := []model.Block{
			{Kind: model.BlockHeading, HeadingLevel: 1},
			{Kind: model.BlockParagraph},
			{Kind: model.BlockCodeBlock},
		}
		got := computeLockRanges(blocks, false, false, false)
		if len(got) != 0 {
			t.Fatalf("expected no lock ranges, got %v", got)
		}
	})

	t.Run("atomic boundary creates lock before code/table/math", func(t *testing.T) {
		blocks := []model.Block{
			{Kind: model.BlockParagraph},
			{Kind: model.BlockCodeBlock},
			{Kind: model.BlockParagraph},
			{Kind: model.BlockTable},
			{Kind: model.BlockParagraph},
			{Kind: model.BlockMathBlock},
		}
		got := computeLockRanges(blocks, false, false, true)
		if len(got) != 3 {
			t.Fatalf("expected 3 lock ranges, got %d: %v", len(got), got)
		}
	})
}

// ---------------------------------------------------------------------------
// optimizeBoundaries
// ---------------------------------------------------------------------------

func TestOptimizeBoundaries(t *testing.T) {
	blocks := make([]model.Block, 6)
	for i := range blocks {
		blocks[i] = model.Block{
			Kind:         model.BlockParagraph,
			Range:        model.SourceRange{Start: i * 100, End: (i + 1) * 100},
			TokensApprox: 100,
		}
	}

	depthScores := []float64{0.1, 0.8, 0.2, 0.7, 0.1}
	boundaries := []int{1, 3}

	got, err := optimizeBoundaries(blocks, depthScores, boundaries, segment.OptConfig{
		MinTokens: 50,
		MaxTokens: 500,
	})
	if err != nil {
		t.Fatal(err)
	}
	// Should produce some boundaries (exact output depends on optimizer)
	if got == nil {
		t.Fatal("expected non-nil boundaries")
	}
}
