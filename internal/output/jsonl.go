package output

import (
	"encoding/json"
	"io"

	"github.com/thirdlf03/kire/internal/model"
)

// JSONLRecord represents a single JSONL line for one segment.
type JSONLRecord struct {
	Content   string        `json:"content"`
	Metadata  JSONLMetadata `json:"metadata"`
	Embedding []float64     `json:"embedding,omitempty"`
}

// JSONLMetadata holds per-segment metadata.
type JSONLMetadata struct {
	Source       string   `json:"source"`
	SegmentIndex int      `json:"segment_index"`
	Filename     string   `json:"filename"`
	ByteRange    [2]int   `json:"byte_range"`
	HeadingPath  []string `json:"heading_path"`
	TokenCount   int      `json:"token_count"`
	BlockCount   int      `json:"block_count"`

	// Agent metadata (populated when --agent-metadata is set)
	SegmentID          string   `json:"segment_id,omitempty"`
	PrevSegmentID      string   `json:"prev_segment_id,omitempty"`
	NextSegmentID      string   `json:"next_segment_id,omitempty"`
	Coherence          *float64 `json:"coherence,omitempty"`
	BoundaryConfidence *float64 `json:"boundary_confidence,omitempty"`
}

// WriteJSONL writes records as one JSON object per line to w.
func WriteJSONL(w io.Writer, records []JSONLRecord) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	for _, r := range records {
		if err := enc.Encode(r); err != nil {
			return err
		}
	}
	return nil
}

// AverageEmbedding computes the element-wise average of embeddings at the given global block indices.
// Returns nil if indices is empty.
func AverageEmbedding(globalIndices []int, embeddings []model.Embedding) []float64 {
	if len(globalIndices) == 0 {
		return nil
	}

	// Build index map for fast lookup
	byIdx := make(map[int][]float64, len(embeddings))
	for _, e := range embeddings {
		byIdx[e.BlockIndex] = e.Vector
	}

	var dim int
	var vectors [][]float64
	for _, idx := range globalIndices {
		v, ok := byIdx[idx]
		if !ok || len(v) == 0 {
			continue
		}
		if dim == 0 {
			dim = len(v)
		}
		vectors = append(vectors, v)
	}
	if len(vectors) == 0 {
		return nil
	}

	avg := make([]float64, dim)
	for _, v := range vectors {
		for i := range avg {
			if i < len(v) {
				avg[i] += v[i]
			}
		}
	}
	n := float64(len(vectors))
	for i := range avg {
		avg[i] /= n
	}
	return avg
}

// SegmentBlockIndices returns the global block indices for each segment.
// It matches segment blocks to allBlocks by SourceRange.Start.
func SegmentBlockIndices(segments []model.Segment, allBlocks []model.Block) [][]int {
	startToIdx := model.BlockStartIndex(allBlocks)

	result := make([][]int, len(segments))
	for i, seg := range segments {
		indices := make([]int, 0, len(seg.Blocks))
		for _, b := range seg.Blocks {
			if idx, ok := startToIdx[b.Range.Start]; ok {
				indices = append(indices, idx)
			}
		}
		result[i] = indices
	}
	return result
}
