package eval

import (
	"math"
	"testing"
)

func almostEqual(a, b, eps float64) bool {
	return math.Abs(a-b) < eps
}

func TestPk_PerfectMatch(t *testing.T) {
	ref := []int{3, 7}
	hyp := []int{3, 7}
	got := Pk(ref, hyp, 10, 0)
	if got != 0 {
		t.Errorf("Pk perfect match: want 0, got %f", got)
	}
}

func TestPk_NoBoundaries(t *testing.T) {
	// Both have no boundaries: perfect agreement
	got := Pk(nil, nil, 10, 3)
	if got != 0 {
		t.Errorf("Pk no boundaries: want 0, got %f", got)
	}
}

func TestPk_AllVsNone(t *testing.T) {
	// ref has boundaries at every gap, hyp has none
	ref := []int{0, 1, 2, 3, 4, 5, 6, 7, 8}
	got := Pk(ref, nil, 10, 2)
	// Every window of size 2: ref always says "different segment", hyp always says "same"
	if got <= 0 {
		t.Errorf("Pk all vs none: expected positive penalty, got %f", got)
	}
	if got > 1 {
		t.Errorf("Pk all vs none: expected <= 1, got %f", got)
	}
}

func TestPk_HandCalculated(t *testing.T) {
	// 6 blocks, ref boundaries at [2], hyp at [3]
	// k = auto = 6/2 / 2 = 1.5 -> 1
	// With k=1, compare pairs (0,1),(1,2),(2,3),(3,4),(4,5)
	// ref segments: [0,0,0,1,1,1]
	// hyp segments: [0,0,0,0,1,1]
	// (0,1): ref same, hyp same -> match
	// (1,2): ref same, hyp same -> match
	// (2,3): ref diff, hyp same -> mismatch
	// (3,4): ref same, hyp diff -> mismatch
	// (4,5): ref same, hyp same -> match
	// Pk = 2/5 = 0.4
	ref := []int{2}
	hyp := []int{3}
	got := Pk(ref, hyp, 6, 1)
	if !almostEqual(got, 0.4, 1e-9) {
		t.Errorf("Pk hand calculated: want 0.4, got %f", got)
	}
}

func TestPk_SingleBlock(t *testing.T) {
	got := Pk(nil, nil, 1, 0)
	if got != 0 {
		t.Errorf("Pk single block: want 0, got %f", got)
	}
}

func TestWindowDiff_PerfectMatch(t *testing.T) {
	ref := []int{3, 7}
	hyp := []int{3, 7}
	got := WindowDiff(ref, hyp, 10, 0)
	if got != 0 {
		t.Errorf("WindowDiff perfect match: want 0, got %f", got)
	}
}

func TestWindowDiff_NoBoundaries(t *testing.T) {
	got := WindowDiff(nil, nil, 10, 3)
	if got != 0 {
		t.Errorf("WindowDiff no boundaries: want 0, got %f", got)
	}
}

func TestWindowDiff_HandCalculated(t *testing.T) {
	// 6 blocks, ref at [2], hyp at [3], k=2
	// Windows [0,2],[1,3],[2,4],[3,5]
	// ref boundaries in window: count of b where lo <= b < hi
	// [0,2): ref has 2? no (2 is not < 2). count=0. hyp: count=0. same -> ok
	// Wait, boundary at index 2 means gap between block 2 and 3.
	// [0,2): boundaries 0,1 -> ref: none, hyp: none -> ok
	// [1,3): boundaries 1,2 -> ref: 2 yes count=1, hyp: none count=0 -> penalty
	// [2,4): boundaries 2,3 -> ref: 2 count=1, hyp: 3 count=1 -> ok
	// [3,5): boundaries 3,4 -> ref: none count=0, hyp: 3 count=1 -> penalty
	// WD = 2/4 = 0.5
	ref := []int{2}
	hyp := []int{3}
	got := WindowDiff(ref, hyp, 6, 2)
	if !almostEqual(got, 0.5, 1e-9) {
		t.Errorf("WindowDiff hand calculated: want 0.5, got %f", got)
	}
}

func TestWindowDiff_SingleBlock(t *testing.T) {
	got := WindowDiff(nil, nil, 1, 0)
	if got != 0 {
		t.Errorf("WindowDiff single block: want 0, got %f", got)
	}
}

func TestWindowDiff_ExtraBoundary(t *testing.T) {
	// ref has 1 boundary, hyp has 2
	// WindowDiff should detect the false positive
	ref := []int{4}
	hyp := []int{2, 4}
	got := WindowDiff(ref, hyp, 10, 0)
	if got <= 0 {
		t.Errorf("WindowDiff extra boundary: expected positive penalty, got %f", got)
	}
}

