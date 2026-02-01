package eval

import (
	"fmt"
	"strings"
)

// MethodResult holds evaluation results for a single segmentation method.
type MethodResult struct {
	Name       string         `json:"name"`
	Boundaries []int          `json:"boundaries"`
	NumSegs    int            `json:"num_segments"`
	Pk         float64        `json:"pk"`
	WindowDiff float64        `json:"window_diff"`
	Boundary   BoundaryScores `json:"boundary_scores"`
}

// EvalReport holds evaluation results for a document.
type EvalReport struct {
	DocID     string         `json:"doc_id"`
	NumBlocks int            `json:"num_blocks"`
	Gold      GoldAnnotation `json:"gold"`
	Methods   []MethodResult `json:"methods"`
}

// Evaluate computes all metrics for a single method against a gold standard.
func Evaluate(gold *GoldAnnotation, numBlocks int, hyp []int, name string, k, tolerance int) MethodResult {
	return MethodResult{
		Name:       name,
		Boundaries: hyp,
		NumSegs:    len(hyp) + 1,
		Pk:         Pk(gold.Boundaries, hyp, numBlocks, k),
		WindowDiff: WindowDiff(gold.Boundaries, hyp, numBlocks, k),
		Boundary:   BoundaryPRF(gold.Boundaries, hyp, numBlocks, tolerance),
	}
}

// FormatTable renders an EvalReport as a human-readable table.
func FormatTable(report EvalReport) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Document: %s (%d blocks, %d gold boundaries)\n\n",
		report.DocID, report.NumBlocks, len(report.Gold.Boundaries))

	header := fmt.Sprintf("%-20s %4s %6s %6s %6s %6s %6s",
		"Method", "Segs", "Pk", "WDiff", "P", "R", "F1")
	b.WriteString(header)
	b.WriteByte('\n')
	b.WriteString(strings.Repeat("\u2500", len(header)))
	b.WriteByte('\n')

	for _, m := range report.Methods {
		fmt.Fprintf(&b, "%-20s %4d %6.2f %6.2f %6.2f %6.2f %6.2f\n",
			m.Name,
			m.NumSegs,
			m.Pk,
			m.WindowDiff,
			m.Boundary.Precision,
			m.Boundary.Recall,
			m.Boundary.F1,
		)
	}

	return b.String()
}
