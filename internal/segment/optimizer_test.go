package segment_test

import (
	"testing"

	"github.com/thirdlf03/kire/internal/boundary"
	"github.com/thirdlf03/kire/internal/model"
	"github.com/thirdlf03/kire/internal/segment"
)

func makeBlocks(n int, tokensEach int) []model.Block {
	blocks := make([]model.Block, n)
	for i := range blocks {
		blocks[i] = model.Block{
			Kind:         model.BlockParagraph,
			Text:         "text",
			TokensApprox: tokensEach,
			Range:        model.SourceRange{Start: i * 100, End: (i + 1) * 100},
		}
	}
	return blocks
}

func makeBlocksWithLines(n int, tokensEach int, linesEach int) []model.Block {
	blocks := makeBlocks(n, tokensEach)
	for i := range blocks {
		blocks[i].LinesApprox = linesEach
	}
	return blocks
}

func makeBlocksWithHeading(headingIdx int, n int, tokensEach int) []model.Block {
	blocks := makeBlocks(n, tokensEach)
	blocks[headingIdx].Kind = model.BlockHeading
	blocks[headingIdx].HeadingLevel = 2
	return blocks
}

func TestOptimize_AllInRange(t *testing.T) {
	blocks := makeBlocks(10, 100)
	br := boundary.BoundaryResult{
		Boundaries:  []int{4}, // Single boundary at index 4
		DepthScores: make([]float64, 9),
	}
	br.DepthScores[4] = 1.0

	config := segment.OptConfig{
		MinTokens: 200,
		MaxTokens: 800,
	}

	segments := segment.Optimize(blocks, br, config)

	// Should produce 2 segments: [0..4] and [5..9]
	if len(segments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(segments))
	}
	// First segment: blocks 0-4 = 500 tokens
	if segments[0].TokenCount != 500 {
		t.Errorf("segment 0 tokens: expected 500, got %d", segments[0].TokenCount)
	}
	// Second segment: blocks 5-9 = 500 tokens
	if segments[1].TokenCount != 500 {
		t.Errorf("segment 1 tokens: expected 500, got %d", segments[1].TokenCount)
	}
}

func TestOptimize_NoBoundaries(t *testing.T) {
	blocks := makeBlocks(5, 100)
	br := boundary.BoundaryResult{
		Boundaries:  nil,
		DepthScores: make([]float64, 4),
	}

	config := segment.OptConfig{
		MinTokens: 200,
		MaxTokens: 1000,
	}

	segments := segment.Optimize(blocks, br, config)

	// No boundaries → single segment with all blocks
	if len(segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(segments))
	}
	if segments[0].TokenCount != 500 {
		t.Errorf("expected 500 tokens, got %d", segments[0].TokenCount)
	}
}

func TestOptimize_HeadingAdjustment(t *testing.T) {
	// Heading at index 5, boundary detected right after heading (index 5)
	// Should shift boundary to before heading (index 4)
	blocks := makeBlocksWithHeading(5, 10, 100)
	br := boundary.BoundaryResult{
		Boundaries:  []int{5},
		DepthScores: make([]float64, 9),
	}
	br.DepthScores[5] = 1.0

	config := segment.OptConfig{
		MinTokens: 200,
		MaxTokens: 800,
	}

	segments := segment.Optimize(blocks, br, config)

	if len(segments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(segments))
	}
	// First segment should end before heading (blocks 0..4 = 5 blocks)
	if len(segments[0].Blocks) != 5 {
		t.Errorf("first segment: expected 5 blocks, got %d", len(segments[0].Blocks))
	}
	// Second segment should start with heading (blocks 5..9 = 5 blocks)
	if len(segments[1].Blocks) != 5 {
		t.Errorf("second segment: expected 5 blocks, got %d", len(segments[1].Blocks))
	}
	if segments[1].Blocks[0].Kind != model.BlockHeading {
		t.Error("second segment should start with heading")
	}
}

func TestOptimize_UndersizedMerge(t *testing.T) {
	// 3 blocks with a boundary after index 0 → first segment has 1 block (100 tokens)
	// which is below min (200), so it should be merged
	blocks := makeBlocks(3, 100)
	br := boundary.BoundaryResult{
		Boundaries:  []int{0},
		DepthScores: make([]float64, 2),
	}
	br.DepthScores[0] = 1.0

	config := segment.OptConfig{
		MinTokens: 200,
		MaxTokens: 500,
	}

	segments := segment.Optimize(blocks, br, config)

	// Undersized first segment should be merged with next
	for _, s := range segments {
		if s.TokenCount < config.MinTokens {
			t.Errorf("segment has %d tokens, below min %d", s.TokenCount, config.MinTokens)
		}
	}
}