func TestBoundaryPRF_PerfectMatch(t *testing.T) {
	ref := []int{2, 5, 8}
	hyp := []int{2, 5, 8}
	got := BoundaryPRF(ref, hyp, 10, 0)
	if got.Precision != 1 || got.Recall != 1 || got.F1 != 1 {
		t.Errorf("BoundaryPRF perfect: want P=R=F1=1, got %+v", got)
	}
}

func TestBoundaryPRF_NoMatch(t *testing.T) {
	ref := []int{2, 5}
	hyp := []int{7, 9}
	got := BoundaryPRF(ref, hyp, 10, 0)
	if got.Precision != 0 || got.Recall != 0 || got.F1 != 0 {
		t.Errorf("BoundaryPRF no match: want P=R=F1=0, got %+v", got)
	}
}

func TestBoundaryPRF_WithTolerance(t *testing.T) {
	ref := []int{2, 5}
	hyp := []int{3, 6}
	// tolerance=1: both hyp match within distance 1
	got := BoundaryPRF(ref, hyp, 10, 1)
	if got.Precision != 1 || got.Recall != 1 || got.F1 != 1 {
		t.Errorf("BoundaryPRF tolerance=1: want P=R=F1=1, got %+v", got)
	}
}

func TestBoundaryPRF_PartialMatch(t *testing.T) {
	ref := []int{2, 5, 8}
	hyp := []int{2, 7}
	// hyp[0]=2 matches ref[0]=2, hyp[1]=7 no exact match
	got := BoundaryPRF(ref, hyp, 10, 0)
	// P = 1/2, R = 1/3
	wantP := 0.5
	wantR := 1.0 / 3.0
	wantF1 := 2 * wantP * wantR / (wantP + wantR)
	if !almostEqual(got.Precision, wantP, 1e-9) {
		t.Errorf("BoundaryPRF partial P: want %f, got %f", wantP, got.Precision)
	}
	if !almostEqual(got.Recall, wantR, 1e-9) {
		t.Errorf("BoundaryPRF partial R: want %f, got %f", wantR, got.Recall)
	}
	if !almostEqual(got.F1, wantF1, 1e-9) {
		t.Errorf("BoundaryPRF partial F1: want %f, got %f", wantF1, got.F1)
	}
}

func TestBoundaryPRF_BothEmpty(t *testing.T) {
	got := BoundaryPRF(nil, nil, 10, 0)
	if got.Precision != 1 || got.Recall != 1 || got.F1 != 1 {
		t.Errorf("BoundaryPRF both empty: want P=R=F1=1, got %+v", got)
	}
}

func TestBoundaryPRF_RefEmptyHypNot(t *testing.T) {
	got := BoundaryPRF(nil, []int{3}, 10, 0)
	if got.Precision != 0 || got.F1 != 0 {
		t.Errorf("BoundaryPRF ref empty: want P=0 F1=0, got %+v", got)
	}
}

func TestBoundaryPRF_HypEmptyRefNot(t *testing.T) {
	got := BoundaryPRF([]int{3}, nil, 10, 0)
	if got.Recall != 0 || got.F1 != 0 {
		t.Errorf("BoundaryPRF hyp empty: want R=0 F1=0, got %+v", got)
	}
}

func TestAutoK(t *testing.T) {
	// 10 blocks, 1 boundary -> 2 segments, avg len = 5, k = 2
	k := autoK([]int{4}, 10)
	if k != 2 {
		t.Errorf("autoK: want 2, got %d", k)
	}
}

func TestAutoK_NoBoundaries(t *testing.T) {
	// 10 blocks, 0 boundaries -> 1 segment, avg len = 10, k = 5
	k := autoK(nil, 10)
	if k != 5 {
		t.Errorf("autoK no boundaries: want 5, got %d", k)
	}
}

func TestBuildSegmentMap(t *testing.T) {
	// 6 blocks, boundaries at [1, 3]
	// Segments: [0,0, 1,1, 2,2]
	// Block 0: seg 0 (not past boundary 1)
	// Block 1: seg 0 (not past boundary 1, since 1 is not > 1)
	// Block 2: seg 1 (2 > 1)
	// Block 3: seg 1 (3 is not > 3)
	// Block 4: seg 2 (4 > 3)
	// Block 5: seg 2
	got := buildSegmentMap([]int{1, 3}, 6)
	want := []int{0, 0, 1, 1, 2, 2}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("buildSegmentMap[%d]: want %d, got %d", i, w, got[i])
		}
	}
}
