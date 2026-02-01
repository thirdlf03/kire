package boundary

import (
	"bytes"
	"math"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/thirdlf03/kire/internal/model"
)

func TestWriteHTMLReport_Basic(t *testing.T) {
	source := []byte("# Introduction\n\nFirst paragraph.\n\nSecond paragraph.\n\n# Conclusion\n\nFinal paragraph.\n")
	blocks := []model.Block{
		{Kind: model.BlockHeading, Text: "Introduction", HeadingLevel: 1, Range: model.SourceRange{Start: 0, End: 16}},
		{Kind: model.BlockParagraph, Text: "First paragraph.", Range: model.SourceRange{Start: 16, End: 34}},
		{Kind: model.BlockParagraph, Text: "Second paragraph.", Range: model.SourceRange{Start: 34, End: 53}},
		{Kind: model.BlockHeading, Text: "Conclusion", HeadingLevel: 1, Range: model.SourceRange{Start: 53, End: 67}},
		{Kind: model.BlockParagraph, Text: "Final paragraph.", Range: model.SourceRange{Start: 67, End: 84}},
	}
	br := BoundaryResult{
		Boundaries:   []int{2},
		Similarities: []float64{0.9, 0.7, 0.3, 0.8},
		DepthScores:  []float64{0.0, 0.2, 0.6, 0.0},
		Threshold:    0.3,
	}
	segments := []model.Segment{
		{Blocks: blocks[0:3], TokenCount: 50, Range: model.SourceRange{Start: 0, End: 53}},
		{Blocks: blocks[3:5], TokenCount: 30, Range: model.SourceRange{Start: 53, End: 84}},
	}

	var buf bytes.Buffer
	cfg := ReportConfig{
		Blocks:    blocks,
		Result:    br,
		Segments:  segments,
		InputName: "test.md",
		Source:    source,
	}
	if err := WriteHTMLReport(&buf, cfg); err != nil {
		t.Fatal(err)
	}

	html := buf.String()

	// Check for canvas element
	if !strings.Contains(html, "<canvas") {
		t.Error("expected <canvas> element in output")
	}
	// Check for Chart.js CDN
	if !strings.Contains(html, "chart.js") {
		t.Error("expected chart.js CDN reference in output")
	}
	// Check block count is represented (5 blocks → table should have rows)
	if !strings.Contains(html, "Introduction") {
		t.Error("expected block text 'Introduction' in output")
	}
	if !strings.Contains(html, "Conclusion") {
		t.Error("expected block text 'Conclusion' in output")
	}
	// Check tab navigation exists
	if !strings.Contains(html, "tab-analysis") {
		t.Error("expected Analysis tab")
	}
	if !strings.Contains(html, "tab-split") {
		t.Error("expected Split View tab")
	}
	// Check split view has segment cards
	if !strings.Contains(html, "Segment 1") {
		t.Error("expected 'Segment 1' label in split view")
	}
	if !strings.Contains(html, "Segment 2") {
		t.Error("expected 'Segment 2' label in split view")
	}
	// Check chart guide text exists
	if !strings.Contains(html, "chart-guide") {
		t.Error("expected chart guide section in analysis tab")
	}
	// Check Japanese guide text
	if !strings.Contains(html, "グラフの読み方") {
		t.Error("expected Japanese chart guide text")
	}
	// Check marked.js is included for markdown rendering
	if !strings.Contains(html, "marked") {
		t.Error("expected marked.js for markdown rendering")
	}
}

func TestWriteHTMLReport_NoBoundaries(t *testing.T) {
	source := []byte("Only one block.\n")
	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "Only one block.", Range: model.SourceRange{Start: 0, End: 16}},
	}
	br := BoundaryResult{
		Boundaries:   nil,
		Similarities: nil,
		DepthScores:  nil,
		Threshold:    0,
	}
	segments := []model.Segment{
		{Blocks: blocks, TokenCount: 10, Range: model.SourceRange{Start: 0, End: 16}},
	}

	var buf bytes.Buffer
	cfg := ReportConfig{
		Blocks:    blocks,
		Result:    br,
		Segments:  segments,
		InputName: "empty.md",
		Source:    source,
	}
	// Should not panic
	if err := WriteHTMLReport(&buf, cfg); err != nil {
		t.Fatal(err)
	}
	if buf.Len() == 0 {
		t.Error("expected non-empty output")
	}
}

