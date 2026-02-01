package parser_test

import (
	"testing"

	"github.com/thirdlf03/kire/internal/model"
	"github.com/thirdlf03/kire/internal/parser"
)

func enabledConfig() parser.PseudoHeadingConfig {
	cfg := parser.DefaultPseudoHeadingConfig()
	cfg.Enabled = true
	return cfg
}

func TestDetectPseudoHeadings_ColonSuffix(t *testing.T) {
	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "リクエスト:"},
		{Kind: model.BlockParagraph, Text: "Some body text here."},
	}
	blocks = parser.DetectPseudoHeadings(blocks, enabledConfig())
	if blocks[0].Kind != model.BlockHeading {
		t.Errorf("expected heading, got %s", blocks[0].Kind)
	}
	if blocks[0].HeadingLevel != 7 {
		t.Errorf("expected level 7, got %d", blocks[0].HeadingLevel)
	}
	if !blocks[0].IsPseudoHeading {
		t.Error("expected IsPseudoHeading=true")
	}
	// Body should have heading path referencing the pseudo heading
	if len(blocks[1].HeadingPath) != 1 || blocks[1].HeadingPath[0] != "リクエスト:" {
		t.Errorf("expected heading path [リクエスト:], got %v", blocks[1].HeadingPath)
	}
}

func TestDetectPseudoHeadings_FullWidthColon(t *testing.T) {
	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "リクエスト："},
		{Kind: model.BlockParagraph, Text: "Body text."},
	}
	blocks = parser.DetectPseudoHeadings(blocks, enabledConfig())
	if blocks[0].Kind != model.BlockHeading {
		t.Errorf("expected heading, got %s", blocks[0].Kind)
	}
}

func TestDetectPseudoHeadings_NumberedItem(t *testing.T) {
	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "1) GET /api/tasks"},
		{Kind: model.BlockParagraph, Text: "Returns all tasks."},
	}
	blocks = parser.DetectPseudoHeadings(blocks, enabledConfig())
	if blocks[0].Kind != model.BlockHeading {
		t.Errorf("expected heading, got %s", blocks[0].Kind)
	}
	if blocks[0].HeadingLevel != 7 {
		t.Errorf("expected level 7, got %d", blocks[0].HeadingLevel)
	}
}

func TestDetectPseudoHeadings_TooLong(t *testing.T) {
	// Line > 40 runes should not be promoted
	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "This is a very long paragraph line that exceeds the forty rune limit for detection:"},
	}
	blocks = parser.DetectPseudoHeadings(blocks, enabledConfig())
	if blocks[0].Kind != model.BlockParagraph {
		t.Errorf("expected paragraph (too long), got %s", blocks[0].Kind)
	}
}

func TestDetectPseudoHeadings_NotParagraph(t *testing.T) {
	// Non-paragraph blocks should not be promoted
	blocks := []model.Block{
		{Kind: model.BlockList, Text: "item:"},
	}
	blocks = parser.DetectPseudoHeadings(blocks, enabledConfig())
	if blocks[0].Kind != model.BlockList {
		t.Errorf("expected list unchanged, got %s", blocks[0].Kind)
	}
}

func TestDetectPseudoHeadings_Disabled(t *testing.T) {
	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "リクエスト:"},
	}
	cfg := parser.DefaultPseudoHeadingConfig()
	cfg.Enabled = false
	blocks = parser.DetectPseudoHeadings(blocks, cfg)
	if blocks[0].Kind != model.BlockParagraph {
		t.Errorf("expected paragraph when disabled, got %s", blocks[0].Kind)
	}
}

func TestDetectPseudoHeadings_HeadingPathRebuild(t *testing.T) {
	// Real heading + pseudo heading: paths should nest correctly
	blocks := []model.Block{
		{Kind: model.BlockHeading, Text: "API Reference", HeadingLevel: 2},
		{Kind: model.BlockParagraph, Text: "リクエスト:"},
		{Kind: model.BlockParagraph, Text: "GET /api/tasks"},
	}
	blocks = parser.DetectPseudoHeadings(blocks, enabledConfig())

	// blocks[1] promoted to heading level 7
	if blocks[1].Kind != model.BlockHeading {
		t.Fatalf("expected pseudo heading, got %s", blocks[1].Kind)
	}
	// Pseudo heading's path = ["API Reference"]
	if len(blocks[1].HeadingPath) != 1 || blocks[1].HeadingPath[0] != "API Reference" {
		t.Errorf("pseudo heading path: expected [API Reference], got %v", blocks[1].HeadingPath)
	}
	// Body under pseudo heading: path = ["API Reference", "リクエスト:"]
	expected := []string{"API Reference", "リクエスト:"}
	if len(blocks[2].HeadingPath) != 2 {
		t.Fatalf("body heading path: expected %v, got %v", expected, blocks[2].HeadingPath)
	}
	for i, e := range expected {
		if blocks[2].HeadingPath[i] != e {
			t.Errorf("body heading path[%d]: expected %q, got %q", i, e, blocks[2].HeadingPath[i])
		}
	}
}

