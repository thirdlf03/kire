package boundary_test

import (
	"testing"

	"github.com/thirdlf03/kire/internal/boundary"
	"github.com/thirdlf03/kire/internal/model"
)

func TestGenerateHints_ColonProhibit(t *testing.T) {
	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "リクエスト:"},
		{Kind: model.BlockParagraph, Text: "GET /api/tasks"},
	}
	hints := boundary.GenerateHints(blocks, boundary.DefaultHintRules())
	// Should have a prohibit hint at gap 0 (after "リクエスト:")
	found := false
	for _, h := range hints {
		if h.Kind == boundary.HintProhibit && h.GapIndex == 0 {
			found = true
		}
	}
	if !found {
		t.Errorf("expected prohibit hint at gap 0, hints=%v", hints)
	}
}

func TestGenerateHints_FullWidthColonProhibit(t *testing.T) {
	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "リクエスト："},
		{Kind: model.BlockParagraph, Text: "Body text"},
	}
	hints := boundary.GenerateHints(blocks, boundary.DefaultHintRules())
	found := false
	for _, h := range hints {
		if h.Kind == boundary.HintProhibit && h.GapIndex == 0 {
			found = true
		}
	}
	if !found {
		t.Errorf("expected prohibit hint for full-width colon, hints=%v", hints)
	}
}

func TestGenerateHints_APIEndpointPrefer(t *testing.T) {
	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "Some text before."},
		{Kind: model.BlockParagraph, Text: "1) GET /api/tasks"},
		{Kind: model.BlockParagraph, Text: "Returns all tasks."},
	}
	hints := boundary.GenerateHints(blocks, boundary.DefaultHintRules())
	// Should prefer a boundary at gap 0 (before "1) GET /api/tasks")
	found := false
	for _, h := range hints {
		if h.Kind == boundary.HintPrefer && h.GapIndex == 0 {
			found = true
		}
	}
	if !found {
		t.Errorf("expected prefer hint at gap 0, hints=%v", hints)
	}
}

func TestGenerateHints_BareHTTPVerb(t *testing.T) {
	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "Previous section."},
		{Kind: model.BlockParagraph, Text: "GET /api/users"},
	}
	hints := boundary.GenerateHints(blocks, boundary.DefaultHintRules())
	found := false
	for _, h := range hints {
		if h.Kind == boundary.HintPrefer && h.GapIndex == 0 {
			found = true
		}
	}
	if !found {
		t.Errorf("expected prefer hint for bare HTTP verb, hints=%v", hints)
	}
}

func TestGenerateHints_NoMatch(t *testing.T) {
	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "Normal paragraph."},
		{Kind: model.BlockParagraph, Text: "Another paragraph."},
	}
	hints := boundary.GenerateHints(blocks, boundary.DefaultHintRules())
	if len(hints) != 0 {
		t.Errorf("expected no hints, got %v", hints)
	}
}

func TestGenerateHints_BoundaryEdge(t *testing.T) {
	// Prefer at first block (offset -1 → gap -1 → out of range, should be skipped)
	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "GET /api/tasks"},
		{Kind: model.BlockParagraph, Text: "Body."},
	}
	hints := boundary.GenerateHints(blocks, boundary.DefaultHintRules())
	// No prefer hint because gap index would be -1
	for _, h := range hints {
		if h.Kind == boundary.HintPrefer {
			t.Errorf("should not prefer at out-of-range gap, hints=%v", hints)
		}
	}
}

func TestUserEnforceRules_Basic(t *testing.T) {
	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "普通のテキスト"},
		{Kind: model.BlockParagraph, Text: "追加機能: ラベル管理"},
		{Kind: model.BlockParagraph, Text: "本文テキスト"},
	}
	rules := boundary.UserEnforceRules([]string{`^追加機能:`})
	hints := boundary.GenerateHints(blocks, rules)

	found := false
	for _, h := range hints {
		if h.Kind == boundary.HintEnforce && h.GapIndex == 0 {
			found = true
		}
	}
	if !found {
		t.Errorf("expected enforce hint at gap 0 (before matching block), hints=%v", hints)
	}
}

