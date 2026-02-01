package parser

import (
	"regexp"
	"strings"

	"github.com/thirdlf03/kire/internal/model"
)

// SplitLongParagraphs splits paragraph blocks at lines matching any of the
// given patterns. Non-paragraph blocks are passed through unchanged.
// The source parameter is needed to recalculate SourceRange byte offsets.
func SplitLongParagraphs(blocks []model.Block, patterns []*regexp.Regexp, source []byte) []model.Block {
	if len(patterns) == 0 {
		return blocks
	}

	var result []model.Block
	for _, b := range blocks {
		if b.Kind != model.BlockParagraph {
			result = append(result, b)
			continue
		}

		splits := splitParagraph(b, patterns, source)
		result = append(result, splits...)
	}
	return result
}

// splitParagraph splits a single paragraph block at lines matching patterns.
// Each matching line starts a new block. Text before the first match also
// becomes its own block. Returns the original block if no match is found.
func splitParagraph(b model.Block, patterns []*regexp.Regexp, source []byte) []model.Block {
	text := b.Text
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if len(lines) <= 1 {
		return []model.Block{b}
	}

	// Find line indices where patterns match (these are split points)
	var splitPoints []int
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		for _, pat := range patterns {
			if pat.MatchString(trimmed) {
				splitPoints = append(splitPoints, i)
				break
			}
		}
	}

	if len(splitPoints) == 0 {
		return []model.Block{b}
	}

	return splitParagraphAtLines(b, splitPoints, source)
}

// splitParagraphAtLines splits a paragraph block at the given line indices.
// Each line at a split index becomes a single-line block, and surrounding
// non-split lines form their own blocks.
// Returns the original block if no effective split is produced.
func splitParagraphAtLines(b model.Block, splitLineIndices []int, source []byte) []model.Block {
	if len(splitLineIndices) == 0 {
		return []model.Block{b}
	}

	text := b.Text
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if len(lines) <= 1 {
		return []model.Block{b}
	}

	// Build segment boundaries.
	// Each split line becomes a single-line block, and surrounding non-split
	// lines form their own blocks. This produces: [before?] [split1] [between?] [split2] [after?] ...
	matchSet := make(map[int]bool, len(splitLineIndices))
	for _, sp := range splitLineIndices {
		matchSet[sp] = true
	}

	// Build line byte offsets within the block's source range
	blockSource := source[b.Range.Start:b.Range.End]
	lineOffsets := computeLineOffsets(blockSource)

	type seg struct {
		startLine int
		endLine   int // exclusive
	}

	var segments []seg
	i := 0
	for i < len(lines) {
		if matchSet[i] {
			// Split line → single-line segment
			segments = append(segments, seg{startLine: i, endLine: i + 1})
			i++
		} else {
			// Non-split lines → collect consecutive non-split
			start := i
			for i < len(lines) && !matchSet[i] {
				i++
			}
			segments = append(segments, seg{startLine: start, endLine: i})
		}
	}

	if len(segments) <= 1 {
		return []model.Block{b}
	}

	// Convert segments to blocks
	var result []model.Block
	for j, s := range segments {
		startByte := b.Range.Start + lineOffsets[s.startLine]
		var endByte int
		if j == len(segments)-1 {
			endByte = b.Range.End
		} else {
			endByte = b.Range.Start + lineOffsets[segments[j+1].startLine]
		}

		segText := strings.Join(lines[s.startLine:s.endLine], "\n")

		result = append(result, model.Block{
			Kind:        b.Kind,
			Text:        segText,
			HeadingPath: b.HeadingPath,
			Range:       model.SourceRange{Start: startByte, End: endByte},
		})
	}

	return result
}

// computeLineOffsets returns the byte offset of each line start within data.
// lineOffsets[0] = 0, lineOffsets[1] = offset of second line, etc.
func computeLineOffsets(data []byte) []int {
	offsets := []int{0}
	for i, b := range data {
		if b == '\n' {
			offsets = append(offsets, i+1)
		}
	}
	return offsets
}
