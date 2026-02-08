package llmsplit

import (
	"strings"
	"testing"

	"github.com/thirdlf03/kire/internal/model"
)

func TestValidateBoundaries(t *testing.T) {
	tests := []struct {
		name      string
		raw       []int
		numBlocks int
		want      []int
	}{
		{
			name:      "valid sorted",
			raw:       []int{2, 5, 8},
			numBlocks: 10,
			want:      []int{2, 5, 8},
		},
		{
			name:      "unsorted",
			raw:       []int{8, 2, 5},
			numBlocks: 10,
			want:      []int{2, 5, 8},
		},
		{
			name:      "duplicates",
			raw:       []int{3, 3, 5, 5},
			numBlocks: 10,
			want:      []int{3, 5},
		},
		{
			name:      "out of range negative",
			raw:       []int{-1, 2, 5},
			numBlocks: 10,
			want:      []int{2, 5},
		},
		{
			name:      "out of range high",
			raw:       []int{2, 5, 9, 10},
			numBlocks: 10,
			want:      []int{2, 5},
		},
		{
			name:      "max valid index",
			raw:       []int{0, 8},
			numBlocks: 10,
			want:      []int{0, 8},
		},
		{
			name:      "empty input",
			raw:       []int{},
			numBlocks: 10,
			want:      nil,
		},
		{
			name:      "single block",
			raw:       []int{0},
			numBlocks: 1,
			want:      nil,
		},
		{
			name:      "two blocks valid",
			raw:       []int{0},
			numBlocks: 2,
			want:      []int{0},
		},
		{
			name:      "all invalid",
			raw:       []int{-5, 100, 200},
			numBlocks: 5,
			want:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateBoundaries(tt.raw, tt.numBlocks)
			if len(got) != len(tt.want) {
				t.Fatalf("len: got %d, want %d (got=%v, want=%v)", len(got), len(tt.want), got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("index %d: got %d, want %d", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestBuildPrompt(t *testing.T) {
	blocks := []model.Block{
		{Kind: model.BlockHeading, HeadingLevel: 1, Text: "# Introduction"},
		{Kind: model.BlockParagraph, Text: "This is the first paragraph."},
		{Kind: model.BlockParagraph, Text: "This is the second paragraph."},
		{Kind: model.BlockCodeBlock, Text: "func main() {}"},
	}

	prompt := buildPrompt(blocks, nil)

	if !strings.Contains(prompt, "4 blocks") {
		t.Error("prompt should mention block count")
	}
	if !strings.Contains(prompt, "gap indices 0 to 2") {
		t.Error("prompt should mention gap index range")
	}
	if !strings.Contains(prompt, "[Block 0] (heading, level 1)") {
		t.Error("prompt should format heading blocks with level")
	}
	if !strings.Contains(prompt, "[Block 3] (codeblock)") {
		t.Error("prompt should format code blocks")
	}
	if !strings.Contains(prompt, "# Introduction") {
		t.Error("prompt should include block text")
	}
	// No similarities → no similarity lines
	if strings.Contains(prompt, "similarity:") {
		t.Error("prompt should not contain similarity when none provided")
	}
}

func TestBuildPrompt_WithSimilarities(t *testing.T) {
	blocks := []model.Block{
		{Kind: model.BlockParagraph, Text: "Block A"},
		{Kind: model.BlockParagraph, Text: "Block B"},
		{Kind: model.BlockParagraph, Text: "Block C"},
	}
	sims := []float64{0.85, 0.32}

	prompt := buildPrompt(blocks, sims)

	if !strings.Contains(prompt, "similarity: 0.8500") {
		t.Errorf("prompt should contain similarity for gap 0, got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "similarity: 0.3200") {
		t.Errorf("prompt should contain similarity for gap 1, got:\n%s", prompt)
	}
}

func TestVerifyLossless(t *testing.T) {
	tests := []struct {
		name      string
		segments  []model.Segment
		sourceLen int
		wantErr   bool
	}{
		{
			name:      "empty source empty segments",
			segments:  nil,
			sourceLen: 0,
			wantErr:   false,
		},
		{
			name:      "empty segments non-empty source",
			segments:  nil,
			sourceLen: 100,
			wantErr:   true,
		},
		{
			name: "single segment covering all",
			segments: []model.Segment{
				{Range: model.SourceRange{Start: 0, End: 100}},
			},
			sourceLen: 100,
			wantErr:   false,
		},
		{
			name: "two segments tiling",
			segments: []model.Segment{
				{Range: model.SourceRange{Start: 0, End: 50}},
				{Range: model.SourceRange{Start: 50, End: 100}},
			},
			sourceLen: 100,
			wantErr:   false,
		},
		{
			name: "gap between segments",
			segments: []model.Segment{
				{Range: model.SourceRange{Start: 0, End: 40}},
				{Range: model.SourceRange{Start: 50, End: 100}},
			},
			sourceLen: 100,
			wantErr:   true,
		},
		{
			name: "first segment not starting at 0",
			segments: []model.Segment{
				{Range: model.SourceRange{Start: 10, End: 100}},
			},
			sourceLen: 100,
			wantErr:   true,
		},
		{
			name: "last segment not ending at sourceLen",
			segments: []model.Segment{
				{Range: model.SourceRange{Start: 0, End: 90}},
			},
			sourceLen: 100,
			wantErr:   true,
		},
		{
			name: "three segments tiling",
			segments: []model.Segment{
				{Range: model.SourceRange{Start: 0, End: 30}},
				{Range: model.SourceRange{Start: 30, End: 70}},
				{Range: model.SourceRange{Start: 70, End: 100}},
			},
			sourceLen: 100,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := VerifyLossless(tt.segments, tt.sourceLen)
			if (err != nil) != tt.wantErr {
				t.Errorf("VerifyLossless() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
