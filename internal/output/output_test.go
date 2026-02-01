package output_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thirdlf03/kire/internal/model"
	"github.com/thirdlf03/kire/internal/output"
)

func TestRender_BasicSegment(t *testing.T) {
	source := []byte("# Hello\n\nWorld paragraph.\n")
	// source len = 26: "# Hello\n" (8) + "\n" (1) + "World paragraph.\n" (17)
	seg := model.Segment{
		Blocks: []model.Block{
			{Range: model.SourceRange{Start: 0, End: 9}},
			{Range: model.SourceRange{Start: 9, End: 26}},
		},
		Range: model.SourceRange{Start: 0, End: 26},
	}
	rendered := output.Render(seg, source, output.RenderConfig{})
	expected := "# Hello\n\nWorld paragraph.\n"
	if rendered != expected {
		t.Errorf("expected %q, got %q", expected, rendered)
	}
}

func TestRender_WithContext(t *testing.T) {
	source := []byte("Content here.\n")
	seg := model.Segment{
		Blocks: []model.Block{
			{Range: model.SourceRange{Start: 0, End: 14}, HeadingPath: []string{"Section A"}},
		},
		Range:       model.SourceRange{Start: 0, End: 14},
		HeadingPath: []string{"Section A"},
	}
	rendered := output.Render(seg, source, output.RenderConfig{
		ContextFormat: "comment",
	})
	expected := "<!-- context: Section A -->\n\nContent here.\n"
	if rendered != expected {
		t.Errorf("expected %q, got %q", expected, rendered)
	}
}

func TestRender_WithOverlap(t *testing.T) {
	source := []byte("Line 1\nLine 2\nLine 3\nLine 4\n")
	// Overlap blocks from previous segment
	overlapBlocks := []model.Block{
		{Range: model.SourceRange{Start: 0, End: 14}}, // "Line 1\nLine 2\n"
	}
	seg := model.Segment{
		Blocks: []model.Block{
			{Range: model.SourceRange{Start: 14, End: 28}},
		},
		Range: model.SourceRange{Start: 14, End: 28},
	}
	rendered := output.Render(seg, source, output.RenderConfig{
		OverlapBlocks: overlapBlocks,
	})
	expected := "Line 1\nLine 2\nLine 3\nLine 4\n"
	if rendered != expected {
		t.Errorf("expected %q, got %q", expected, rendered)
	}
}

func TestWriteFiles(t *testing.T) {
	dir := t.TempDir()
	contents := []string{"# Part 1\n\nContent.\n", "# Part 2\n\nMore content.\n"}

	err := output.WriteFiles(dir, "split", contents)
	if err != nil {
		t.Fatal(err)
	}

	for i, expected := range contents {
		path := filepath.Join(dir, "split"+string(rune('1'+i))+".md")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read %s: %v", path, err)
		}
		if string(data) != expected {
			t.Errorf("file %d: expected %q, got %q", i+1, expected, string(data))
		}
	}
}

func TestLosslessRoundTrip(t *testing.T) {
	source := []byte("# Heading 1\n\nParagraph one.\n\n## Heading 2\n\nParagraph two.\n")
	// source len = 58
	// "# Heading 1\n" (12) + "\n" (1) + "Paragraph one.\n" (15) + "\n" (1)
	// + "## Heading 2\n" (13) + "\n" (1) + "Paragraph two.\n" (15) = 58
	// Segment 1: blocks at [0..13) and [13..29) → segment [0..29)
	// Segment 2: blocks at [29..43) and [43..58) → segment [29..58)
	seg1 := model.Segment{
		Blocks: []model.Block{
			{Range: model.SourceRange{Start: 0, End: 13}},
			{Range: model.SourceRange{Start: 13, End: 29}},
		},
		Range: model.SourceRange{Start: 0, End: 29},
	}
	seg2 := model.Segment{
		Blocks: []model.Block{
			{Range: model.SourceRange{Start: 29, End: 43}},
			{Range: model.SourceRange{Start: 43, End: 58}},
		},
		Range: model.SourceRange{Start: 29, End: 58},
	}

	// Render without context/overlap → should be lossless
	rendered1 := output.Render(seg1, source, output.RenderConfig{})
	rendered2 := output.Render(seg2, source, output.RenderConfig{})

	concatenated := rendered1 + rendered2
	if concatenated != string(source) {
		t.Errorf("lossless round-trip failed")
		t.Logf("source:       %q", string(source))
		t.Logf("concatenated: %q", concatenated)
		// Find first difference
		for i := 0; i < len(source) && i < len(concatenated); i++ {
			if source[i] != concatenated[i] {
				t.Logf("first diff at byte %d: source=%q concat=%q",
					i, string(source[max(0, i-10):min(len(source), i+10)]),
					string(concatenated[max(0, i-10):min(len(concatenated), i+10)]))
				break
			}
		}
	}
}

func TestRender_OutOfBoundsRange(t *testing.T) {
	source := []byte("short")
	seg := model.Segment{
		Blocks: []model.Block{
			{Range: model.SourceRange{Start: 0, End: 100}}, // End > len(source)
		},
		Range: model.SourceRange{Start: 0, End: 100},
	}
	// Should not panic
	rendered := output.Render(seg, source, output.RenderConfig{})
	if rendered != "" {
		t.Errorf("expected empty string for out-of-bounds, got %q", rendered)
	}
}

func TestWriteNamedFiles_LengthMismatch(t *testing.T) {
	dir := t.TempDir()
	err := output.WriteNamedFiles(dir, []string{"a.md"}, []string{"x", "y"})
	if err == nil {
		t.Fatal("expected error for length mismatch")
	}
}

func TestWriteNamedFiles_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	err := output.WriteNamedFiles(dir, []string{"../escape.md"}, []string{"content"})
	if err != nil {
		// filepath.Base("../escape.md") = "escape.md", which is safe
		// So this should succeed but write inside dir, not outside
		t.Fatalf("unexpected error: %v", err)
	}
	// Verify file was written inside dir, not outside
	if _, err := os.Stat(filepath.Join(dir, "escape.md")); err != nil {
		t.Error("expected escape.md inside dir")
	}
	if _, err := os.Stat(filepath.Join(dir, "..", "escape.md")); err == nil {
		t.Error("file should not exist outside dir")
	}
}

func TestComputeOverlapBlocks(t *testing.T) {
	source := []byte("Line1\nLine2\nLine3\nLine4\nLine5\n")
	blocks := []model.Block{
		{Range: model.SourceRange{Start: 0, End: 6}},   // "Line1\n"
		{Range: model.SourceRange{Start: 6, End: 12}},  // "Line2\n"
		{Range: model.SourceRange{Start: 12, End: 18}}, // "Line3\n"
		{Range: model.SourceRange{Start: 18, End: 24}}, // "Line4\n"
		{Range: model.SourceRange{Start: 24, End: 30}}, // "Line5\n"
	}

	overlap := output.ComputeOverlapBlocks(blocks, source, 2)
	if len(overlap) != 2 {
		t.Fatalf("expected 2 overlap blocks, got %d", len(overlap))
	}

	// Should be last 2 blocks covering at least 2 lines
	overlapText := ""
	for _, b := range overlap {
		overlapText += string(source[b.Range.Start:b.Range.End])
	}
	if !strings.Contains(overlapText, "Line4") || !strings.Contains(overlapText, "Line5") {
		t.Errorf("expected overlap to contain Line4 and Line5, got %q", overlapText)
	}
}
