package boundary

import (
	"math"

	"github.com/thirdlf03/kire/internal/model"
	"github.com/thirdlf03/kire/internal/vecmath"
)

// GramMatrix holds the precomputed cosine kernel matrix and prefix sums
// for O(1) segment cost queries.
type GramMatrix struct {
	n   int
	K   [][]float64 // K[i][j] = cosine(embedding_i, embedding_j)
	pre [][]float64 // 2D prefix sum: pre[i][j] = sum of K[a][b] for a<i, b<j
}

// NewGramMatrix builds a cosine kernel Gram matrix from embeddings.
// Time: O(n^2 * d), Space: O(n^2).
func NewGramMatrix(embeddings []model.Embedding) GramMatrix {
	n := len(embeddings)
	if n == 0 {
		return GramMatrix{}
	}

	K := make([][]float64, n)
	for i := 0; i < n; i++ {
		K[i] = make([]float64, n)
		K[i][i] = vecmath.CosineSimilarity(embeddings[i].Vector, embeddings[i].Vector)
		for j := 0; j < i; j++ {
			sim := vecmath.CosineSimilarity(embeddings[i].Vector, embeddings[j].Vector)
			K[i][j] = sim
			K[j][i] = sim
		}
	}

	// Build 2D prefix sum: pre[i+1][j+1] = sum of K[a][b] for 0<=a<=i, 0<=b<=j
	pre := make([][]float64, n+1)
	for i := 0; i <= n; i++ {
		pre[i] = make([]float64, n+1)
	}
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			pre[i+1][j+1] = K[i][j] + pre[i][j+1] + pre[i+1][j] - pre[i][j]
		}
	}

	return GramMatrix{n: n, K: K, pre: pre}
}

// rectSum returns the sum of K[i][j] for a<=i<b, a<=j<b using 2D prefix sums.
func (g *GramMatrix) rectSum(a, b int) float64 {
	return g.pre[b][b] - g.pre[a][b] - g.pre[b][a] + g.pre[a][a]
}

// SegmentCost computes the kernel change-point detection cost for segment [a, b).
// Cost(a,b) = sum_{i in [a,b)} K[i][i] - (1/(b-a)) * sum_{i,j in [a,b)} K[i][j]
// A homogeneous segment (all same topic) has cost near 0.
// A mixed segment has higher cost.
func (g *GramMatrix) SegmentCost(a, b int) float64 {
	length := b - a
	if length <= 0 {
		return 0
	}
	if length == 1 {
		return 0 // single element: K[a][a] - (1/1)*K[a][a] = 0
	}

	diagSum := 0.0
	for i := a; i < b; i++ {
		diagSum += g.K[i][i]
	}

	crossSum := g.rectSum(a, b)
	return diagSum - crossSum/float64(length)
}

// CVSScore computes the Content Vector Segmentation score: the sum of L2 norms
// of segment mean vectors. Higher CVS indicates better-aligned segments.
// boundaries are gap indices (0-indexed); the last segment extends to len(embeddings).
func CVSScore(embeddings []model.Embedding, boundaries []int) float64 {
	n := len(embeddings)
	if n == 0 {
		return 0
	}

	totalScore := 0.0
	start := 0
	for _, b := range boundaries {
		end := b + 1
		if end > n {
			end = n
		}
		totalScore += segmentNorm(embeddings, start, end)
		start = end
	}
	if start < n {
		totalScore += segmentNorm(embeddings, start, n)
	}
	return totalScore
}

// segmentNorm returns the L2 norm of the mean vector for embeddings[a:b).
func segmentNorm(embeddings []model.Embedding, a, b int) float64 {
	length := b - a
	if length <= 0 {
		return 0
	}
	dims := len(embeddings[a].Vector)
	if dims == 0 {
		return 0
	}

	mean := make([]float64, dims)
	for i := a; i < b; i++ {
		for j, v := range embeddings[i].Vector {
			mean[j] += v
		}
	}
	inv := 1.0 / float64(length)
	norm := 0.0
	for j := range mean {
		mean[j] *= inv
		norm += mean[j] * mean[j]
	}
	return math.Sqrt(norm)
}
