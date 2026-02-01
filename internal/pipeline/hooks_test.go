package pipeline

import (
	"context"
	"fmt"
	"testing"

	"github.com/thirdlf03/kire/internal/boundary"
	"github.com/thirdlf03/kire/internal/embedding"
	"github.com/thirdlf03/kire/internal/model"
	"github.com/thirdlf03/kire/internal/tokenizer"
)

func TestHooksAreCalledInOrder(t *testing.T) {
	source := []byte("# Title\n\nParagraph one.\n\n## Section\n\nParagraph two.\n")

	var callOrder []string

	hooks := &Hooks{
		OnParse: func(blocks []model.Block) error {
			callOrder = append(callOrder, "parse")
			if len(blocks) == 0 {
				t.Error("OnParse received empty blocks")
			}
			return nil
		},
		OnEmbed: func(embeddings []model.Embedding) error {
			callOrder = append(callOrder, "embed")
			if len(embeddings) == 0 {
				t.Error("OnEmbed received empty embeddings")
			}
			return nil
		},
		OnBoundary: func(result boundary.BoundaryResult) error {
			callOrder = append(callOrder, "boundary")
			return nil
		},
		OnSegment: func(segments []model.Segment) error {
			callOrder = append(callOrder, "segment")
			if len(segments) == 0 {
				t.Error("OnSegment received empty segments")
			}
			return nil
		},
		OnRender: func(index int, content string) error {
			callOrder = append(callOrder, fmt.Sprintf("render-%d", index))
			return nil
		},
	}

	cfg := Config{
		Source:         source,
		SourceName:     "test.md",
		Embedder:       embedding.NewMockEmbedder(128),
		TokenEstimator: tokenizer.NewLocalEstimator(),
		MinTokens:      0,
		MaxTokens:      10000,
		Window:         1,
		MinGap:         1,
		Hooks:          hooks,
	}

	_, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Verify order
	if len(callOrder) < 4 {
		t.Fatalf("expected at least 4 hook calls, got %d: %v", len(callOrder), callOrder)
	}
	if callOrder[0] != "parse" {
		t.Errorf("expected first call to be 'parse', got %q", callOrder[0])
	}
	if callOrder[1] != "embed" {
		t.Errorf("expected second call to be 'embed', got %q", callOrder[1])
	}
	if callOrder[2] != "boundary" {
		t.Errorf("expected third call to be 'boundary', got %q", callOrder[2])
	}
	if callOrder[3] != "segment" {
		t.Errorf("expected fourth call to be 'segment', got %q", callOrder[3])
	}
}

func TestHookErrorStopsPipeline(t *testing.T) {
	source := []byte("# Title\n\nParagraph one.\n")

	hooks := &Hooks{
		OnParse: func(blocks []model.Block) error {
			return fmt.Errorf("parse hook error")
		},
	}

	cfg := Config{
		Source:         source,
		SourceName:     "test.md",
		Embedder:       embedding.NewMockEmbedder(128),
		TokenEstimator: tokenizer.NewLocalEstimator(),
		MinTokens:      0,
		MaxTokens:      10000,
		Window:         1,
		MinGap:         1,
		Hooks:          hooks,
	}

	_, err := Run(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error from hook, got nil")
	}
	if err.Error() != "hook OnParse: parse hook error" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestNilHooksNoError(t *testing.T) {
	source := []byte("# Title\n\nParagraph one.\n")

	cfg := Config{
		Source:         source,
		SourceName:     "test.md",
		Embedder:       embedding.NewMockEmbedder(128),
		TokenEstimator: tokenizer.NewLocalEstimator(),
		MinTokens:      0,
		MaxTokens:      10000,
		Window:         1,
		MinGap:         1,
		Hooks:          nil, // no hooks
	}

	_, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error with nil hooks: %v", err)
	}
}
