package pipeline

import (
	"github.com/thirdlf03/kire/internal/boundary"
	"github.com/thirdlf03/kire/internal/model"
)

// Hooks provides callback functions for pipeline stages.
// Each hook is optional (nil = skipped). Returning an error stops the pipeline.
type Hooks struct {
	OnParse    func(blocks []model.Block) error
	OnEmbed    func(embeddings []model.Embedding) error
	OnBoundary func(result boundary.BoundaryResult) error
	OnSegment  func(segments []model.Segment) error
	OnRender   func(index int, content string) error
}
