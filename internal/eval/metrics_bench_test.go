package eval

import "testing"

func BenchmarkPk(b *testing.B) {
	ref := []int{5, 15, 25, 35, 45, 55, 65, 75, 85, 95}
	hyp := []int{4, 14, 26, 34, 46, 54, 66, 74, 86, 94}
	numBlocks := 100

	b.ResetTimer()
	for b.Loop() {
		Pk(ref, hyp, numBlocks, 0)
	}
}

func BenchmarkWindowDiff(b *testing.B) {
	ref := []int{5, 15, 25, 35, 45, 55, 65, 75, 85, 95}
	hyp := []int{4, 14, 26, 34, 46, 54, 66, 74, 86, 94}
	numBlocks := 100

	b.ResetTimer()
	for b.Loop() {
		WindowDiff(ref, hyp, numBlocks, 0)
	}
}

func BenchmarkBoundaryPRF(b *testing.B) {
	ref := []int{5, 15, 25, 35, 45, 55, 65, 75, 85, 95}
	hyp := []int{4, 14, 26, 34, 46, 54, 66, 74, 86, 94}

	b.ResetTimer()
	for b.Loop() {
		BoundaryPRF(ref, hyp, 100, 1)
	}
}

func BenchmarkPk_LargeDoc(b *testing.B) {
	// 1000 blocks, 50 boundaries
	ref := make([]int, 50)
	hyp := make([]int, 50)
	for i := range ref {
		ref[i] = (i + 1) * 19
		hyp[i] = (i+1)*19 + 1
	}

	b.ResetTimer()
	for b.Loop() {
		Pk(ref, hyp, 1000, 0)
	}
}
