package segment

import (
	"regexp"
	"strings"

	"github.com/thirdlf03/kire/internal/model"
)

var (
	numberedLeadRe   = regexp.MustCompile(`^\d+\)\s+`)
	httpMethodLeadRe = regexp.MustCompile(`(?:GET|POST|PUT|PATCH|DELETE)\s+/`)
)

// LockRange represents a range of block indices that should not be split.
type LockRange struct {
	Start int // Inclusive start block index
	End   int // Exclusive end block index
}

// LockConfig configures section lock detection.
type LockConfig struct {
	Enabled          bool
	MaxLockBlocks    int  // Maximum blocks in a lock range (default 10)
	LockAfterHeading bool // Also treat headings (incl. pseudo) as lead lines
}

// DefaultLockConfig returns a config with sensible defaults.
func DefaultLockConfig() LockConfig {
	return LockConfig{
		Enabled:          true,
		MaxLockBlocks:    10,
		LockAfterHeading: true,
	}
}

// isLeadLine returns true if the block looks like a "lead" line.
// Matches: colon-suffix lines, or numbered HTTP endpoint lines (e.g. "1) GET /api/tasks").
func isLeadLine(b model.Block) bool {
	text := strings.TrimSpace(b.Text)
	if text == "" {
		return false
	}

	// Colon-suffix lead
	if b.Kind == model.BlockParagraph || b.Kind == model.BlockHeading {
		if strings.HasSuffix(text, ":") || strings.HasSuffix(text, "：") {
			return true
		}
	}

	// Numbered HTTP endpoint lead (e.g. "1) GET /api/tasks")
	firstLine := text
	if idx := strings.IndexByte(text, '\n'); idx >= 0 {
		firstLine = text[:idx]
	}
	firstLine = strings.TrimSpace(firstLine)
	if numberedLeadRe.MatchString(firstLine) && httpMethodLeadRe.MatchString(firstLine) {
		return true
	}

	return false
}

// DetectLockRanges finds ranges of blocks that should be kept together.
// A lock range starts at a lead line (ends with colon) and extends until
// the next heading, lead line, or MaxLockBlocks is reached.
// When LockAfterHeading is true, heading blocks (including pseudo-headings)
// also act as lead lines.
func DetectLockRanges(blocks []model.Block, cfg LockConfig) []LockRange {
	if !cfg.Enabled || len(blocks) == 0 {
		return nil
	}
	maxBlocks := cfg.MaxLockBlocks
	if maxBlocks <= 0 {
		maxBlocks = 10
	}

	isLead := func(b model.Block) bool {
		if isLeadLine(b) {
			return true
		}
		if cfg.LockAfterHeading && b.Kind == model.BlockHeading {
			return true
		}
		return false
	}

	var ranges []LockRange
	i := 0
	for i < len(blocks) {
		if !isLead(blocks[i]) {
			i++
			continue
		}
		// Found a lead line; determine the end of the lock range
		end := i + 1
		for end < len(blocks) && end-i < maxBlocks {
			// Stop before the next heading or lead line
			if blocks[end].Kind == model.BlockHeading {
				break
			}
			if isLeadLine(blocks[end]) {
				break
			}
			end++
		}
		// Only create a lock if there's at least one block after the lead
		if end > i+1 {
			ranges = append(ranges, LockRange{Start: i, End: end})
		}
		i = end
	}
	return ranges
}

// LockedGaps returns a set of gap indices that fall within lock ranges.
// A gap index i represents the gap between block[i] and block[i+1].
func LockedGaps(ranges []LockRange) map[int]bool {
	locked := make(map[int]bool)
	for _, r := range ranges {
		for i := r.Start; i < r.End-1; i++ {
			locked[i] = true
		}
	}
	return locked
}
