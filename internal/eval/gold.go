package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
)

// GoldAnnotation holds the reference segmentation for a document.
type GoldAnnotation struct {
	DocID      string            `json:"doc_id"`
	Unit       string            `json:"unit"`             // "block"
	Boundaries []int             `json:"boundaries"`       // sorted gap indices
	Params     *GoldParams       `json:"params,omitempty"` // recommended bench parameters
	Notes      map[string]string `json:"notes,omitempty"`
}

// GoldParams holds recommended bench parameters for this annotation.
// All fields are optional; nil means "use default".
type GoldParams struct {
	EvalStage            string   `json:"eval_stage,omitempty"`
	Tolerance            *int     `json:"tolerance,omitempty"`
	K                    *int     `json:"k,omitempty"`
	MinGap               *int     `json:"min_gap,omitempty"`
	MinTokens            *int     `json:"min_tokens,omitempty"`
	MaxTokens            *int     `json:"max_tokens,omitempty"`
	MaxLines             *int     `json:"max_lines,omitempty"`
	Window               *int     `json:"window,omitempty"`
	Threshold            *float64 `json:"threshold,omitempty"`
	SplitCount           *int     `json:"split_count,omitempty"`
	PackHeadingBarrier   *int     `json:"pack_heading_barrier,omitempty"`
	PseudoHeading        *bool    `json:"pseudo_heading,omitempty"`
	BoundaryHints        *bool    `json:"boundary_hints,omitempty"`
	SectionLock          *bool    `json:"section_lock,omitempty"`
	LockAfterHeading     *bool    `json:"lock_after_heading,omitempty"`
	ParaSplitPattern     *string  `json:"para_split_pattern,omitempty"`
	ForceBoundary        *string  `json:"force_boundary,omitempty"`
	SuppressListBoundary *bool    `json:"suppress_list_boundary,omitempty"`
	AtomicBoundary       *bool    `json:"atomic_boundary,omitempty"`
	BaselineMinGap       *bool    `json:"baseline_min_gap,omitempty"`
	AllowBlockMismatch   *bool    `json:"allow_block_mismatch,omitempty"`
}

// LoadGold reads a gold annotation from a JSON file.
func LoadGold(path string) (*GoldAnnotation, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load gold %s: %w", path, err)
	}
	var ann GoldAnnotation
	if err := json.Unmarshal(data, &ann); err != nil {
		return nil, fmt.Errorf("parse gold %s: %w", path, err)
	}
	slices.Sort(ann.Boundaries)
	return &ann, nil
}

// SaveGold writes a gold annotation to a JSON file.
func SaveGold(path string, ann *GoldAnnotation) error {
	slices.Sort(ann.Boundaries)
	data, err := json.MarshalIndent(ann, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal gold: %w", err)
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
}
