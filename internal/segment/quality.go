package segment

import (
	"math"

	"github.com/thirdlf03/kire/internal/boundary"
	"github.com/thirdlf03/kire/internal/model"
	"github.com/thirdlf03/kire/internal/vecmath"
)

// ComputeCoherence computes the average pairwise cosine similarity
// among blocks at the given indices. Returns 1.0 for single/empty sets.
func ComputeCoherence(indices []int, embeddings []model.Embedding) float64 {
	if len(indices) <= 1 {
		if len(indices) == 1 {
			return 1.0
		}
		return 0
	}

	byIdx := make(map[int][]float64, len(embeddings))
	for _, e := range embeddings {
		byIdx[e.BlockIndex] = e.Vector
	}

	var sum float64
	var count int
	for i := 0; i < len(indices); i++ {
		for j := i + 1; j < len(indices); j++ {
			va, oka := byIdx[indices[i]]
			vb, okb := byIdx[indices[j]]
			if oka && okb {
				sum += vecmath.CosineSimilarity(va, vb)
				count++
			}
		}
	}

	if count == 0 {
		return 0
	}
	return sum / float64(count)
}

// ComputeBoundaryConfidences returns a confidence score (0.0-1.0) for each boundary.
// Confidence is based on how much the depth score exceeds the threshold,
// normalized by the maximum depth score.
func ComputeBoundaryConfidences(br boundary.BoundaryResult) []float64 {
	if len(br.Boundaries) == 0 {
		return nil
	}

	maxDepth := 0.0
	for _, d := range br.DepthScores {
		if d > maxDepth {
			maxDepth = d
		}
	}

	confidences := make([]float64, len(br.Boundaries))
	for i, b := range br.Boundaries {
		if b < 0 || b >= len(br.DepthScores) || maxDepth <= 0 {
			confidences[i] = 0
			continue
		}
		raw := (br.DepthScores[b] - br.Threshold) / maxDepth
		confidences[i] = clamp01(raw)
	}
	return confidences
}

// QualityMetrics holds overall quality indicators for the splitting result.
type QualityMetrics struct {
	MeanCoherence     float64
	MeanConfidence    float64
	MinCoherence      float64
	SegmentCountScore float64
}

// ComputeQualityMetrics computes aggregate quality metrics for the split result.
func ComputeQualityMetrics(segments []model.Segment, allBlocks []model.Block, embeddings []model.Embedding, br boundary.BoundaryResult) QualityMetrics {
	if len(segments) == 0 {
		return QualityMetrics{}
	}

	// Build block index lookup
	startToIdx := model.BlockStartIndex(allBlocks)

	// Compute per-segment coherence and assign to each segment
	coherences := make([]float64, len(segments))
	for i := range segments {
		indices := make([]int, 0, len(segments[i].Blocks))
		for _, b := range segments[i].Blocks {
			if idx, ok := startToIdx[b.Range.Start]; ok {
				indices = append(indices, idx)
			}
		}
		coherences[i] = ComputeCoherence(indices, embeddings)
		segments[i].Coherence = coherences[i]
	}

	meanCoh := 0.0
	minCoh := math.Inf(1)
	for _, c := range coherences {
		meanCoh += c
		if c < minCoh {
			minCoh = c
		}
	}
	meanCoh /= float64(len(coherences))

	// Boundary confidences
	confidences := ComputeBoundaryConfidences(br)
	meanConf := 0.0
	if len(confidences) > 0 {
		for _, c := range confidences {
			meanConf += c
		}
		meanConf /= float64(len(confidences))
	}

	// Segment count score: prefer 5-15 segments, penalize extremes
	n := float64(len(segments))
	segScore := 1.0 - math.Abs(n-10)/10
	segScore = clamp01(segScore)

	return QualityMetrics{
		MeanCoherence:     meanCoh,
		MeanConfidence:    meanConf,
		MinCoherence:      minCoh,
		SegmentCountScore: segScore,
	}
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
