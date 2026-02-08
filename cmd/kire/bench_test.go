package main

import (
	"bytes"
	"slices"
	"testing"

	"github.com/spf13/cobra"

	"github.com/thirdlf03/kire/internal/eval"
	"github.com/thirdlf03/kire/internal/model"
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
// applyGoldParams
// ---------------------------------------------------------------------------

func intPtr(v int) *int { return &v }

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
			Tolerance: intPtr(2),
			K:         intPtr(5),
		}
		if err := applyGoldParams(cmd, params); err != nil {
			t.Fatal(err)
		}
		tol, _ := cmd.Flags().GetInt("tolerance")
		if tol != 2 {
			t.Errorf("tolerance: got %d, want 2", tol)
		}
		k, _ := cmd.Flags().GetInt("k")
		if k != 5 {
			t.Errorf("k: got %d, want 5", k)
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
// warnAutoK
// ---------------------------------------------------------------------------

func TestWarnAutoK(t *testing.T) {
	t.Run("warns for tiny avg segment", func(t *testing.T) {
		cmd := &cobra.Command{}
		var buf bytes.Buffer
		cmd.SetErr(&buf)
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
