package parser_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/thirdlf03/kire/internal/model"
	"github.com/thirdlf03/kire/internal/parser"
)

func testdataPath(name string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", name)
}

func readTestdata(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(testdataPath(name))
	if err != nil {
		t.Fatalf("failed to read testdata %s: %v", name, err)
	}
	return data
}

func TestParse_Empty(t *testing.T) {
	blocks, err := parser.Parse([]byte(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(blocks) != 0 {
		t.Errorf("expected 0 blocks, got %d", len(blocks))
	}
}

func TestParse_SingleParagraph(t *testing.T) {
	source := readTestdata(t, "single_paragraph.md")
	blocks, err := parser.Parse(source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	b := blocks[0]
	if b.Kind != model.BlockParagraph {
		t.Errorf("expected paragraph, got %s", b.Kind)
	}
	if len(b.HeadingPath) != 0 {
		t.Errorf("expected empty heading path, got %v", b.HeadingPath)
	}
}

func TestParse_HeadingAndParagraph(t *testing.T) {
	source := []byte("# Title\n\nSome text.\n")
	blocks, err := parser.Parse(source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}
	if blocks[0].Kind != model.BlockHeading {
		t.Errorf("expected heading, got %s", blocks[0].Kind)
	}
	if blocks[0].HeadingLevel != 1 {
		t.Errorf("expected level 1, got %d", blocks[0].HeadingLevel)
	}
	if blocks[1].Kind != model.BlockParagraph {
		t.Errorf("expected paragraph, got %s", blocks[1].Kind)
	}
	// Paragraph under "# Title" should have HeadingPath ["Title"]
	if len(blocks[1].HeadingPath) != 1 || blocks[1].HeadingPath[0] != "Title" {
		t.Errorf("expected heading path [Title], got %v", blocks[1].HeadingPath)
	}
}

func TestParse_NestedHeadings(t *testing.T) {
	source := readTestdata(t, "nested_headings.md")
	blocks, err := parser.Parse(source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find the block under "### Subsection A1"
	var found *model.Block
	for i := range blocks {
		if blocks[i].Kind == model.BlockParagraph && blocks[i].Text == "Content A1." {
			found = &blocks[i]
			break
		}
	}
	if found == nil {
		t.Fatal("could not find 'Content A1.' block")
	}
	expected := []string{"Top Level", "Section A", "Subsection A1"}
	if len(found.HeadingPath) != len(expected) {
		t.Fatalf("expected heading path %v, got %v", expected, found.HeadingPath)
	}
	for i, e := range expected {
		if found.HeadingPath[i] != e {
			t.Errorf("heading path[%d]: expected %q, got %q", i, e, found.HeadingPath[i])
		}
	}

	// Deep nesting: "#### Deep B1a" -> path should be [Top Level, Section B, Subsection B1, Deep B1a]
	var deep *model.Block
	for i := range blocks {
		if blocks[i].Kind == model.BlockParagraph && blocks[i].Text == "Deep content." {
			deep = &blocks[i]
			break
		}
	}
	if deep == nil {
		t.Fatal("could not find 'Deep content.' block")
	}
	expectedDeep := []string{"Top Level", "Section B", "Subsection B1", "Deep B1a"}
	if len(deep.HeadingPath) != len(expectedDeep) {
		t.Fatalf("expected heading path %v, got %v", expectedDeep, deep.HeadingPath)
	}
}

func TestParse_List(t *testing.T) {
	source := []byte("- Item 1\n- Item 2\n- Item 3\n")
	blocks, err := parser.Parse(source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block (list), got %d", len(blocks))
	}
	if blocks[0].Kind != model.BlockList {
		t.Errorf("expected list, got %s", blocks[0].Kind)
	}
}

func TestParse_CodeBlock(t *testing.T) {
	source := []byte("```go\nfunc main() {}\n```\n")
	blocks, err := parser.Parse(source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if blocks[0].Kind != model.BlockCodeBlock {
		t.Errorf("expected codeblock, got %s", blocks[0].Kind)
	}
}

func TestParse_Table(t *testing.T) {
	source := []byte("| A | B |\n|---|---|\n| 1 | 2 |\n")
	blocks, err := parser.Parse(source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if blocks[0].Kind != model.BlockTable {
		t.Errorf("expected table, got %s", blocks[0].Kind)
	}
}

func TestParse_Blockquote(t *testing.T) {
	source := []byte("> This is a quote.\n> Second line.\n")
	blocks, err := parser.Parse(source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if blocks[0].Kind != model.BlockBlockquote {
		t.Errorf("expected blockquote, got %s", blocks[0].Kind)
	}
}

func TestParse_ThematicBreak(t *testing.T) {
	source := []byte("---\n")
	blocks, err := parser.Parse(source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if blocks[0].Kind != model.BlockThematicBreak {
		t.Errorf("expected thematic_break, got %s", blocks[0].Kind)
	}
}

func TestParse_SourceRange(t *testing.T) {
	source := []byte("# Hello\n\nWorld.\n")
	blocks, err := parser.Parse(source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}
	// Block ranges tile the source: block[i].End == block[i+1].Start,
	// last block.End == len(source). So heading range includes trailing blank line.
	h := blocks[0]
	if string(source[h.Range.Start:h.Range.End]) != "# Hello\n\n" {
		t.Errorf("heading range: got %q", string(source[h.Range.Start:h.Range.End]))
	}
	p := blocks[1]
	if string(source[p.Range.Start:p.Range.End]) != "World.\n" {
		t.Errorf("paragraph range: got %q", string(source[p.Range.Start:p.Range.End]))
	}
	// Verify tiling: first start == 0, last end == len(source)
	if h.Range.Start != 0 {
		t.Errorf("first block start should be 0, got %d", h.Range.Start)
	}
	if p.Range.End != len(source) {
		t.Errorf("last block end should be %d, got %d", len(source), p.Range.End)
	}
	if h.Range.End != p.Range.Start {
		t.Errorf("blocks should tile: h.End=%d != p.Start=%d", h.Range.End, p.Range.Start)
	}
}

func TestParse_LosslessReconstruction(t *testing.T) {
	source := readTestdata(t, "simple.md")
	blocks, err := parser.Parse(source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(blocks) == 0 {
		t.Fatal("expected blocks")
	}
	// Reconstruct using source[first.Start:last.End]
	first := blocks[0].Range.Start
	last := blocks[len(blocks)-1].Range.End
	reconstructed := source[first:last]
	// Should cover from first block to last block
	if first != 0 {
		t.Errorf("expected first block start at 0, got %d", first)
	}
	if last != len(source) {
		t.Errorf("expected last block end at %d, got %d", len(source), last)
	}
	if string(reconstructed) != string(source) {
		t.Errorf("lossless reconstruction failed")
	}
}

func TestParse_ListSourceRange_MarkerIncluded(t *testing.T) {
	source := []byte("# Title\n\n- Item 1\n- Item 2\n\nEnd.\n")
	blocks, err := parser.Parse(source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Find the list block
	var listBlock *model.Block
	for i := range blocks {
		if blocks[i].Kind == model.BlockList {
			listBlock = &blocks[i]
			break
		}
	}
	if listBlock == nil {
		t.Fatal("expected a list block")
	}
	// The list block's range should include the "- " marker
	rangeText := string(source[listBlock.Range.Start:listBlock.Range.End])
	if !strings.HasPrefix(rangeText, "- ") {
		t.Errorf("list range should start with '- ', got range text: %q", rangeText)
	}
}

func TestParse_OrderedListSourceRange(t *testing.T) {
	source := []byte("1. First\n2. Second\n")
	blocks, err := parser.Parse(source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if blocks[0].Kind != model.BlockList {
		t.Fatalf("expected list, got %s", blocks[0].Kind)
	}
	rangeText := string(source[blocks[0].Range.Start:blocks[0].Range.End])
	if !strings.HasPrefix(rangeText, "1.") {
		t.Errorf("ordered list range should start with '1.', got range text: %q", rangeText)
	}
}

func TestParse_SimpleFile(t *testing.T) {
	source := readTestdata(t, "simple.md")
	blocks, err := parser.Parse(source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// simple.md has: heading1, paragraph, heading2, paragraph, list, heading3, paragraph, codeblock, paragraph
	expectedKinds := []model.BlockKind{
		model.BlockHeading,   // # Heading 1
		model.BlockParagraph, // This is a paragraph.
		model.BlockHeading,   // ## Heading 2
		model.BlockParagraph, // Another paragraph here.
		model.BlockList,      // - Item 1, 2, 3
		model.BlockHeading,   // ### Heading 3
		model.BlockParagraph, // Some code:
		model.BlockCodeBlock, // ```go ... ```
		model.BlockParagraph, // Final paragraph.
	}
	if len(blocks) != len(expectedKinds) {
		t.Fatalf("expected %d blocks, got %d", len(expectedKinds), len(blocks))
		for i, b := range blocks {
			t.Logf("  block[%d]: %s %q", i, b.Kind, b.Text)
		}
	}
	for i, ek := range expectedKinds {
		if blocks[i].Kind != ek {
			t.Errorf("block[%d]: expected %s, got %s", i, ek, blocks[i].Kind)
		}
	}
}

func TestParse_HTMLBlock(t *testing.T) {
	source := []byte("<div>\nhello\n</div>\n")
	blocks, err := parser.Parse(source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if blocks[0].Kind != model.BlockHTMLBlock {
		t.Errorf("expected html_block, got %s", blocks[0].Kind)
	}
}

func TestParse_MathBlock(t *testing.T) {
	source := readTestdata(t, "math_block.md")
	blocks, err := parser.Parse(source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expected: heading, paragraph, math_block, paragraph
	expectedKinds := []model.BlockKind{
		model.BlockHeading,
		model.BlockParagraph,
		model.BlockMathBlock,
		model.BlockParagraph,
	}
	if len(blocks) != len(expectedKinds) {
		for i, b := range blocks {
			t.Logf("  block[%d]: %s %q", i, b.Kind, b.Text)
		}
		t.Fatalf("expected %d blocks, got %d", len(expectedKinds), len(blocks))
	}
	for i, ek := range expectedKinds {
		if blocks[i].Kind != ek {
			t.Errorf("block[%d]: expected %s, got %s", i, ek, blocks[i].Kind)
		}
	}

	// Lossless reconstruction
	first := blocks[0].Range.Start
	last := blocks[len(blocks)-1].Range.End
	if string(source[first:last]) != string(source) {
		t.Error("lossless reconstruction failed for math_block.md")
	}
}

func TestParse_IndentedCodeBlock(t *testing.T) {
	source := []byte("    code line 1\n    code line 2\n")
	blocks, err := parser.Parse(source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if blocks[0].Kind != model.BlockCodeBlock {
		t.Errorf("expected codeblock, got %s", blocks[0].Kind)
	}
}

func TestParse_TableAndQuoteFile(t *testing.T) {
	source := readTestdata(t, "table_and_quote.md")
	blocks, err := parser.Parse(source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Expected: heading, table, blockquote, thematic_break, paragraph
	expectedKinds := []model.BlockKind{
		model.BlockHeading,
		model.BlockTable,
		model.BlockBlockquote,
		model.BlockThematicBreak,
		model.BlockParagraph,
	}
	if len(blocks) != len(expectedKinds) {
		for i, b := range blocks {
			t.Logf("  block[%d]: %s %q", i, b.Kind, b.Text)
		}
		t.Fatalf("expected %d blocks, got %d", len(expectedKinds), len(blocks))
	}
	for i, ek := range expectedKinds {
		if blocks[i].Kind != ek {
			t.Errorf("block[%d]: expected %s, got %s", i, ek, blocks[i].Kind)
		}
	}
	// Lossless
	first := blocks[0].Range.Start
	last := blocks[len(blocks)-1].Range.End
	if string(source[first:last]) != string(source) {
		t.Error("lossless reconstruction failed for table_and_quote.md")
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