func TestOptimize_OversizedResplit(t *testing.T) {
	// 10 blocks each 200 tokens, no boundaries → single segment of 2000 tokens
	// Max is 800, so it should be re-split
	blocks := makeBlocks(10, 200)
	br := boundary.BoundaryResult{
		Boundaries:  nil,
		DepthScores: make([]float64, 9),
	}
	// Add some depth scores so re-split has candidates
	for i := range br.DepthScores {
		br.DepthScores[i] = float64(i) * 0.1
	}

	config := segment.OptConfig{
		MinTokens: 400,
		MaxTokens: 800,
	}

	segments := segment.Optimize(blocks, br, config)

	// Each segment should be within range
	for i, s := range segments {
		if s.TokenCount > config.MaxTokens {
			t.Errorf("segment %d has %d tokens, above max %d", i, s.TokenCount, config.MaxTokens)
		}
	}
	if len(segments) < 2 {
		t.Errorf("expected at least 2 segments after re-split, got %d", len(segments))
	}
}

func TestOptimize_EmptyBlocks(t *testing.T) {
	segments := segment.Optimize(nil, boundary.BoundaryResult{}, segment.OptConfig{})
	if len(segments) != 0 {
		t.Errorf("expected 0 segments for nil blocks, got %d", len(segments))
	}
}

func TestPackSegments_Basic(t *testing.T) {
	// 5 small segments (each 10 lines), maxLines=30 → should merge into 2 segments
	blocks := makeBlocksWithLines(5, 100, 10)
	br := boundary.BoundaryResult{
		Boundaries:  []int{0, 1, 2, 3},
		DepthScores: make([]float64, 4),
	}
	for i := range br.DepthScores {
		br.DepthScores[i] = 1.0
	}

	config := segment.OptConfig{
		MinTokens: 0,
		MaxTokens: 0, // disabled
		MaxLines:  30,
	}

	segments := segment.Optimize(blocks, br, config)

	// 5 blocks of 10 lines each → with maxLines=30, should pack into 2 segments (30+20)
	if len(segments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(segments))
	}
	if segments[0].LineCount != 30 {
		t.Errorf("segment 0 lines: expected 30, got %d", segments[0].LineCount)
	}
	if segments[1].LineCount != 20 {
		t.Errorf("segment 1 lines: expected 20, got %d", segments[1].LineCount)
	}
}

func TestPackSegments_MaxLinesRespected(t *testing.T) {
	// 4 segments of 20 lines each, maxLines=30 → should not merge any pair (20+20=40>30)
	blocks := makeBlocksWithLines(4, 100, 20)
	br := boundary.BoundaryResult{
		Boundaries:  []int{0, 1, 2},
		DepthScores: make([]float64, 3),
	}
	for i := range br.DepthScores {
		br.DepthScores[i] = 1.0
	}

	config := segment.OptConfig{
		MinTokens: 0,
		MaxTokens: 0,
		MaxLines:  30,
	}

	segments := segment.Optimize(blocks, br, config)

	// Each segment is 20 lines, merging any two would be 40 > 30, so all 4 remain
	if len(segments) != 4 {
		t.Fatalf("expected 4 segments, got %d", len(segments))
	}
	for i, s := range segments {
		if s.LineCount > 30 {
			t.Errorf("segment %d exceeds maxLines: %d > 30", i, s.LineCount)
		}
	}
}

func TestPackSegments_Disabled(t *testing.T) {
	// MaxLines=0 and MaxTokens=0 → pack should be no-op, segments unchanged
	blocks := makeBlocksWithLines(4, 100, 10)
	br := boundary.BoundaryResult{
		Boundaries:  []int{0, 1, 2},
		DepthScores: make([]float64, 3),
	}
	for i := range br.DepthScores {
		br.DepthScores[i] = 1.0
	}

	config := segment.OptConfig{
		MinTokens: 0,
		MaxTokens: 0,
		MaxLines:  0,
	}

	segments := segment.Optimize(blocks, br, config)

	if len(segments) != 4 {
		t.Fatalf("expected 4 segments (pack disabled), got %d", len(segments))
	}
}