func TestDetectPseudoHeadings_Empty(t *testing.T) {
	blocks := parser.DetectPseudoHeadings(nil, enabledConfig())
	if len(blocks) != 0 {
		t.Errorf("expected nil/empty, got %d blocks", len(blocks))
	}
}

func TestDetectPseudoHeadings_ExcludedPattern(t *testing.T) {
	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "バリデーション:"},
	}
	blocks = parser.DetectPseudoHeadings(blocks, enabledConfig())
	if blocks[0].Kind != model.BlockParagraph {
		t.Errorf("expected paragraph (excluded), got %s", blocks[0].Kind)
	}
}

func TestDetectPseudoHeadings_ExcludedResponse(t *testing.T) {
	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "レスポンス(成功時):"},
	}
	blocks = parser.DetectPseudoHeadings(blocks, enabledConfig())
	if blocks[0].Kind != model.BlockParagraph {
		t.Errorf("expected paragraph (excluded by partial match), got %s", blocks[0].Kind)
	}
}

func TestDetectPseudoHeadings_ExcludedProcessing(t *testing.T) {
	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "処理:"},
	}
	blocks = parser.DetectPseudoHeadings(blocks, enabledConfig())
	if blocks[0].Kind != model.BlockParagraph {
		t.Errorf("expected paragraph (excluded), got %s", blocks[0].Kind)
	}
}

func TestDetectPseudoHeadings_NonExcludedPromoted(t *testing.T) {
	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "追加機能:"},
	}
	blocks = parser.DetectPseudoHeadings(blocks, enabledConfig())
	if blocks[0].Kind != model.BlockHeading {
		t.Errorf("expected heading (not excluded), got %s", blocks[0].Kind)
	}
}

func TestDetectPseudoHeadings_EmptyExcludeList(t *testing.T) {
	cfg := enabledConfig()
	cfg.ExcludePatterns = nil
	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "バリデーション:"},
	}
	blocks = parser.DetectPseudoHeadings(blocks, cfg)
	if blocks[0].Kind != model.BlockHeading {
		t.Errorf("expected heading (no exclude list), got %s", blocks[0].Kind)
	}
}

func TestDetectPseudoHeadings_PrefixPattern(t *testing.T) {
	// PrefixPatterns should match lines starting with the pattern,
	// bypassing the MaxRuneLen check.
	cfg := enabledConfig()
	cfg.PrefixPatterns = []string{`^追加機能:`}
	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "追加機能: ラベル管理（Label）"},
		{Kind: model.BlockParagraph, Text: "Some body text."},
	}
	blocks = parser.DetectPseudoHeadings(blocks, cfg)
	if blocks[0].Kind != model.BlockHeading {
		t.Errorf("expected heading via PrefixPattern, got %s", blocks[0].Kind)
	}
	if !blocks[0].IsPseudoHeading {
		t.Error("expected IsPseudoHeading=true")
	}
	if blocks[0].HeadingLevel != 7 {
		t.Errorf("expected level 7, got %d", blocks[0].HeadingLevel)
	}
}

func TestDetectPseudoHeadings_PrefixPatternBypassesMaxRuneLen(t *testing.T) {
	// Even if the text exceeds MaxRuneLen, PrefixPattern match should promote it
	cfg := enabledConfig()
	cfg.PrefixPatterns = []string{`^追加機能:`}
	cfg.MaxRuneLen = 10 // Very short limit
	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "追加機能: これは非常に長いテキストです"},
	}
	blocks = parser.DetectPseudoHeadings(blocks, cfg)
	if blocks[0].Kind != model.BlockHeading {
		t.Errorf("PrefixPattern should bypass MaxRuneLen, got %s", blocks[0].Kind)
	}
}

func TestDetectPseudoHeadings_PrefixPatternWithExclude(t *testing.T) {
	// PrefixPattern matches, but ExcludePatterns should still exclude
	cfg := enabledConfig()
	cfg.PrefixPatterns = []string{`^バリデーション`}
	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "バリデーション: 入力チェック"},
	}
	blocks = parser.DetectPseudoHeadings(blocks, cfg)
	if blocks[0].Kind != model.BlockParagraph {
		t.Errorf("expected paragraph (excluded despite PrefixPattern match), got %s", blocks[0].Kind)
	}
}