func TestUserEnforceRules_NoMatch(t *testing.T) {
	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "普通のテキスト"},
		{Kind: model.BlockParagraph, Text: "別のテキスト"},
	}
	rules := boundary.UserEnforceRules([]string{`^追加機能:`})
	hints := boundary.GenerateHints(blocks, rules)
	if len(hints) != 0 {
		t.Errorf("expected no hints, got %v", hints)
	}
}

func TestUserEnforceRules_InvalidRegex(t *testing.T) {
	// Invalid regex should be silently skipped
	rules := boundary.UserEnforceRules([]string{`[invalid`})
	if len(rules) != 0 {
		t.Errorf("expected no rules for invalid regex, got %d", len(rules))
	}
}

func TestUserEnforceRules_MultiplePatterns(t *testing.T) {
	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "テキスト"},
		{Kind: model.BlockParagraph, Text: "追加機能: ラベル"},
		{Kind: model.BlockParagraph, Text: "中間テキスト"},
		{Kind: model.BlockParagraph, Text: "共通仕様"},
		{Kind: model.BlockParagraph, Text: "末尾テキスト"},
	}
	rules := boundary.UserEnforceRules([]string{`^追加機能:`, `^共通仕様`})
	hints := boundary.GenerateHints(blocks, rules)

	enforceGaps := make(map[int]bool)
	for _, h := range hints {
		if h.Kind == boundary.HintEnforce {
			enforceGaps[h.GapIndex] = true
		}
	}
	if !enforceGaps[0] {
		t.Error("expected enforce at gap 0")
	}
	if !enforceGaps[2] {
		t.Error("expected enforce at gap 2")
	}
}

func TestUserEnforceRules_EmptyPatterns(t *testing.T) {
	rules := boundary.UserEnforceRules(nil)
	if len(rules) != 0 {
		t.Errorf("expected no rules for empty patterns, got %d", len(rules))
	}
}

func TestListBoundaryRules_ListToList(t *testing.T) {
	blocks := []model.Block{
		{Kind: model.BlockList, Text: "- item1"},
		{Kind: model.BlockList, Text: "- item2"},
		{Kind: model.BlockList, Text: "- item3"},
	}
	hints := boundary.GenerateListHints(blocks)

	// Should prohibit gaps 0 and 1 (between consecutive lists)
	prohibitGaps := make(map[int]bool)
	for _, h := range hints {
		if h.Kind == boundary.HintProhibit {
			prohibitGaps[h.GapIndex] = true
		}
	}
	if !prohibitGaps[0] {
		t.Error("expected prohibit at gap 0 (list→list)")
	}
	if !prohibitGaps[1] {
		t.Error("expected prohibit at gap 1 (list→list)")
	}
}

func TestListBoundaryRules_HeadingToList(t *testing.T) {
	blocks := []model.Block{
		{Kind: model.BlockHeading, Text: "見出し", HeadingLevel: 2},
		{Kind: model.BlockList, Text: "- item"},
	}
	hints := boundary.GenerateListHints(blocks)

	found := false
	for _, h := range hints {
		if h.Kind == boundary.HintProhibit && h.GapIndex == 0 {
			found = true
		}
	}
	if !found {
		t.Error("expected prohibit at gap 0 (heading→list)")
	}
}

func TestListBoundaryRules_ListToParagraph_NoProhibit(t *testing.T) {
	blocks := []model.Block{
		{Kind: model.BlockList, Text: "- item"},
		{Kind: model.BlockParagraph, Text: "paragraph"},
	}
	hints := boundary.GenerateListHints(blocks)

	for _, h := range hints {
		if h.Kind == boundary.HintProhibit {
			t.Errorf("should not prohibit list→paragraph, got hint %v", h)
		}
	}
}

func TestListBoundaryRules_ParagraphToList_NoProhibit(t *testing.T) {
	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "paragraph"},
		{Kind: model.BlockList, Text: "- item"},
	}
	hints := boundary.GenerateListHints(blocks)

	for _, h := range hints {
		if h.Kind == boundary.HintProhibit {
			t.Errorf("should not prohibit paragraph→list, got hint %v", h)
		}
	}
}

