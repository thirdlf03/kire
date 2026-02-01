package parser_test

import (
	"regexp"
	"testing"

	"github.com/thirdlf03/kire/internal/model"
	"github.com/thirdlf03/kire/internal/parser"
)

func TestSplitLongParagraphs_Basic(t *testing.T) {
	// A paragraph containing "追加機能:" followed by other text should be split
	source := []byte("追加機能: ラベル管理（Label）\n背景\nタスクの分類を現場の言葉に合わせて整理できる\n")
	blocks := []model.Block{
		{
			Kind:  model.BlockParagraph,
			Text:  "追加機能: ラベル管理（Label）\n背景\nタスクの分類を現場の言葉に合わせて整理できる\n",
			Range: model.SourceRange{Start: 0, End: len(source)},
		},
	}

	patterns := []*regexp.Regexp{regexp.MustCompile(`^追加機能:`)}
	result := parser.SplitLongParagraphs(blocks, patterns, source)

	if len(result) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(result))
	}

	// First block should be the matching line
	if result[0].Kind != model.BlockParagraph {
		t.Errorf("block 0: expected paragraph, got %s", result[0].Kind)
	}
	if result[0].Text != "追加機能: ラベル管理（Label）" {
		t.Errorf("block 0 text: got %q", result[0].Text)
	}

	// Second block should be the remainder
	if result[1].Kind != model.BlockParagraph {
		t.Errorf("block 1: expected paragraph, got %s", result[1].Kind)
	}
	if result[1].Text != "背景\nタスクの分類を現場の言葉に合わせて整理できる" {
		t.Errorf("block 1 text: got %q", result[1].Text)
	}
}

func TestSplitLongParagraphs_SourceRangeConsistency(t *testing.T) {
	// Verify that SourceRange is correctly recalculated after split
	source := []byte("追加機能: ラベル管理（Label）\n背景\nテキスト\n")
	blocks := []model.Block{
		{
			Kind:  model.BlockParagraph,
			Text:  "追加機能: ラベル管理（Label）\n背景\nテキスト\n",
			Range: model.SourceRange{Start: 0, End: len(source)},
		},
	}

	patterns := []*regexp.Regexp{regexp.MustCompile(`^追加機能:`)}
	result := parser.SplitLongParagraphs(blocks, patterns, source)

	if len(result) < 2 {
		t.Fatalf("expected at least 2 blocks, got %d", len(result))
	}

	// Ranges should tile: block[0].End == block[1].Start
	if result[0].Range.End != result[1].Range.Start {
		t.Errorf("ranges don't tile: block0.End=%d, block1.Start=%d",
			result[0].Range.End, result[1].Range.Start)
	}

	// Combined range should match original
	if result[0].Range.Start != 0 {
		t.Errorf("first block start should be 0, got %d", result[0].Range.Start)
	}
	if result[len(result)-1].Range.End != len(source) {
		t.Errorf("last block end should be %d, got %d", len(source), result[len(result)-1].Range.End)
	}

	// Source reconstruction
	for i, b := range result {
		reconstructed := string(source[b.Range.Start:b.Range.End])
		if len(reconstructed) == 0 {
			t.Errorf("block %d: empty source reconstruction", i)
		}
	}
}

func TestSplitLongParagraphs_NoMatch(t *testing.T) {
	source := []byte("普通のテキスト。\n次の行。\n")
	blocks := []model.Block{
		{
			Kind:  model.BlockParagraph,
			Text:  "普通のテキスト。\n次の行。\n",
			Range: model.SourceRange{Start: 0, End: len(source)},
		},
	}

	patterns := []*regexp.Regexp{regexp.MustCompile(`^追加機能:`)}
	result := parser.SplitLongParagraphs(blocks, patterns, source)

	if len(result) != 1 {
		t.Fatalf("expected 1 block (no match), got %d", len(result))
	}
}

func TestSplitLongParagraphs_NonParagraph(t *testing.T) {
	// Non-paragraph blocks should be passed through unchanged
	source := []byte("- 追加機能: item\n- other\n")
	blocks := []model.Block{
		{
			Kind:  model.BlockList,
			Text:  "追加機能: item\nother",
			Range: model.SourceRange{Start: 0, End: len(source)},
		},
	}

	patterns := []*regexp.Regexp{regexp.MustCompile(`^追加機能:`)}
	result := parser.SplitLongParagraphs(blocks, patterns, source)

	if len(result) != 1 {
		t.Fatalf("expected 1 block (non-paragraph), got %d", len(result))
	}
	if result[0].Kind != model.BlockList {
		t.Errorf("expected list, got %s", result[0].Kind)
	}
}

func TestSplitLongParagraphs_EmptyPatterns(t *testing.T) {
	source := []byte("テキスト\n")
	blocks := []model.Block{
		{
			Kind:  model.BlockParagraph,
			Text:  "テキスト\n",
			Range: model.SourceRange{Start: 0, End: len(source)},
		},
	}

	result := parser.SplitLongParagraphs(blocks, nil, source)
	if len(result) != 1 {
		t.Fatalf("expected 1 block (no patterns), got %d", len(result))
	}
}

