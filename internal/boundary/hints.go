package boundary

import (
	"log"
	"regexp"
	"strings"

	"github.com/thirdlf03/kire/internal/model"
)

// HintRule defines a pattern-based rule for generating boundary hints.
type HintRule struct {
	// Match tests whether a block matches this rule.
	Match func(block model.Block) bool
	// Kind is the hint kind to generate.
	Kind HintKind
	// Offset: 0 = gap after the block, -1 = gap before the block.
	Offset int
}

var apiEndpointRe = regexp.MustCompile(`(?:^\d+\)\s+(?:GET|POST|PUT|PATCH|DELETE)\s|^(?:GET|POST|PUT|PATCH|DELETE)\s+/)`)

// DefaultHintRules returns the standard set of hint rules.
func DefaultHintRules() []HintRule {
	return []HintRule{
		// Prohibit: do not split right after a colon-suffix "lead" line
		{
			Match: func(b model.Block) bool {
				if b.Kind != model.BlockParagraph && b.Kind != model.BlockHeading {
					return false
				}
				text := strings.TrimSpace(b.Text)
				return strings.HasSuffix(text, ":") || strings.HasSuffix(text, "：")
			},
			Kind:   HintProhibit,
			Offset: 0, // gap after this block
		},
		// Prefer: boost a boundary before an API endpoint line
		{
			Match: func(b model.Block) bool {
				if b.Kind != model.BlockParagraph {
					return false
				}
				text := strings.TrimSpace(b.Text)
				return apiEndpointRe.MatchString(text)
			},
			Kind:   HintPrefer,
			Offset: -1, // gap before this block
		},
	}
}

// UserEnforceRules creates HintEnforce rules from user-specified regex patterns.
// Each pattern matches against block text; a match generates HintEnforce at
// the gap before the matching block (Offset=-1). Invalid regex patterns are skipped.
func UserEnforceRules(patterns []string) []HintRule {
	var rules []HintRule
	for _, pat := range patterns {
		re, err := regexp.Compile(pat)
		if err != nil {
			log.Printf("WARNING: invalid force-boundary pattern %q: %v", pat, err)
			continue
		}
		capturedRe := re
		rules = append(rules, HintRule{
			Match: func(b model.Block) bool {
				text := strings.TrimSpace(b.Text)
				return capturedRe.MatchString(text)
			},
			Kind:   HintEnforce,
			Offset: -1, // gap before this block
		})
	}
	return rules
}

// GenerateListHints directly generates prohibit hints for list→list and
// heading→list gaps. This cannot be expressed as single-block HintRules.
func GenerateListHints(blocks []model.Block) []BoundaryHint {
	var hints []BoundaryHint
	for i := 0; i < len(blocks)-1; i++ {
		cur := blocks[i].Kind
		next := blocks[i+1].Kind
		if cur == model.BlockList && next == model.BlockList {
			hints = append(hints, BoundaryHint{GapIndex: i, Kind: HintProhibit})
		}
		if cur == model.BlockHeading && next == model.BlockList {
			hints = append(hints, BoundaryHint{GapIndex: i, Kind: HintProhibit})
		}
	}
	return hints
}

// GenerateAtomicHints generates HintProhibit at gaps immediately before
// code blocks, tables, and math blocks to prevent splitting them from
// their preceding context (e.g., a lead-in paragraph).
func GenerateAtomicHints(blocks []model.Block) []BoundaryHint {
	var hints []BoundaryHint
	for i := 1; i < len(blocks); i++ {
		k := blocks[i].Kind
		if k == model.BlockCodeBlock || k == model.BlockTable || k == model.BlockMathBlock {
			hints = append(hints, BoundaryHint{GapIndex: i - 1, Kind: HintProhibit})
		}
	}
	return hints
}

// GenerateHints applies rules to blocks and returns hints.
// The gap index for block i is i (gap between block[i] and block[i+1]).
// For Offset=-1, the gap is i-1 (gap before block[i]).
func GenerateHints(blocks []model.Block, rules []HintRule) []BoundaryHint {
	var hints []BoundaryHint
	for i, b := range blocks {
		for _, rule := range rules {
			if !rule.Match(b) {
				continue
			}
			gapIdx := i + rule.Offset
			if gapIdx < 0 || gapIdx >= len(blocks)-1 {
				continue
			}
			hints = append(hints, BoundaryHint{
				GapIndex: gapIdx,
				Kind:     rule.Kind,
			})
		}
	}
	return hints
}
