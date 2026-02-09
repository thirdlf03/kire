package segment

import (
	"slices"
	"strings"
	"unicode"

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

	// Adjust boundaries: snap to heading runs, then snap flat-doc boundaries
	boundaries := AdjustHeadingBoundaries(blocks, br.Boundaries)
	boundaries = adjustFlatBoundaries(blocks, boundaries)

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

// adjustHeadingBoundaries snaps boundaries into heading runs so that
// headings always start the next segment.
//
// A boundary at gap b means a split between block[b] and block[b+1].
// If block[b] is a heading, the heading would end the current segment
// instead of starting the next one. This function walks backward through
// any consecutive heading run and places the boundary just before the
// first heading in the run.
func AdjustHeadingBoundaries(blocks []model.Block, boundaries []int) []int {
	if len(boundaries) == 0 {
		return boundaries
	}
	adjusted := slices.Clone(boundaries)

	for i, b := range adjusted {
		if b < len(blocks) && blocks[b].Kind == model.BlockHeading && b > 0 {
			// Walk backward through consecutive headings to find the run start.
			start := b
			for start > 0 && blocks[start-1].Kind == model.BlockHeading {
				start--
			}
			if start > 0 {
				adjusted[i] = start - 1
			} else {
				// Heading run starts at block 0; place boundary at gap 0
				// so the first heading begins the next segment.
				adjusted[i] = 0
			}
		}
	}

	// Deduplicate and sort
	return dedupSorted(adjusted)
}

const flatSnapWindow = 3

// adjustFlatBoundaries snaps boundaries in flat documents (no headings) using
// IDF-weighted block-fit analysis. For each boundary at gap b, it checks whether
// block[b] fits better with the preceding or following blocks. If it fits better
// with the following blocks, the boundary snaps to b-1 so that block[b] starts
// the next segment instead of ending the current one.
func adjustFlatBoundaries(blocks []model.Block, boundaries []int) []int {
	if len(boundaries) == 0 || hasHeadings(blocks) {
		return boundaries
	}

	sets := make([]map[string]struct{}, len(blocks))
	for i := range blocks {
		sets[i] = tokenSet(blocks[i].Text)
	}
	df := docFrequency(sets)
	n := len(blocks)

	adjusted := slices.Clone(boundaries)
	for i, b := range adjusted {
		if b <= 0 || b >= n {
			continue
		}
		prevSet := mergeTokenSets(sets, max(0, b-flatSnapWindow), b)
		nextSet := mergeTokenSets(sets, b+1, min(n, b+1+flatSnapWindow))
		fitPrev := idfSimilarity(sets[b], prevSet, df)
		fitNext := idfSimilarity(sets[b], nextSet, df)
		if fitNext > fitPrev {
			adjusted[i] = b - 1
		}
	}
	return dedupSorted(adjusted)
}

// hasHeadings reports whether any block is a heading.
func hasHeadings(blocks []model.Block) bool {
	for _, b := range blocks {
		if b.Kind == model.BlockHeading {
			return true
		}
	}
	return false
}

// tokenSet extracts a set of tokens from text. ASCII words are extracted
// normally; CJK characters (Han, Katakana, Hiragana) are treated as
// individual tokens since each character carries semantic meaning.
func tokenSet(text string) map[string]struct{} {
	set := make(map[string]struct{})
	lower := strings.ToLower(text)
	var word strings.Builder
	for _, r := range lower {
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Katakana, r) || unicode.Is(unicode.Hiragana, r) {
			if word.Len() > 0 {
				set[word.String()] = struct{}{}
				word.Reset()
			}
			set[string(r)] = struct{}{}
		} else if unicode.IsLetter(r) || unicode.IsDigit(r) {
			word.WriteRune(r)
		} else {
			if word.Len() > 0 {
				set[word.String()] = struct{}{}
				word.Reset()
			}
		}
	}
	if word.Len() > 0 {
		set[word.String()] = struct{}{}
	}
	return set
}

// mergeTokenSets unions the token sets of blocks[from:to].
func mergeTokenSets(sets []map[string]struct{}, from, to int) map[string]struct{} {
	merged := make(map[string]struct{})
	for i := from; i < to; i++ {
		for w := range sets[i] {
			merged[w] = struct{}{}
		}
	}
	return merged
}

// docFrequency counts how many blocks each token appears in.
func docFrequency(sets []map[string]struct{}) map[string]int {
	df := make(map[string]int)
	for _, s := range sets {
		for w := range s {
			df[w]++
		}
	}
	return df
}

// idfSimilarity computes IDF-weighted Jaccard: sum(IDF for intersection) / sum(IDF for union).
// IDF is defined as 1/df(token), giving higher weight to rare tokens.
func idfSimilarity(a, b map[string]struct{}, df map[string]int) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1.0
	}
	var interIDF, unionIDF float64
	all := make(map[string]struct{}, len(a)+len(b))
	for w := range a {
		all[w] = struct{}{}
	}
	for w := range b {
		all[w] = struct{}{}
	}
	for w := range all {
		idf := 1.0
		if c := df[w]; c > 0 {
			idf = 1.0 / float64(c)
		}
		unionIDF += idf
		_, inA := a[w]
		_, inB := b[w]
		if inA && inB {
			interIDF += idf
		}
	}
	if unionIDF == 0 {
		return 1.0
	}
	return interIDF / unionIDF
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