func TestOptimize_OversizedResplit_MaxLines(t *testing.T) {
	// 1 segment of 10 blocks, each 50 lines, maxLines=200 → should resplit
	blocks := makeBlocksWithLines(10, 100, 50)
	br := boundary.BoundaryResult{
		Boundaries:  nil,
		DepthScores: make([]float64, 9),
	}
	for i := range br.DepthScores {
		br.DepthScores[i] = float64(i) * 0.1
	}

	config := segment.OptConfig{
		MinTokens: 0,
		MaxTokens: 0,
		MaxLines:  200,
	}

	segments := segment.Optimize(blocks, br, config)

	// 10 blocks × 50 lines = 500 lines total, maxLines=200 → need at least 3 segments
	if len(segments) < 3 {
		t.Fatalf("expected at least 3 segments after resplit, got %d", len(segments))
	}
	for i, s := range segments {
		if s.LineCount > 200 {
			t.Errorf("segment %d exceeds maxLines: %d > 200", i, s.LineCount)
		}
	}
}

func TestPackSegments_HeadingBarrier(t *testing.T) {
	// Helper: build segments from blocks manually
	makeSeg := func(blocks []model.Block) model.Segment {
		tokens := 0
		lines := 0
		for _, b := range blocks {
			tokens += b.TokensApprox
			lines += b.LinesApprox
		}
		return model.Segment{
			Blocks:     blocks,
			TokenCount: tokens,
			LineCount:  lines,
		}
	}

	tests := []struct {
		name     string
		segments []model.Segment
		barrier  int
		maxLines int
		wantN    int // expected segment count
	}{
		{
			name: "h2 blocks merge",
			segments: []model.Segment{
				makeSeg([]model.Block{{Kind: model.BlockParagraph, TokensApprox: 50, LinesApprox: 5}}),
				makeSeg([]model.Block{{Kind: model.BlockHeading, HeadingLevel: 2, TokensApprox: 10, LinesApprox: 1}}),
			},
			barrier:  2,
			maxLines: 100,
			wantN:    2, // barrier prevents merge
		},
		{
			name: "barrier=0 disabled",
			segments: []model.Segment{
				makeSeg([]model.Block{{Kind: model.BlockParagraph, TokensApprox: 50, LinesApprox: 5}}),
				makeSeg([]model.Block{{Kind: model.BlockHeading, HeadingLevel: 2, TokensApprox: 10, LinesApprox: 1}}),
			},
			barrier:  0,
			maxLines: 100,
			wantN:    1, // no barrier, merges
		},
		{
			name: "h3 passes barrier=2",
			segments: []model.Segment{
				makeSeg([]model.Block{{Kind: model.BlockParagraph, TokensApprox: 50, LinesApprox: 5}}),
				makeSeg([]model.Block{{Kind: model.BlockHeading, HeadingLevel: 3, TokensApprox: 10, LinesApprox: 1}}),
			},
			barrier:  2,
			maxLines: 100,
			wantN:    1, // h3 > barrier=2, so merges
		},
		{
			name: "h3 blocked by barrier=3",
			segments: []model.Segment{
				makeSeg([]model.Block{{Kind: model.BlockParagraph, TokensApprox: 50, LinesApprox: 5}}),
				makeSeg([]model.Block{{Kind: model.BlockHeading, HeadingLevel: 3, TokensApprox: 10, LinesApprox: 1}}),
			},
			barrier:  3,
			maxLines: 100,
			wantN:    2, // h3 <= barrier=3, blocked
		},
		{
			name: "pseudo-heading blocked",
			segments: []model.Segment{
				makeSeg([]model.Block{{Kind: model.BlockParagraph, TokensApprox: 50, LinesApprox: 5}}),
				makeSeg([]model.Block{{Kind: model.BlockParagraph, IsPseudoHeading: true, HeadingLevel: 7, TokensApprox: 10, LinesApprox: 1}}),
			},
			barrier:  2,
			maxLines: 100,
			wantN:    2, // pseudo-heading is a barrier
		},
		{
			name: "heading segment can merge with next",
			segments: []model.Segment{
				makeSeg([]model.Block{{Kind: model.BlockHeading, HeadingLevel: 2, TokensApprox: 10, LinesApprox: 1}}),
				makeSeg([]model.Block{{Kind: model.BlockParagraph, TokensApprox: 50, LinesApprox: 5}}),
			},
			barrier:  2,
			maxLines: 100,
			wantN:    1, // barrier checks segments[i], not current; paragraph segment has no barrier
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := segment.OptConfig{
				MaxLines:           tt.maxLines,
				PackHeadingBarrier: tt.barrier,
			}
			got := segment.PackSegments(tt.segments, cfg)
			if len(got) != tt.wantN {
				t.Errorf("got %d segments, want %d", len(got), tt.wantN)
			}
		})
	}
}