func TestDetectPseudoHeadings_PrefixPatternNonParagraph(t *testing.T) {
	// Non-paragraph blocks should not be affected by PrefixPatterns
	cfg := enabledConfig()
	cfg.PrefixPatterns = []string{`^追加機能:`}
	blocks := []model.Block{
		{Kind: model.BlockList, Text: "追加機能: item"},
	}
	blocks = parser.DetectPseudoHeadings(blocks, cfg)
	if blocks[0].Kind != model.BlockList {
		t.Errorf("expected list unchanged, got %s", blocks[0].Kind)
	}
}

func TestIsPseudoHeadingCandidate_ColonSuffix(t *testing.T) {
	cfg := enabledConfig()
	prefixRes := parser.CompilePrefixPatterns(cfg.PrefixPatterns)
	if !parser.IsPseudoHeadingCandidate("リクエスト:", cfg, prefixRes) {
		t.Error("expected true for colon suffix")
	}
}

func TestIsPseudoHeadingCandidate_FullWidthColon(t *testing.T) {
	cfg := enabledConfig()
	prefixRes := parser.CompilePrefixPatterns(cfg.PrefixPatterns)
	if !parser.IsPseudoHeadingCandidate("リクエスト：", cfg, prefixRes) {
		t.Error("expected true for full-width colon suffix")
	}
}

func TestIsPseudoHeadingCandidate_NumberedItem(t *testing.T) {
	cfg := enabledConfig()
	prefixRes := parser.CompilePrefixPatterns(cfg.PrefixPatterns)
	if !parser.IsPseudoHeadingCandidate("1) GET /api/tasks", cfg, prefixRes) {
		t.Error("expected true for numbered item")
	}
}

func TestIsPseudoHeadingCandidate_TooLong(t *testing.T) {
	cfg := enabledConfig()
	prefixRes := parser.CompilePrefixPatterns(cfg.PrefixPatterns)
	if parser.IsPseudoHeadingCandidate("This is a very long paragraph line that exceeds the forty rune limit for detection:", cfg, prefixRes) {
		t.Error("expected false for too-long line")
	}
}

func TestIsPseudoHeadingCandidate_Excluded(t *testing.T) {
	cfg := enabledConfig()
	prefixRes := parser.CompilePrefixPatterns(cfg.PrefixPatterns)
	if parser.IsPseudoHeadingCandidate("バリデーション:", cfg, prefixRes) {
		t.Error("expected false for excluded pattern")
	}
}

func TestIsPseudoHeadingCandidate_Empty(t *testing.T) {
	cfg := enabledConfig()
	prefixRes := parser.CompilePrefixPatterns(cfg.PrefixPatterns)
	if parser.IsPseudoHeadingCandidate("", cfg, prefixRes) {
		t.Error("expected false for empty text")
	}
}

func TestIsPseudoHeadingCandidate_PrefixBypassesMaxLen(t *testing.T) {
	cfg := enabledConfig()
	cfg.PrefixPatterns = []string{`^追加機能:`}
	cfg.MaxRuneLen = 5
	prefixRes := parser.CompilePrefixPatterns(cfg.PrefixPatterns)
	if !parser.IsPseudoHeadingCandidate("追加機能: ラベル管理（Label）", cfg, prefixRes) {
		t.Error("expected true: prefix pattern should bypass MaxRuneLen")
	}
}

func TestAutoSplitParagraphs_SplitsAtCandidate(t *testing.T) {
	// A multi-line paragraph with a pseudo-heading candidate line should be split
	source := []byte("背景テキスト\n追加機能:\n詳細テキスト\n")
	blocks := []model.Block{
		{
			Kind:  model.BlockParagraph,
			Text:  "背景テキスト\n追加機能:\n詳細テキスト",
			Range: model.SourceRange{Start: 0, End: len(source)},
		},
	}
	cfg := enabledConfig()
	result := parser.AutoSplitParagraphs(blocks, cfg, source)

	if len(result) != 3 {
		t.Fatalf("expected 3 blocks, got %d", len(result))
	}
	if result[0].Text != "背景テキスト" {
		t.Errorf("block 0: expected '背景テキスト', got %q", result[0].Text)
	}
	if result[1].Text != "追加機能:" {
		t.Errorf("block 1: expected '追加機能:', got %q", result[1].Text)
	}
	if result[2].Text != "詳細テキスト" {
		t.Errorf("block 2: expected '詳細テキスト', got %q", result[2].Text)
	}
}