func TestGenerateAtomicHints_CodeBlock(t *testing.T) {
	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "Some code:"},
		{Kind: model.BlockCodeBlock, Text: "fmt.Println()"},
		{Kind: model.BlockParagraph, Text: "After code."},
	}
	hints := boundary.GenerateAtomicHints(blocks)
	if len(hints) != 1 {
		t.Fatalf("expected 1 hint, got %d", len(hints))
	}
	if hints[0].GapIndex != 0 || hints[0].Kind != boundary.HintProhibit {
		t.Errorf("expected prohibit at gap 0, got %v", hints[0])
	}
}

func TestGenerateAtomicHints_Table(t *testing.T) {
	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "Table:"},
		{Kind: model.BlockTable, Text: "| A | B |"},
	}
	hints := boundary.GenerateAtomicHints(blocks)
	if len(hints) != 1 {
		t.Fatalf("expected 1 hint, got %d", len(hints))
	}
	if hints[0].GapIndex != 0 {
		t.Errorf("expected prohibit at gap 0, got %v", hints[0])
	}
}

func TestGenerateAtomicHints_MathBlock(t *testing.T) {
	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "Formula:"},
		{Kind: model.BlockMathBlock, Text: "x = y"},
	}
	hints := boundary.GenerateAtomicHints(blocks)
	if len(hints) != 1 {
		t.Fatalf("expected 1 hint, got %d", len(hints))
	}
	if hints[0].GapIndex != 0 {
		t.Errorf("expected prohibit at gap 0, got %v", hints[0])
	}
}

func TestGenerateAtomicHints_FirstBlockIsCode(t *testing.T) {
	blocks := []model.Block{
		{Kind: model.BlockCodeBlock, Text: "code"},
		{Kind: model.BlockParagraph, Text: "text"},
	}
	hints := boundary.GenerateAtomicHints(blocks)
	// First block has no preceding gap → no hint
	if len(hints) != 0 {
		t.Errorf("expected 0 hints for first-block code, got %d", len(hints))
	}
}

func TestGenerateAtomicHints_Multiple(t *testing.T) {
	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "intro"},
		{Kind: model.BlockCodeBlock, Text: "code1"},
		{Kind: model.BlockParagraph, Text: "mid"},
		{Kind: model.BlockTable, Text: "table"},
		{Kind: model.BlockParagraph, Text: "end"},
		{Kind: model.BlockMathBlock, Text: "math"},
	}
	hints := boundary.GenerateAtomicHints(blocks)
	if len(hints) != 3 {
		t.Fatalf("expected 3 hints, got %d", len(hints))
	}
	expectedGaps := []int{0, 2, 4}
	for i, h := range hints {
		if h.GapIndex != expectedGaps[i] {
			t.Errorf("hint[%d]: expected gap %d, got %d", i, expectedGaps[i], h.GapIndex)
		}
	}
}

func TestBoundaryDetection_PreferBoost(t *testing.T) {
	// Verify that HintPrefer boosts depth score but doesn't force a boundary
	// Create embeddings that produce uniform similarities (no natural boundaries)
	embs := make([]model.Embedding, 4)
	for i := range embs {
		vec := make([]float64, 3)
		vec[0] = 1.0
		vec[1] = 0.0
		vec[2] = 0.0
		embs[i] = model.Embedding{Vector: vec}
	}
	// With identical embeddings, depth scores are all 0. A prefer hint at gap 1
	// should boost it to 0.1.
	cfg := boundary.ScoringConfig{
		Hints: []boundary.BoundaryHint{
			{GapIndex: 1, Kind: boundary.HintPrefer},
		},
	}
	result := boundary.DetectBoundaries(embs, cfg)
	// Depth score at gap 1 should be boosted
	if len(result.DepthScores) > 1 && result.DepthScores[1] < 0.09 {
		t.Errorf("expected depth score boost at gap 1, got %.4f", result.DepthScores[1])
	}
}