func TestSplitLongParagraphs_MultipleSplitsInOneParagraph(t *testing.T) {
	// Multiple matching lines in one paragraph
	// Each match line becomes a single-line block; non-matching lines between them
	// also become separate blocks: [match1] [between] [match2] [after]
	source := []byte("追加機能: ラベル管理\n背景テキスト\n追加機能: プロジェクト管理\n詳細テキスト\n")
	blocks := []model.Block{
		{
			Kind:  model.BlockParagraph,
			Text:  "追加機能: ラベル管理\n背景テキスト\n追加機能: プロジェクト管理\n詳細テキスト\n",
			Range: model.SourceRange{Start: 0, End: len(source)},
		},
	}

	patterns := []*regexp.Regexp{regexp.MustCompile(`^追加機能:`)}
	result := parser.SplitLongParagraphs(blocks, patterns, source)

	if len(result) != 4 {
		t.Fatalf("expected 4 blocks, got %d", len(result))
	}

	// Verify tiling
	for i := 0; i < len(result)-1; i++ {
		if result[i].Range.End != result[i+1].Range.Start {
			t.Errorf("blocks %d and %d don't tile: end=%d, start=%d",
				i, i+1, result[i].Range.End, result[i+1].Range.Start)
		}
	}

	// Verify content
	if result[0].Text != "追加機能: ラベル管理" {
		t.Errorf("block 0: got %q", result[0].Text)
	}
	if result[1].Text != "背景テキスト" {
		t.Errorf("block 1: got %q", result[1].Text)
	}
	if result[2].Text != "追加機能: プロジェクト管理" {
		t.Errorf("block 2: got %q", result[2].Text)
	}
	if result[3].Text != "詳細テキスト" {
		t.Errorf("block 3: got %q", result[3].Text)
	}
}

func TestSplitLongParagraphs_MatchAtFirstLine(t *testing.T) {
	// Pattern matches at the very first line — no preceding text
	source := []byte("追加機能: テスト\n後続テキスト\n")
	blocks := []model.Block{
		{
			Kind:  model.BlockParagraph,
			Text:  "追加機能: テスト\n後続テキスト\n",
			Range: model.SourceRange{Start: 0, End: len(source)},
		},
	}

	patterns := []*regexp.Regexp{regexp.MustCompile(`^追加機能:`)}
	result := parser.SplitLongParagraphs(blocks, patterns, source)

	// First line matches, so split into: [matched line] [remainder]
	if len(result) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(result))
	}
}

func TestSplitLongParagraphs_PreservesOtherBlocks(t *testing.T) {
	// Multiple blocks where only one is a matching paragraph
	source := []byte("# Heading\n\n追加機能: ラベル\n背景テキスト\n\n- list item\n")
	blocks := []model.Block{
		{
			Kind:         model.BlockHeading,
			Text:         "Heading",
			HeadingLevel: 1,
			Range:        model.SourceRange{Start: 0, End: 12},
		},
		{
			Kind:  model.BlockParagraph,
			Text:  "追加機能: ラベル\n背景テキスト\n",
			Range: model.SourceRange{Start: 12, End: 52},
		},
		{
			Kind:  model.BlockList,
			Text:  "list item",
			Range: model.SourceRange{Start: 52, End: len(source)},
		},
	}

	patterns := []*regexp.Regexp{regexp.MustCompile(`^追加機能:`)}
	result := parser.SplitLongParagraphs(blocks, patterns, source)

	if len(result) != 4 {
		t.Fatalf("expected 4 blocks (heading + 2 split paragraphs + list), got %d", len(result))
	}
	if result[0].Kind != model.BlockHeading {
		t.Errorf("block 0: expected heading, got %s", result[0].Kind)
	}
	if result[3].Kind != model.BlockList {
		t.Errorf("block 3: expected list, got %s", result[3].Kind)
	}
}

func TestSplitLongParagraphs_PreservesHeadingPath(t *testing.T) {
	source := []byte("追加機能: ラベル\n背景テキスト\n")
	blocks := []model.Block{
		{
			Kind:        model.BlockParagraph,
			Text:        "追加機能: ラベル\n背景テキスト\n",
			HeadingPath: []string{"API仕様"},
			Range:       model.SourceRange{Start: 0, End: len(source)},
		},
	}

	patterns := []*regexp.Regexp{regexp.MustCompile(`^追加機能:`)}
	result := parser.SplitLongParagraphs(blocks, patterns, source)

	if len(result) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(result))
	}
	// Both split blocks should inherit the HeadingPath
	for i, b := range result {
		if len(b.HeadingPath) != 1 || b.HeadingPath[0] != "API仕様" {
			t.Errorf("block %d: expected HeadingPath [API仕様], got %v", i, b.HeadingPath)
		}
	}
}

func TestSplitLongParagraphs_PrecedingTextBeforeMatch(t *testing.T) {
	// Text before the matching line should form its own block
	source := []byte("前のテキスト\n追加機能: ラベル\n後続テキスト\n")
	blocks := []model.Block{
		{
			Kind:  model.BlockParagraph,
			Text:  "前のテキスト\n追加機能: ラベル\n後続テキスト\n",
			Range: model.SourceRange{Start: 0, End: len(source)},
		},
	}

	patterns := []*regexp.Regexp{regexp.MustCompile(`^追加機能:`)}
	result := parser.SplitLongParagraphs(blocks, patterns, source)

	if len(result) != 3 {
		t.Fatalf("expected 3 blocks (before + match + after), got %d", len(result))
	}
	if result[0].Text != "前のテキスト" {
		t.Errorf("block 0: expected '前のテキスト', got %q", result[0].Text)
	}
}
