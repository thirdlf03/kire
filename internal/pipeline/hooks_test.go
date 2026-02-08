package pipeline

import (
	"context"
	"fmt"
	"testing"

	"github.com/thirdlf03/kire/internal/boundary"
	"github.com/thirdlf03/kire/internal/model"
	"github.com/thirdlf03/kire/internal/tokenizer"
)

// internalStubDetector is a test-only boundary detector.
type internalStubDetector struct {
	boundaries []int
}

func (s *internalStubDetector) DetectBoundaries(_ context.Context, blocks []model.Block) (boundary.BoundaryResult, error) {
	return boundary.BoundaryResult{
		Boundaries:  s.boundaries,
		DepthScores: make([]float64, max(len(blocks)-1, 0)),
	}, nil
}

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
		Source:           source,
		SourceName:       "test.md",
		TokenEstimator:   tokenizer.NewLocalEstimator(),
		BoundaryDetector: &internalStubDetector{boundaries: []int{2}},
		Hooks:            hooks,
		ContextFormat:    "none",
	}

	_, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Verify order: parse → boundary → segment → render-*
	if len(callOrder) < 4 {
		t.Fatalf("expected at least 4 hook calls, got %d: %v", len(callOrder), callOrder)
	}
	if callOrder[0] != "parse" {
		t.Errorf("expected first call to be 'parse', got %q", callOrder[0])
	}
	if callOrder[1] != "boundary" {
		t.Errorf("expected second call to be 'boundary', got %q", callOrder[1])
	}
	if callOrder[2] != "segment" {
		t.Errorf("expected third call to be 'segment', got %q", callOrder[2])
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
		Source:           source,
		SourceName:       "test.md",
		TokenEstimator:   tokenizer.NewLocalEstimator(),
		BoundaryDetector: &internalStubDetector{},
		Hooks:            hooks,
		ContextFormat:    "none",
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
		Source:           source,
		SourceName:       "test.md",
		TokenEstimator:   tokenizer.NewLocalEstimator(),
		BoundaryDetector: &internalStubDetector{},
		Hooks:            nil,
		ContextFormat:    "none",
	}

	_, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error with nil hooks: %v", err)
	}
}
