package boundary_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/thirdlf03/kire/internal/boundary"
	"github.com/thirdlf03/kire/internal/model"
)

func TestWriteDebugTSV_Header(t *testing.T) {
	var buf bytes.Buffer
	err := boundary.WriteDebugTSV(&buf, nil, boundary.BoundaryResult{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 header line for empty blocks, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "block_index") {
		t.Error("header should contain block_index")
	}
}

func TestWriteDebugTSV_BasicOutput(t *testing.T) {
	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "Hello world"},
		{Kind: model.BlockParagraph, Text: "Foo bar"},
	}
	result := boundary.BoundaryResult{
		Boundaries:   []int{0},
		Similarities: []float64{0.5},
		DepthScores:  []float64{0.3},
	}
	var buf bytes.Buffer
	err := boundary.WriteDebugTSV(&buf, blocks, result, 0)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	// Header + 2 data lines
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	// First data line should have boundary marker
	if !strings.Contains(lines[1], "***") {
		t.Error("expected boundary marker on first data line")
	}
	// Second data line should not have marker
	if strings.Contains(lines[2], "***") {
		t.Error("did not expect boundary marker on second data line")
	}
}

func TestWriteDebugTSV_PreviewTruncation(t *testing.T) {
	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "This is a very long text that should be truncated"},
	}
	result := boundary.BoundaryResult{}
	var buf bytes.Buffer
	err := boundary.WriteDebugTSV(&buf, blocks, result, 10)
	if err != nil {
		t.Fatal(err)
	}
	output := buf.String()
	if !strings.Contains(output, "This is a ...") {
		t.Errorf("expected truncated preview, got: %s", output)
	}
}

func TestWriteDebugTSV_UTF8Preview(t *testing.T) {
	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "日本語のテキストサンプル"},
	}
	result := boundary.BoundaryResult{}
	var buf bytes.Buffer
	err := boundary.WriteDebugTSV(&buf, blocks, result, 5)
	if err != nil {
		t.Fatal(err)
	}
	output := buf.String()
	if !strings.Contains(output, "日本語のテ...") {
		t.Errorf("expected rune-based truncation '日本語のテ...', got: %s", output)
	}
}

func TestWriteDebugTSV_TabNewlineInPreview(t *testing.T) {
	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "line1\ttab\nline2"},
	}
	result := boundary.BoundaryResult{}
	var buf bytes.Buffer
	err := boundary.WriteDebugTSV(&buf, blocks, result, 0)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	// Data line should not contain raw tabs in preview (tabs are field separators)
	dataLine := lines[1]
	// The preview field is the 3rd tab-separated field
	fields := strings.Split(dataLine, "\t")
	preview := fields[2]
	if strings.Contains(preview, "\t") || strings.Contains(preview, "\n") {
		t.Errorf("preview should not contain raw tabs/newlines: %q", preview)
	}
}
