package segment_test

import (
	"testing"

	"github.com/thirdlf03/kire/internal/model"
	"github.com/thirdlf03/kire/internal/segment"
)

func enabledLockConfig() segment.LockConfig {
	cfg := segment.DefaultLockConfig()
	cfg.Enabled = true
	return cfg
}

func TestDetectLockRanges_Basic(t *testing.T) {
	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "リクエスト:"},
		{Kind: model.BlockParagraph, Text: "GET /api/tasks"},
		{Kind: model.BlockParagraph, Text: "Returns all tasks."},
		{Kind: model.BlockHeading, Text: "Next Section", HeadingLevel: 2},
	}
	ranges := segment.DetectLockRanges(blocks, enabledLockConfig())
	if len(ranges) != 1 {
		t.Fatalf("expected 1 lock range, got %d", len(ranges))
	}
	if ranges[0].Start != 0 || ranges[0].End != 3 {
		t.Errorf("expected range [0,3), got [%d,%d)", ranges[0].Start, ranges[0].End)
	}
}

func TestDetectLockRanges_FullWidthColon(t *testing.T) {
	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "リクエスト："},
		{Kind: model.BlockParagraph, Text: "Body text"},
	}
	ranges := segment.DetectLockRanges(blocks, enabledLockConfig())
	if len(ranges) != 1 {
		t.Fatalf("expected 1 lock range, got %d", len(ranges))
	}
}

func TestDetectLockRanges_StopsAtHeading(t *testing.T) {
	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "Lead:"},
		{Kind: model.BlockParagraph, Text: "Body 1"},
		{Kind: model.BlockHeading, Text: "H2", HeadingLevel: 2},
		{Kind: model.BlockParagraph, Text: "Body 2"},
	}
	cfg := enabledLockConfig()
	cfg.LockAfterHeading = false // Heading should stop lock, not start one
	ranges := segment.DetectLockRanges(blocks, cfg)
	if len(ranges) != 1 {
		t.Fatalf("expected 1 lock range, got %d", len(ranges))
	}
	if ranges[0].End != 2 {
		t.Errorf("expected range end at 2 (before heading), got %d", ranges[0].End)
	}
}

func TestDetectLockRanges_StopsAtNextLead(t *testing.T) {
	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "Lead 1:"},
		{Kind: model.BlockParagraph, Text: "Body 1"},
		{Kind: model.BlockParagraph, Text: "Lead 2:"},
		{Kind: model.BlockParagraph, Text: "Body 2"},
	}
	ranges := segment.DetectLockRanges(blocks, enabledLockConfig())
	if len(ranges) != 2 {
		t.Fatalf("expected 2 lock ranges, got %d", len(ranges))
	}
	if ranges[0].Start != 0 || ranges[0].End != 2 {
		t.Errorf("range 0: expected [0,2), got [%d,%d)", ranges[0].Start, ranges[0].End)
	}
	if ranges[1].Start != 2 || ranges[1].End != 4 {
		t.Errorf("range 1: expected [2,4), got [%d,%d)", ranges[1].Start, ranges[1].End)
	}
}

func TestDetectLockRanges_MaxBlocks(t *testing.T) {
	blocks := make([]model.Block, 15)
	blocks[0] = model.Block{Kind: model.BlockParagraph, Text: "Lead:"}
	for i := 1; i < 15; i++ {
		blocks[i] = model.Block{Kind: model.BlockParagraph, Text: "Body"}
	}
	cfg := enabledLockConfig()
	cfg.MaxLockBlocks = 5
	ranges := segment.DetectLockRanges(blocks, cfg)
	if len(ranges) == 0 {
		t.Fatal("expected at least 1 lock range")
	}
	if ranges[0].End-ranges[0].Start > 5 {
		t.Errorf("lock range exceeds MaxLockBlocks: [%d,%d)", ranges[0].Start, ranges[0].End)
	}
}

func TestDetectLockRanges_Disabled(t *testing.T) {
	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "Lead:"},
		{Kind: model.BlockParagraph, Text: "Body"},
	}
	cfg := segment.DefaultLockConfig()
	cfg.Enabled = false
	ranges := segment.DetectLockRanges(blocks, cfg)
	if len(ranges) != 0 {
		t.Errorf("expected no ranges when disabled, got %d", len(ranges))
	}
}

func TestDetectLockRanges_LeadAlone(t *testing.T) {
	// Lead at last position → no body → no lock range
	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "Lead:"},
	}
	ranges := segment.DetectLockRanges(blocks, enabledLockConfig())
	if len(ranges) != 0 {
		t.Errorf("expected no lock range for lone lead, got %d", len(ranges))
	}
}

func TestDetectLockRanges_NumberedHTTPLead(t *testing.T) {
	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "1) GET /api/tasks"},
		{Kind: model.BlockParagraph, Text: "Returns all tasks."},
	}
	ranges := segment.DetectLockRanges(blocks, enabledLockConfig())
	if len(ranges) != 1 {
		t.Fatalf("expected 1 lock range, got %d", len(ranges))
	}
	if ranges[0].Start != 0 || ranges[0].End != 2 {
		t.Errorf("expected range [0,2), got [%d,%d)", ranges[0].Start, ranges[0].End)
	}
}

