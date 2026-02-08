package boundary

import (
	"slices"

	"github.com/thirdlf03/kire/internal/model"
	"github.com/thirdlf03/kire/internal/vecmath"
)

// BoundaryResult holds the detected boundaries and scoring details.
type BoundaryResult struct {
	Boundaries   []int     // Block indices where boundaries occur (gap between index and index+1)
	Similarities []float64 // Raw cosine similarities between adjacent blocks
	DepthScores  []float64 // Depth scores for each gap
	Threshold    float64   // Depth score cutoff used for boundary selection
	Confidences  []float64 // Confidence score (0.0-1.0) for each boundary
}

// MeanPool returns the element-wise mean of the given vectors.
// Returns nil for empty input. Returns a clone for single input.
func MeanPool(vectors [][]float64) []float64 {
	if len(vectors) == 0 {
		return nil
	}
	dims := len(vectors[0])
	result := make([]float64, dims)
	if len(vectors) == 1 {
		copy(result, vectors[0])
		return result
	}
	for _, v := range vectors {
		for j, val := range v {
			result[j] += val
		}
	}
	inv := 1.0 / float64(len(vectors))
	for j := range result {
		result[j] *= inv
	}
	return result
}

// BlockSimilarities computes cosine similarity between left and right block
// windows of size k for each gap. When k <= 1, falls back to adjacent
// comparison (equivalent to the original TextTiling implementation).
func BlockSimilarities(embeddings []model.Embedding, k int) []float64 {
	n := len(embeddings)
	if n < 2 {
		return nil
	}

	sims := make([]float64, n-1)

	if k <= 1 {
		for i := 0; i < n-1; i++ {
			sims[i] = CosineSimilarity(embeddings[i].Vector, embeddings[i+1].Vector)
		}
		return sims
	}

	for i := 0; i < n-1; i++ {
		lo := i - k + 1
		if lo < 0 {
			lo = 0
		}
		leftVecs := make([][]float64, 0, i-lo+1)
		for j := lo; j <= i; j++ {
			leftVecs = append(leftVecs, embeddings[j].Vector)
		}

		hi := i + k
		if hi > n-1 {
			hi = n - 1
		}
		rightVecs := make([][]float64, 0, hi-i)
		for j := i + 1; j <= hi; j++ {
			rightVecs = append(rightVecs, embeddings[j].Vector)
		}

		sims[i] = CosineSimilarity(MeanPool(leftVecs), MeanPool(rightVecs))
	}
	return sims
}

// CosineSimilarity computes the cosine similarity between two vectors.
func CosineSimilarity(a, b []float64) float64 {
	return vecmath.CosineSimilarity(a, b)
}

// Smooth applies a moving average to the similarity scores.
func Smooth(vals []float64, window int) []float64 {
	if window <= 0 || len(vals) == 0 {
		return slices.Clone(vals)
	}
	result := make([]float64, len(vals))
	for i := range vals {
		lo := i - window
		if lo < 0 {
			lo = 0
		}
		hi := i + window
		if hi >= len(vals) {
			hi = len(vals) - 1
		}
		sum := 0.0
		count := 0
		for j := lo; j <= hi; j++ {
			sum += vals[j]
			count++
		}
		result[i] = sum / float64(count)
	}
	return result
}

// DepthScore computes the depth score for each gap.
// For each position i, depth = (leftPeak - val[i]) + (rightPeak - val[i])
// where leftPeak is the max value to the left and rightPeak is the max to the right.
func DepthScore(sims []float64) []float64 {
	n := len(sims)
	depths := make([]float64, n)
	for i := 0; i < n; i++ {
		leftPeak := sims[i]
		for j := i - 1; j >= 0; j-- {
			if sims[j] > leftPeak {
				leftPeak = sims[j]
			}
			if sims[j] < sims[j+1] {
				break
			}
		}
		rightPeak := sims[i]
		for j := i + 1; j < n; j++ {
			if sims[j] > rightPeak {
				rightPeak = sims[j]
			}
			if sims[j] < sims[j-1] {
				break
			}
		}
		depths[i] = (leftPeak - sims[i]) + (rightPeak - sims[i])
	}
	return depths
}
