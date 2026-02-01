package eval

import (
	"strings"
	"testing"
)

func TestEvaluate_PerfectMatch(t *testing.T) {
	gold := &GoldAnnotation{
		DocID:      "test",
		Unit:       "block",
		Boundaries: []int{3, 7},
	}
	result := Evaluate(gold, 10, []int{3, 7}, "perfect", 0, 0)

	if result.Name != "perfect" {
		t.Errorf("Name: want perfect, got %s", result.Name)
	}
	if result.NumSegs != 3 {
		t.Errorf("NumSegs: want 3, got %d", result.NumSegs)
	}
	if result.Pk != 0 {
		t.Errorf("Pk: want 0, got %f", result.Pk)
	}
	if result.WindowDiff != 0 {
		t.Errorf("WindowDiff: want 0, got %f", result.WindowDiff)
	}
	if result.Boundary.F1 != 1 {
		t.Errorf("F1: want 1, got %f", result.Boundary.F1)
	}
}

func TestEvaluate_Mismatch(t *testing.T) {
	gold := &GoldAnnotation{
		DocID:      "test",
		Unit:       "block",
		Boundaries: []int{3, 7},
	}
	result := Evaluate(gold, 10, []int{1, 5}, "shifted", 0, 0)

	if result.Pk <= 0 {
		t.Errorf("Pk should be positive for mismatch, got %f", result.Pk)
	}
	if result.Boundary.F1 >= 1 {
		t.Errorf("F1 should be < 1 for mismatch, got %f", result.Boundary.F1)
	}
}

func TestFormatTable(t *testing.T) {
	report := EvalReport{
		DocID:     "test-doc",
		NumBlocks: 20,
		Gold: GoldAnnotation{
			Boundaries: []int{5, 10, 15},
		},
		Methods: []MethodResult{
			{Name: "kire", NumSegs: 4, Pk: 0.12, WindowDiff: 0.15,
				Boundary: BoundaryScores{Precision: 0.83, Recall: 0.67, F1: 0.74}},
			{Name: "heading-split", NumSegs: 6, Pk: 0.25, WindowDiff: 0.30,
				Boundary: BoundaryScores{Precision: 0.50, Recall: 1.00, F1: 0.67}},
		},
	}

	out := FormatTable(report)

	// Check header line
	if !strings.Contains(out, "test-doc") {
		t.Error("FormatTable should contain doc ID")
	}
	if !strings.Contains(out, "20 blocks") {
		t.Error("FormatTable should contain block count")
	}
	if !strings.Contains(out, "3 gold boundaries") {
		t.Error("FormatTable should contain gold boundary count")
	}

	// Check method rows
	if !strings.Contains(out, "kire") {
		t.Error("FormatTable should contain method name 'kire'")
	}
	if !strings.Contains(out, "heading-split") {
		t.Error("FormatTable should contain method name 'heading-split'")
	}

	// Check separator line exists
	if !strings.Contains(out, "\u2500") {
		t.Error("FormatTable should contain separator")
	}
}

func TestFormatTable_Empty(t *testing.T) {
	report := EvalReport{
		DocID:     "empty",
		NumBlocks: 0,
		Gold:      GoldAnnotation{},
	}

	out := FormatTable(report)
	if !strings.Contains(out, "empty") {
		t.Error("FormatTable should contain doc ID even with no methods")
	}
}