func TestMergeAt_NoSliceAliasing(t *testing.T) {
	// Verify that mergeAt does not modify the original segment's blocks
	blocks := makeBlocks(4, 100)
	br := boundary.BoundaryResult{
		Boundaries:  []int{0, 1, 2},
		DepthScores: make([]float64, 3),
	}
	for i := range br.DepthScores {
		br.DepthScores[i] = 1.0
	}

	// Create 4 segments with 1 block each, then merge to trigger mergeAt
	config := segment.OptConfig{
		MinTokens: 200, // forces merge of undersized 100-token segments
		MaxTokens: 1000,
	}
	segments := segment.Optimize(blocks, br, config)

	// Verify no aliasing: mutating one segment's blocks should not affect another
	if len(segments) >= 2 {
		origText := segments[1].Blocks[0].Text
		segments[0].Blocks[0].Text = "MUTATED"
		if segments[1].Blocks[0].Text != origText {
			t.Error("slice aliasing detected: mutating segment 0 affected segment 1")
		}
	}
}

func TestAdjustHeadingBoundaries(t *testing.T) {
	h := func(level int) model.Block {
		return model.Block{Kind: model.BlockHeading, HeadingLevel: level}
	}
	p := model.Block{Kind: model.BlockParagraph}

	tests := []struct {
		name       string
		blocks     []model.Block
		boundaries []int
		want       []int
	}{
		{
			name:       "single heading snap back",
			blocks:     []model.Block{p, h(2), p},
			boundaries: []int{1},
			want:       []int{0},
		},
		{
			name:       "already before heading",
			blocks:     []model.Block{p, h(2), p},
			boundaries: []int{0},
			want:       []int{0},
		},
		{
			name:       "consecutive headings snap to run start",
			blocks:     []model.Block{p, h(2), h(3), p},
			boundaries: []int{2},
			want:       []int{0},
		},
		{
			name:       "boundary within heading run",
			blocks:     []model.Block{p, h(2), h(3), h(4), p},
			boundaries: []int{1},
			want:       []int{0},
		},
		{
			name:       "document starts with heading run",
			blocks:     []model.Block{h(1), h(2), h(3), p, p},
			boundaries: []int{2},
			want:       []int{0},
		},
		{
			name:       "multiple boundaries with heading runs",
			blocks:     []model.Block{p, h(2), h(3), p, h(2), p},
			boundaries: []int{2, 4},
			want:       []int{0, 3},
		},
		{
			name:       "no headings no change",
			blocks:     []model.Block{p, p, p, p},
			boundaries: []int{1, 2},
			want:       []int{1, 2},
		},
		{
			name:       "boundary not touching heading",
			blocks:     []model.Block{p, p, h(2), p},
			boundaries: []int{0},
			want:       []int{0},
		},
		{
			name:       "dedup after snap",
			blocks:     []model.Block{p, h(2), h(3), p},
			boundaries: []int{1, 2},
			want:       []int{0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := segment.AdjustHeadingBoundaries(tt.blocks, tt.boundaries)
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("got %v, want %v", got, tt.want)
					break
				}
			}
		})
	}
}

func TestAutoMaxLines(t *testing.T) {
	tests := []struct {
		name       string
		totalLines int
		want       int
	}{
		{"small file 500 lines", 500, 200}, // 500/10=50 → clamp to 200
		{"1000 lines", 1000, 200},          // 1000/10=100 → clamp to 200
		{"3000 lines", 3000, 300},          // 3000/10=300
		{"5000 lines", 5000, 500},          // 5000/10=500
		{"10000 lines", 10000, 1000},       // 10000/10=1000
		{"20000 lines", 20000, 2000},       // 20000/10=2000
		{"50000 lines", 50000, 2000},       // 50000/10=5000 → clamp to 2000
		{"zero lines", 0, 200},             // 0/10=0 → clamp to 200
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := segment.AutoMaxLines(tt.totalLines)
			if got != tt.want {
				t.Errorf("AutoMaxLines(%d) = %d, want %d", tt.totalLines, got, tt.want)
			}
		})
	}
}
