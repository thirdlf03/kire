package embedding

import (
	"context"
	"fmt"
	"math"

	"github.com/thirdlf03/kire/internal/model"
)

func init() {
	Register("mock", func(_ context.Context, _ ProviderConfig) (Embedder, string, error) {
		const dims = 128
		return NewMockEmbedder(dims), fmt.Sprintf("mock(dim=%d)", dims), nil
	})
}

// MockEmbedder generates deterministic embeddings based on character frequency.
// Used for testing boundary detection without an API dependency.
type MockEmbedder struct {
	Dimensions int
}

func NewMockEmbedder(dimensions int) *MockEmbedder {
	if dimensions <= 0 {
		dimensions = 128
	}
	return &MockEmbedder{Dimensions: dimensions}
}

func (m *MockEmbedder) Embed(_ context.Context, blocks []model.Block, _ EmbedOptions) ([]model.Embedding, error) {
	result := make([]model.Embedding, len(blocks))
	for i, b := range blocks {
		result[i] = model.Embedding{
			BlockIndex: i,
			Vector:     charFreqVector(b.Text, m.Dimensions),
		}
	}
	return result, nil
}

// charFreqVector builds a deterministic vector from character frequencies.
// Each dimension corresponds to a character bucket (char % dimensions).
func charFreqVector(text string, dims int) []float64 {
	vec := make([]float64, dims)
	if len(text) == 0 {
		return vec
	}
	for _, r := range text {
		idx := int(r) % dims
		vec[idx]++
	}
	// Normalize to unit vector
	var norm float64
	for _, v := range vec {
		norm += v * v
	}
	if norm > 0 {
		norm = math.Sqrt(norm)
		for i := range vec {
			vec[i] /= norm
		}
	}
	return vec
}
