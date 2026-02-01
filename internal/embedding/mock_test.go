package embedding_test

import (
	"context"
	"testing"

	"github.com/thirdlf03/kire/internal/embedding"
	"github.com/thirdlf03/kire/internal/model"
)

func TestNewMockEmbedder_DefaultDimensions(t *testing.T) {
	m := embedding.NewMockEmbedder(0)
	if m.Dimensions != 128 {
		t.Errorf("expected default 128, got %d", m.Dimensions)
	}
}

func TestNewMockEmbedder_NegativeDimensions(t *testing.T) {
	m := embedding.NewMockEmbedder(-1)
	if m.Dimensions != 128 {
		t.Errorf("expected default 128, got %d", m.Dimensions)
	}
}

func TestMockEmbedder_EmptyBlocks(t *testing.T) {
	m := embedding.NewMockEmbedder(64)
	results, err := m.Embed(context.Background(), nil, embedding.EmbedOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestMockEmbedder_EmptyText(t *testing.T) {
	m := embedding.NewMockEmbedder(64)
	blocks := []model.Block{{Kind: model.BlockParagraph, Text: ""}}
	results, err := m.Embed(context.Background(), blocks, embedding.EmbedOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	// Empty text produces a zero vector
	for i, v := range results[0].Vector {
		if v != 0 {
			t.Errorf("expected zero vector, but vec[%d] = %f", i, v)
			break
		}
	}
}
