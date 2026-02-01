package embedding

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/thirdlf03/kire/internal/model"
)

func init() {
	Register("tfidf", func(_ context.Context, _ ProviderConfig) (Embedder, string, error) {
		const dims = 256
		return NewTFIDFEmbedder(dims), fmt.Sprintf("tfidf(dim=%d)", dims), nil
	})
}

// TFIDFEmbedder generates embeddings using character 3-gram TF-IDF.
// Used as a fallback when API embedding is unavailable.
type TFIDFEmbedder struct {
	Dimensions int
}

func NewTFIDFEmbedder(dims int) *TFIDFEmbedder {
	if dims <= 0 {
		dims = 256
	}
	return &TFIDFEmbedder{Dimensions: dims}
}

func (t *TFIDFEmbedder) Embed(_ context.Context, blocks []model.Block, _ EmbedOptions) ([]model.Embedding, error) {
	// Build document-level vocabulary
	docFreq := make(map[string]int)
	blockGrams := make([]map[string]int, len(blocks))

	for i, b := range blocks {
		grams := charNGrams(strings.ToLower(b.Text), 3)
		blockGrams[i] = grams
		seen := make(map[string]bool)
		for g := range grams {
			if !seen[g] {
				docFreq[g]++
				seen[g] = true
			}
		}
	}

	n := float64(len(blocks))
	results := make([]model.Embedding, len(blocks))
	for i, grams := range blockGrams {
		vec := make([]float64, t.Dimensions)
		for gram, count := range grams {
			tf := float64(count)
			idf := math.Log(1 + n/float64(docFreq[gram]+1))
			idx := hashGram(gram) % t.Dimensions
			vec[idx] += tf * idf
		}
		// Normalize
		var norm float64
		for _, v := range vec {
			norm += v * v
		}
		if norm > 0 {
			norm = math.Sqrt(norm)
			for j := range vec {
				vec[j] /= norm
			}
		}
		results[i] = model.Embedding{BlockIndex: i, Vector: vec}
	}

	return results, nil
}

func charNGrams(text string, n int) map[string]int {
	grams := make(map[string]int)
	runes := []rune(text)
	for i := 0; i <= len(runes)-n; i++ {
		gram := string(runes[i : i+n])
		grams[gram]++
	}
	return grams
}

func hashGram(s string) int {
	var h uint64
	for _, r := range s {
		h = h*31 + uint64(r)
	}
	return int(h & 0x7fffffffffffffff)
}
