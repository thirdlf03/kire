package segment

import (
	"slices"

	"github.com/thirdlf03/kire/internal/boundary"
	"github.com/thirdlf03/kire/internal/model"
)

// AutoMaxLines computes a max-lines value from the total line count,
// targeting ~10 segments clamped to [200, 2000].
func AutoMaxLines(totalLines int) int {
	v := totalLines / 10
	if v < 200 {
		return 200
	}
	if v > 2000 {
		return 2000
	}
	return v
}

// OptConfig configures segment optimization.
type OptConfig struct {
	MinTokens          int
	MaxTokens          int
	MaxLines           int
	PackHeadingBarrier int // heading level barrier for packing (0 = disabled)
}

// Optimize splits blocks into segments using boundary information and token constraints.
func Optimize(blocks []model.Block, br boundary.BoundaryResult, config OptConfig) []model.Segment {
	if len(blocks) == 0 {
		return nil
	}

	// Adjust boundaries: shift boundary after heading to before heading
	boundaries := adjustHeadingBoundaries(blocks, br.Boundaries)

	// Initial split at boundaries
	segments := splitAtBoundaries(blocks, boundaries)

	// Merge undersized segments
	segments = mergeUndersized(segments, config.MinTokens)

	// Re-split oversized segments
	segments = splitOversized(segments, config.MaxTokens, config.MaxLines, br.DepthScores, blocks)

	// Pack small segments together up to limits
	segments = packSegments(segments, config.MaxTokens, config.MaxLines, config.PackHeadingBarrier)

	return segments
}

// PackSegments packs adjacent segments greedily within the configured limits.
func PackSegments(segments []model.Segment, config OptConfig) []model.Segment {
	return packSegments(segments, config.MaxTokens, config.MaxLines, config.PackHeadingBarrier)
}

// adjustHeadingBoundaries shifts boundaries that fall right after a heading
// to right before the heading.
func adjustHeadingBoundaries(blocks []model.Block, boundaries []int) []int {
	if len(boundaries) == 0 {
		return boundaries
	}
	adjusted := slices.Clone(boundaries)

	for i, b := range adjusted {
		// A boundary at index b means a split between block[b] and block[b+1].
		// If block[b+1] exists and is a heading, we want the heading to start
		// the next segment. But the boundary already does that—block[b+1] starts next segment.
		// The issue is when the boundary is at the heading itself:
		// If block[b] is a heading, move boundary to b-1 so heading starts the next segment.
		if b < len(blocks) && blocks[b].Kind == model.BlockHeading && b > 0 {
			adjusted[i] = b - 1
		}
	}

	// Deduplicate and sort
	return dedupSorted(adjusted)
}

func dedupSorted(sorted []int) []int {
	if len(sorted) <= 1 {
		return sorted
	}
	result := []int{sorted[0]}
	for i := 1; i < len(sorted); i++ {
		if sorted[i] != sorted[i-1] {
			result = append(result, sorted[i])
		}
	}
	return result
}

// splitAtBoundaries splits blocks into segments at the given boundary indices.
// A boundary at index i means the split occurs after block[i].
func splitAtBoundaries(blocks []model.Block, boundaries []int) []model.Segment {
	if len(boundaries) == 0 {
		return []model.Segment{makeSegment(blocks)}
	}

	var segments []model.Segment
	start := 0
	for _, b := range boundaries {
		end := b + 1
		if end > len(blocks) {
			end = len(blocks)
		}
		if start < end {
			segments = append(segments, makeSegment(blocks[start:end]))
		}
		start = end
	}
	if start < len(blocks) {
		segments = append(segments, makeSegment(blocks[start:]))
	}

	return segments
}

func makeSegment(blocks []model.Block) model.Segment {
	if len(blocks) == 0 {
		return model.Segment{}
	}
	tokens := 0
	lines := 0
	for _, b := range blocks {
		tokens += b.TokensApprox
		lines += b.LinesApprox
	}
	return model.Segment{
		Blocks:      slices.Clone(blocks),
		Range:       model.SourceRange{Start: blocks[0].Range.Start, End: blocks[len(blocks)-1].Range.End},
		TokenCount:  tokens,
		LineCount:   lines,
		HeadingPath: blocks[0].HeadingPath,
	}
}

// mergeUndersized merges segments that are below the minimum token count
// with the adjacent segment that has fewer tokens.
func mergeUndersized(segments []model.Segment, minTokens int) []model.Segment {
	if minTokens <= 0 || len(segments) <= 1 {
		return segments
	}

	for {
		merged := false
		for i := 0; i < len(segments); i++ {
			if segments[i].TokenCount < minTokens && len(segments) > 1 {
				if i == 0 {
					segments = mergeAt(segments, 0)
				} else if i == len(segments)-1 {
					segments = mergeAt(segments, i-1)
				} else {
					if segments[i-1].TokenCount <= segments[i+1].TokenCount {
						segments = mergeAt(segments, i-1)
					} else {
						segments = mergeAt(segments, i)
					}
				}
				merged = true
				break
			}
		}
		if !merged {
			break
		}
	}

	return segments
}