func TestAutoSplitParagraphs_SingleLine_NoChange(t *testing.T) {
	source := []byte("単一行テキスト\n")
	blocks := []model.Block{
		{
			Kind:  model.BlockParagraph,
			Text:  "単一行テキスト",
			Range: model.SourceRange{Start: 0, End: len(source)},
		},
	}
	cfg := enabledConfig()
	result := parser.AutoSplitParagraphs(blocks, cfg, source)

	if len(result) != 1 {
		t.Fatalf("expected 1 block, got %d", len(result))
	}
}

func TestAutoSplitParagraphs_Disabled_NoChange(t *testing.T) {
	source := []byte("背景テキスト\n追加機能:\n詳細テキスト\n")
	blocks := []model.Block{
		{
			Kind:  model.BlockParagraph,
			Text:  "背景テキスト\n追加機能:\n詳細テキスト",
			Range: model.SourceRange{Start: 0, End: len(source)},
		},
	}
	cfg := enabledConfig()
	cfg.Enabled = false
	result := parser.AutoSplitParagraphs(blocks, cfg, source)

	if len(result) != 1 {
		t.Fatalf("expected 1 block (disabled), got %d", len(result))
	}
}

func TestAutoSplitParagraphs_NonParagraph_Unchanged(t *testing.T) {
	source := []byte("- 追加機能:\n- 詳細\n")
	blocks := []model.Block{
		{
			Kind:  model.BlockList,
			Text:  "追加機能:\n詳細",
			Range: model.SourceRange{Start: 0, End: len(source)},
		},
	}
	cfg := enabledConfig()
	result := parser.AutoSplitParagraphs(blocks, cfg, source)

	if len(result) != 1 {
		t.Fatalf("expected 1 block (non-paragraph), got %d", len(result))
	}
}

func TestAutoSplitParagraphs_SourceRangeConsistency(t *testing.T) {
	source := []byte("テキスト1\n追加機能:\nテキスト2\n")
	blocks := []model.Block{
		{
			Kind:  model.BlockParagraph,
			Text:  "テキスト1\n追加機能:\nテキスト2",
			Range: model.SourceRange{Start: 0, End: len(source)},
		},
	}
	cfg := enabledConfig()
	result := parser.AutoSplitParagraphs(blocks, cfg, source)

	if len(result) < 2 {
		t.Fatalf("expected multiple blocks, got %d", len(result))
	}

	// Ranges should tile
	for i := 0; i < len(result)-1; i++ {
		if result[i].Range.End != result[i+1].Range.Start {
			t.Errorf("blocks %d and %d don't tile: end=%d, start=%d",
				i, i+1, result[i].Range.End, result[i+1].Range.Start)
		}
	}

	// Combined range should match original
	if result[0].Range.Start != 0 {
		t.Errorf("first block start should be 0, got %d", result[0].Range.Start)
	}
	if result[len(result)-1].Range.End != len(source) {
		t.Errorf("last block end should be %d, got %d", len(source), result[len(result)-1].Range.End)
	}
}

func TestAutoSplitParagraphs_NoCandidates_NoSplit(t *testing.T) {
	source := []byte("普通のテキスト。\n次の行です。\nまだ続きます。\n")
	blocks := []model.Block{
		{
			Kind:  model.BlockParagraph,
			Text:  "普通のテキスト。\n次の行です。\nまだ続きます。",
			Range: model.SourceRange{Start: 0, End: len(source)},
		},
	}
	cfg := enabledConfig()
	result := parser.AutoSplitParagraphs(blocks, cfg, source)

	if len(result) != 1 {
		t.Fatalf("expected 1 block (no candidates), got %d", len(result))
	}
}

func TestDetectPseudoHeadings_MultiplePrefixPatterns(t *testing.T) {
	cfg := enabledConfig()
	cfg.PrefixPatterns = []string{`^追加機能:`, `^共通仕様`}
	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "追加機能: ラベル管理（Label）"},
		{Kind: model.BlockParagraph, Text: "共通仕様"},
		{Kind: model.BlockParagraph, Text: "普通のテキスト"},
	}
	blocks = parser.DetectPseudoHeadings(blocks, cfg)
	if blocks[0].Kind != model.BlockHeading {
		t.Errorf("block 0: expected heading, got %s", blocks[0].Kind)
	}
	if blocks[1].Kind != model.BlockHeading {
		t.Errorf("block 1: expected heading, got %s", blocks[1].Kind)
	}
	if blocks[2].Kind != model.BlockParagraph {
		t.Errorf("block 2: expected paragraph, got %s", blocks[2].Kind)
	}
}