func TestWriteHTMLReport_CJKTruncation(t *testing.T) {
	// 90 CJK characters — should truncate to 80 runes, not 80 bytes
	longCJK := strings.Repeat("漢", 90)
	source := []byte(longCJK + "\n")
	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: longCJK, Range: model.SourceRange{Start: 0, End: len(source)}},
	}
	br := BoundaryResult{
		Similarities: nil,
		DepthScores:  nil,
	}
	segments := []model.Segment{
		{Blocks: blocks, TokenCount: 10, Range: model.SourceRange{Start: 0, End: len(source)}},
	}

	var buf bytes.Buffer
	cfg := ReportConfig{
		Blocks:    blocks,
		Result:    br,
		Segments:  segments,
		InputName: "cjk.md",
		Source:    source,
	}
	if err := WriteHTMLReport(&buf, cfg); err != nil {
		t.Fatal(err)
	}

	html := buf.String()
	// The output should contain valid UTF-8
	if !utf8.ValidString(html) {
		t.Error("output contains invalid UTF-8")
	}
}

func TestJsFloatSlice_NaN(t *testing.T) {
	vals := []float64{1.0, math.NaN(), math.Inf(1), 2.0}
	result := jsFloatSlice(vals)
	if strings.Contains(result, "NaN") || strings.Contains(result, "Inf") {
		t.Errorf("expected no NaN/Inf in output, got %s", result)
	}
}

func TestWriteHTMLReport_XSSPrevention(t *testing.T) {
	// Verify marked.use with html renderer override is present
	source := []byte("<script>alert(1)</script>\n")
	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "<script>alert(1)</script>", Range: model.SourceRange{Start: 0, End: len(source)}},
	}
	br := BoundaryResult{}
	segments := []model.Segment{
		{Blocks: blocks, TokenCount: 10, Range: model.SourceRange{Start: 0, End: len(source)}},
	}

	var buf bytes.Buffer
	cfg := ReportConfig{
		Blocks:    blocks,
		Result:    br,
		Segments:  segments,
		InputName: "xss-test.md",
		Source:    source,
	}
	if err := WriteHTMLReport(&buf, cfg); err != nil {
		t.Fatal(err)
	}

	html := buf.String()
	// Verify the XSS mitigation code is present in the rendered template
	if !strings.Contains(html, "renderer: { html: function()") {
		t.Error("expected marked.use XSS mitigation in output")
	}
}

func TestWriteHTMLReport_HTMLEscaping(t *testing.T) {
	source := []byte("<script>alert(\"xss\")</script>\n\nNormal text.\n")
	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: `<script>alert("xss")</script>`, Range: model.SourceRange{Start: 0, End: 30}},
		{Kind: model.BlockParagraph, Text: "Normal text.", Range: model.SourceRange{Start: 30, End: 44}},
	}
	br := BoundaryResult{
		Boundaries:   nil,
		Similarities: []float64{0.5},
		DepthScores:  []float64{0.1},
		Threshold:    0.2,
	}
	segments := []model.Segment{
		{Blocks: blocks, TokenCount: 10, Range: model.SourceRange{Start: 0, End: 44}},
	}

	var buf bytes.Buffer
	cfg := ReportConfig{
		Blocks:    blocks,
		Result:    br,
		Segments:  segments,
		InputName: "xss.md",
		Source:    source,
	}
	if err := WriteHTMLReport(&buf, cfg); err != nil {
		t.Fatal(err)
	}

	html := buf.String()
	// The raw <script> tag should be escaped
	if strings.Contains(html, `<script>alert`) {
		t.Error("expected <script> to be HTML-escaped in output")
	}
}
