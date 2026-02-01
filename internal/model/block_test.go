package model_test

import (
	"testing"

	"github.com/thirdlf03/kire/internal/model"
)

func TestBlockKind_String(t *testing.T) {
	tests := []struct {
		kind     model.BlockKind
		expected string
	}{
		{model.BlockHeading, "heading"},
		{model.BlockParagraph, "paragraph"},
		{model.BlockList, "list"},
		{model.BlockCodeBlock, "codeblock"},
		{model.BlockThematicBreak, "thematic_break"},
		{model.BlockTable, "table"},
		{model.BlockBlockquote, "blockquote"},
		{model.BlockHTMLBlock, "html_block"},
		{model.BlockMathBlock, "math_block"},
		{model.BlockKind(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.kind.String(); got != tt.expected {
			t.Errorf("BlockKind(%d).String() = %q, want %q", tt.kind, got, tt.expected)
		}
	}
}

func TestBlockStartIndex(t *testing.T) {
	blocks := []model.Block{
		{Range: model.SourceRange{Start: 0, End: 10}},
		{Range: model.SourceRange{Start: 10, End: 25}},
		{Range: model.SourceRange{Start: 25, End: 50}},
	}
	m := model.BlockStartIndex(blocks)
	if len(m) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(m))
	}
	for i, b := range blocks {
		if got, ok := m[b.Range.Start]; !ok || got != i {
			t.Errorf("BlockStartIndex[%d] = %d, %v; want %d, true", b.Range.Start, got, ok, i)
		}
	}
}

func TestBlockStartIndex_Empty(t *testing.T) {
	m := model.BlockStartIndex(nil)
	if len(m) != 0 {
		t.Errorf("expected empty map, got %d entries", len(m))
	}
}
