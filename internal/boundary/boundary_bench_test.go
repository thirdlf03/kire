package boundary

import (
	"testing"

	"github.com/thirdlf03/kire/internal/model"
)

func makeBenchEmbeddings(n, dims int) []model.Embedding {
	embeddings := make([]model.Embedding, n)
	for i := range embeddings {
		vec := make([]float64, dims)
		for j := range vec {
			// Deterministic pseudo-random pattern
			vec[j] = float64((i*7+j*13)%100) / 100.0
		}
		// Normalize
		norm := 0.0
		for _, v := range vec {
			norm += v * v
		}
		if norm > 0 {
			norm = 1.0 / sqrt(norm)
			for j := range vec {
				vec[j] *= norm
			}
		}
		embeddings[i] = model.Embedding{
			BlockIndex: i,
			Vector:     vec,
		}
	}
	return embeddings
}

func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	z := x
	for i := 0; i < 20; i++ {
		z = (z + x/z) / 2
	}
	return z
}

func BenchmarkDetectBoundaries_20blocks(b *testing.B) {
	embeddings := makeBenchEmbeddings(20, 128)
	config := ScoringConfig{Window: 3, MinGap: 3}

	b.ResetTimer()
	for b.Loop() {
		DetectBoundaries(embeddings, config)
	}
}

func BenchmarkDetectBoundaries_100blocks(b *testing.B) {
	embeddings := makeBenchEmbeddings(100, 256)
	config := ScoringConfig{Window: 3, MinGap: 3}

	b.ResetTimer()
	for b.Loop() {
		DetectBoundaries(embeddings, config)
	}
}

func BenchmarkDetectBoundaries_500blocks(b *testing.B) {
	embeddings := makeBenchEmbeddings(500, 256)
	config := ScoringConfig{Window: 3, MinGap: 3}

	b.ResetTimer()
	for b.Loop() {
		DetectBoundaries(embeddings, config)
	}
}

func BenchmarkDetectBoundaries_100blocks_BlockK3(b *testing.B) {
	embeddings := makeBenchEmbeddings(100, 256)
	config := ScoringConfig{Window: 3, MinGap: 3, BlockK: 3}

	b.ResetTimer()
	for b.Loop() {
		DetectBoundaries(embeddings, config)
	}
}