func mergeAt(segments []model.Segment, idx int) []model.Segment {
	if idx+1 >= len(segments) {
		return segments
	}
	combined := slices.Concat(segments[idx].Blocks, segments[idx+1].Blocks)
	merged := makeSegment(combined)
	return slices.Concat(segments[:idx], []model.Segment{merged}, segments[idx+2:])
}

func isOversized(seg model.Segment, maxTokens, maxLines int) bool {
	if maxTokens > 0 && seg.TokenCount > maxTokens {
		return true
	}
	if maxLines > 0 && seg.LineCount > maxLines {
		return true
	}
	return false
}

// splitOversized re-splits segments that exceed the maximum token count or line count.
func splitOversized(segments []model.Segment, maxTokens, maxLines int, depthScores []float64, allBlocks []model.Block) []model.Segment {
	if maxTokens <= 0 && maxLines <= 0 {
		return segments
	}

	var result []model.Segment
	for _, seg := range segments {
		if !isOversized(seg, maxTokens, maxLines) {
			result = append(result, seg)
			continue
		}
		offset := findBlockOffset(seg.Blocks, allBlocks)
		split := resplit(seg.Blocks, maxTokens, maxLines, depthScores, offset, 0)
		result = append(result, split...)
	}

	return result
}

func findBlockOffset(segBlocks []model.Block, allBlocks []model.Block) int {
	if len(segBlocks) == 0 || len(allBlocks) == 0 {
		return 0
	}
	for i := range allBlocks {
		if allBlocks[i].Range.Start == segBlocks[0].Range.Start {
			return i
		}
	}
	return 0
}

const maxResplitDepth = 20

// resplit divides an oversized segment's blocks using depth scores.
// depth limits recursion to prevent stack overflow.
func resplit(blocks []model.Block, maxTokens, maxLines int, depthScores []float64, blockOffset int, depth int) []model.Segment {
	if len(blocks) <= 1 || depth >= maxResplitDepth {
		return []model.Segment{makeSegment(blocks)}
	}

	// Find the best internal split point by highest depth score
	bestIdx := -1
	bestScore := -1.0
	for i := 0; i < len(blocks)-1; i++ {
		globalIdx := blockOffset + i
		if globalIdx < len(depthScores) {
			score := depthScores[globalIdx]
			if score > bestScore {
				bestScore = score
				bestIdx = i
			}
		}
	}

	if bestIdx < 0 {
		// No depth scores available; split at midpoint
		bestIdx = len(blocks)/2 - 1
		if bestIdx < 0 {
			bestIdx = 0
		}
	}

	left := blocks[:bestIdx+1]
	right := blocks[bestIdx+1:]

	var result []model.Segment
	leftSeg := makeSegment(left)
	if isOversized(leftSeg, maxTokens, maxLines) && len(left) > 1 {
		result = append(result, resplit(left, maxTokens, maxLines, depthScores, blockOffset, depth+1)...)
	} else {
		result = append(result, leftSeg)
	}

	rightSeg := makeSegment(right)
	if isOversized(rightSeg, maxTokens, maxLines) && len(right) > 1 {
		result = append(result, resplit(right, maxTokens, maxLines, depthScores, blockOffset+bestIdx+1, depth+1)...)
	} else {
		result = append(result, rightSeg)
	}

	return result
}

// isHeadingBarrier reports whether the segment starts with a heading (or pseudo-heading)
// that should prevent merging with the previous segment.
func isHeadingBarrier(seg model.Segment, barrier int) bool {
	if barrier <= 0 || len(seg.Blocks) == 0 {
		return false
	}
	b := seg.Blocks[0]
	if b.IsPseudoHeading {
		return true
	}
	return b.Kind == model.BlockHeading && b.HeadingLevel <= barrier
}

// packSegments greedily merges adjacent segments as long as the merged result
// does not exceed maxTokens or maxLines.
// headingBarrier prevents merging when the next segment starts with a heading
// of level <= barrier (0 = disabled).
func packSegments(segments []model.Segment, maxTokens, maxLines, headingBarrier int) []model.Segment {
	if (maxTokens <= 0 && maxLines <= 0) || len(segments) <= 1 {
		return segments
	}
	var result []model.Segment
	current := segments[0]
	for i := 1; i < len(segments); i++ {
		if isHeadingBarrier(segments[i], headingBarrier) {
			result = append(result, current)
			current = segments[i]
			continue
		}
		merged := mergeTwo(current, segments[i])
		tokenOK := maxTokens <= 0 || merged.TokenCount <= maxTokens
		lineOK := maxLines <= 0 || merged.LineCount <= maxLines
		if tokenOK && lineOK {
			current = merged
		} else {
			result = append(result, current)
			current = segments[i]
		}
	}
	result = append(result, current)
	return result
}

// mergeTwo creates a new segment by combining two adjacent segments.
func mergeTwo(a, b model.Segment) model.Segment {
	return makeSegment(slices.Concat(a.Blocks, b.Blocks))
}
