package vecmath

import "math"

// CosineSimilarity computes the cosine similarity between two vectors.
// Returns 0 for zero-length, mismatched, or zero-norm vectors.
func CosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	result := dot / (math.Sqrt(normA) * math.Sqrt(normB))
	if math.IsNaN(result) || math.IsInf(result, 0) {
		return 0
	}
	return result
}
