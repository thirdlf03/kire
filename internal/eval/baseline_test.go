package eval

import (
	"reflect"
	"testing"

	"github.com/thirdlf03/kire/internal/model"
)

func TestHeadingSplit(t *testing.T) {
	blocks := []model.Block{
		{Kind: model.BlockHeading, HeadingLevel: 1},  // 0: H1
		{Kind: model.BlockParagraph},                  // 1: Para
		{Kind: model.BlockHeading, HeadingLevel: 2},   // 2: H2
		{Kind: model.BlockParagraph},                  // 3: Para
		{Kind: model.BlockList},                       // 4: List
		{Kind: model.BlockHeading, HeadingLevel: 3},   // 5: H3
		{Kind: model.BlockParagraph},                  // 6: Para
		{Kind: model.BlockCodeBlock},                  // 7: Code
		{Kind: model.BlockParagraph},                  // 8: Para
	}

	got := HeadingSplit(blocks)
	// Boundaries before H2 (gap 1), H3 (gap 4)
	// H1 at index 0 is skipped (i=0 check)
	want := []int{1, 4}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("HeadingSplit: want %v, got %v", want, got)
	}
}

func TestHeadingSplit_NoHeadings(t *testing.T) {
	blocks := []model.Block{
		{Kind: model.BlockParagraph},
		{Kind: model.BlockParagraph},
	}
	got := HeadingSplit(blocks)
	if got != nil {
		t.Errorf("HeadingSplit no headings: want nil, got %v", got)
	}
}

func TestHeadingSplit_OnlyH1(t *testing.T) {
	blocks := []model.Block{
		{Kind: model.BlockHeading, HeadingLevel: 1},
		{Kind: model.BlockParagraph},
	}
	got := HeadingSplit(blocks)
	if got != nil {
		t.Errorf("HeadingSplit only H1 at start: want nil, got %v", got)
	}
}

func TestFixedSplit(t *testing.T) {
	tests := []struct {
		name      string
		numBlocks int
		n         int
		want      []int
	}{
		{"10 blocks / 3", 10, 3, []int{2, 5, 8}},
		{"6 blocks / 3", 6, 3, []int{2}},
		{"3 blocks / 3", 3, 3, nil},
		{"5 blocks / 2", 5, 2, []int{1, 3}},
		{"n=0", 10, 0, nil},
		{"n negative", 10, -1, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FixedSplit(tt.numBlocks, tt.n)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FixedSplit(%d, %d): want %v, got %v", tt.numBlocks, tt.n, tt.want, got)
			}
		})
	}
}

func TestRandomSplit_Reproducible(t *testing.T) {
	a := RandomSplit(20, 5, 42)
	b := RandomSplit(20, 5, 42)
	if !reflect.DeepEqual(a, b) {
		t.Errorf("RandomSplit not reproducible: %v vs %v", a, b)
	}
}

func TestRandomSplit_Sorted(t *testing.T) {
	got := RandomSplit(20, 5, 123)
	for i := 1; i < len(got); i++ {
		if got[i] <= got[i-1] {
			t.Errorf("RandomSplit not sorted: %v", got)
			break
		}
	}
}

func TestRandomSplit_CorrectCount(t *testing.T) {
	got := RandomSplit(20, 5, 99)
	if len(got) != 5 {
		t.Errorf("RandomSplit count: want 5, got %d", len(got))
	}
}

func TestRandomSplit_ClampToMaxGaps(t *testing.T) {
	// 5 blocks -> 4 possible gaps, request 10
	got := RandomSplit(5, 10, 1)
	if len(got) != 4 {
		t.Errorf("RandomSplit clamp: want 4, got %d (%v)", len(got), got)
	}
}

func TestRandomSplit_EdgeCases(t *testing.T) {
	if got := RandomSplit(1, 1, 0); got != nil {
		t.Errorf("RandomSplit 1 block: want nil, got %v", got)
	}
	if got := RandomSplit(10, 0, 0); got != nil {
		t.Errorf("RandomSplit 0 boundaries: want nil, got %v", got)
	}
}

func TestRandomSplit_DifferentSeeds(t *testing.T) {
	a := RandomSplit(50, 10, 1)
	b := RandomSplit(50, 10, 2)
	if reflect.DeepEqual(a, b) {
		t.Errorf("RandomSplit different seeds should (very likely) produce different results")
	}
}