func TestDetectLockRanges_NumberedNonHTTP_NotLead(t *testing.T) {
	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "1) Some regular text"},
		{Kind: model.BlockParagraph, Text: "More text."},
	}
	ranges := segment.DetectLockRanges(blocks, enabledLockConfig())
	if len(ranges) != 0 {
		t.Errorf("expected no lock range for non-HTTP numbered text, got %d", len(ranges))
	}
}

func TestDetectLockRanges_HeadingFollowedByList(t *testing.T) {
	cfg := enabledLockConfig()
	cfg.LockAfterHeading = true
	blocks := []model.Block{
		{Kind: model.BlockHeading, Text: "Projectデータ定義", HeadingLevel: 3},
		{Kind: model.BlockList, Text: "- id: string\n- name: string"},
		{Kind: model.BlockList, Text: "- status: active"},
		{Kind: model.BlockParagraph, Text: "次のセクション"},
	}
	ranges := segment.DetectLockRanges(blocks, cfg)
	if len(ranges) != 1 {
		t.Fatalf("expected 1 lock range, got %d", len(ranges))
	}
	// Lock extends from heading through all following non-heading/non-lead blocks
	if ranges[0].Start != 0 || ranges[0].End != 4 {
		t.Errorf("expected range [0,4), got [%d,%d)", ranges[0].Start, ranges[0].End)
	}
}

func TestDetectLockRanges_PseudoHeadingFollowedByList(t *testing.T) {
	cfg := enabledLockConfig()
	cfg.LockAfterHeading = true
	blocks := []model.Block{
		{Kind: model.BlockHeading, Text: "追加機能:", HeadingLevel: 7, IsPseudoHeading: true},
		{Kind: model.BlockList, Text: "- item1\n- item2"},
		{Kind: model.BlockHeading, Text: "次の見出し", HeadingLevel: 2},
	}
	ranges := segment.DetectLockRanges(blocks, cfg)
	if len(ranges) != 1 {
		t.Fatalf("expected 1 lock range, got %d", len(ranges))
	}
	if ranges[0].Start != 0 || ranges[0].End != 2 {
		t.Errorf("expected range [0,2), got [%d,%d)", ranges[0].Start, ranges[0].End)
	}
}

func TestDetectLockRanges_HeadingFollowedByParagraph(t *testing.T) {
	cfg := enabledLockConfig()
	cfg.LockAfterHeading = true
	blocks := []model.Block{
		{Kind: model.BlockHeading, Text: "概要", HeadingLevel: 2},
		{Kind: model.BlockParagraph, Text: "この章では..."},
		{Kind: model.BlockParagraph, Text: "さらに..."},
		{Kind: model.BlockHeading, Text: "詳細", HeadingLevel: 2},
	}
	ranges := segment.DetectLockRanges(blocks, cfg)
	if len(ranges) != 1 {
		t.Fatalf("expected 1 lock range, got %d", len(ranges))
	}
	if ranges[0].Start != 0 || ranges[0].End != 3 {
		t.Errorf("expected range [0,3), got [%d,%d)", ranges[0].Start, ranges[0].End)
	}
}

func TestDetectLockRanges_HeadingFollowedByHeading_NoLock(t *testing.T) {
	cfg := enabledLockConfig()
	cfg.LockAfterHeading = true
	blocks := []model.Block{
		{Kind: model.BlockHeading, Text: "H1", HeadingLevel: 1},
		{Kind: model.BlockHeading, Text: "H2", HeadingLevel: 2},
	}
	// Heading→Heading: the second heading stops the lock, no body → no lock range
	ranges := segment.DetectLockRanges(blocks, cfg)
	if len(ranges) != 0 {
		t.Errorf("expected no lock range for heading→heading, got %d", len(ranges))
	}
}

func TestDetectLockRanges_LockAfterHeadingDisabled(t *testing.T) {
	cfg := enabledLockConfig()
	cfg.LockAfterHeading = false
	blocks := []model.Block{
		{Kind: model.BlockHeading, Text: "見出し", HeadingLevel: 2},
		{Kind: model.BlockList, Text: "- item"},
	}
	// Without LockAfterHeading, headings are not lead lines (no colon suffix)
	ranges := segment.DetectLockRanges(blocks, cfg)
	if len(ranges) != 0 {
		t.Errorf("expected no lock range when LockAfterHeading=false, got %d", len(ranges))
	}
}

func TestLockedGaps(t *testing.T) {
	ranges := []segment.LockRange{
		{Start: 2, End: 5},
	}
	gaps := segment.LockedGaps(ranges)
	// Gaps 2, 3, 4 should be locked (between blocks 2-3, 3-4, 4-5)
	// Actually: gaps within the range are Start to End-2: 2, 3
	for _, g := range []int{2, 3} {
		if !gaps[g] {
			t.Errorf("expected gap %d to be locked", g)
		}
	}
	// Gap outside range
	if gaps[1] {
		t.Error("gap 1 should not be locked")
	}
	if gaps[5] {
		t.Error("gap 5 should not be locked")
	}
}
