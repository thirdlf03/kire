package parser

import (
	mathjax "github.com/litao91/goldmark-mathjax"
	"github.com/thirdlf03/kire/internal/model"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
)

// Parse parses markdown source into a slice of Blocks with source ranges and heading paths.
func Parse(source []byte) ([]model.Block, error) {
	md := goldmark.New(
		goldmark.WithExtensions(extension.Table, mathjax.MathJax),
	)
	reader := text.NewReader(source)
	doc := md.Parser().Parse(reader)

	// First pass: collect raw blocks with their AST-derived ranges
	var rawBlocks []rawBlock
	var headingStack []headingEntry

	for child := doc.FirstChild(); child != nil; child = child.NextSibling() {
		collectRawBlocks(child, source, &rawBlocks, &headingStack)
	}

	if len(rawBlocks) == 0 {
		return nil, nil
	}

	// Second pass: adjust ranges so that blocks tile the source completely.
	// Each block's End extends to the next block's Start.
	// The last block's End extends to len(source).
	blocks := make([]model.Block, len(rawBlocks))
	for i, rb := range rawBlocks {
		var end int
		if i+1 < len(rawBlocks) {
			end = rawBlocks[i+1].start
		} else {
			end = len(source)
		}
		blocks[i] = model.Block{
			Kind:         rb.kind,
			Text:         rb.text,
			HeadingLevel: rb.headingLevel,
			HeadingPath:  rb.headingPath,
			Range:        model.SourceRange{Start: rb.start, End: end},
		}
	}

	return blocks, nil
}

type headingEntry struct {
	level int
	title string
}

type rawBlock struct {
	kind         model.BlockKind
	text         string
	headingLevel int
	headingPath  []string
	start        int
	end          int
}

func collectRawBlocks(node ast.Node, source []byte, blocks *[]rawBlock, headingStack *[]headingEntry) {
	kind, ok := classifyNode(node)
	if !ok {
		return
	}

	start, end := nodeByteRange(node, source)

	var headingLevel int
	var headingTitle string
	if kind == model.BlockHeading {
		if h, ok := node.(*ast.Heading); ok {
			headingLevel = h.Level
			headingTitle = collectNodeText(h, source)
		}
	}

	// Update heading stack for headings
	if kind == model.BlockHeading {
		for len(*headingStack) > 0 && (*headingStack)[len(*headingStack)-1].level >= headingLevel {
			*headingStack = (*headingStack)[:len(*headingStack)-1]
		}
		*headingStack = append(*headingStack, headingEntry{level: headingLevel, title: headingTitle})
	}

	// Build heading path for this block
	var headingPath []string
	if kind == model.BlockHeading {
		if len(*headingStack) > 1 {
			headingPath = make([]string, len(*headingStack)-1)
			for i := 0; i < len(*headingStack)-1; i++ {
				headingPath[i] = (*headingStack)[i].title
			}
		}
	} else {
		if len(*headingStack) > 0 {
			headingPath = make([]string, len(*headingStack))
			for i, h := range *headingStack {
				headingPath[i] = h.title
			}
		}
	}

	blockText := extractText(node, source)

	*blocks = append(*blocks, rawBlock{
		kind:         kind,
		text:         blockText,
		headingLevel: headingLevel,
		headingPath:  headingPath,
		start:        start,
		end:          end,
	})
}

func classifyNode(node ast.Node) (model.BlockKind, bool) {
	switch node.(type) {
	case *ast.Heading:
		return model.BlockHeading, true
	case *ast.Paragraph:
		return model.BlockParagraph, true
	case *ast.List:
		return model.BlockList, true
	case *ast.FencedCodeBlock:
		return model.BlockCodeBlock, true
	case *ast.CodeBlock:
		return model.BlockCodeBlock, true
	case *ast.ThematicBreak:
		return model.BlockThematicBreak, true
	case *ast.Blockquote:
		return model.BlockBlockquote, true
	case *ast.HTMLBlock:
		return model.BlockHTMLBlock, true
	default:
		kindStr := node.Kind().String()
		if kindStr == "Table" {
			return model.BlockTable, true
		}
		if kindStr == "MathBLock" {
			return model.BlockMathBlock, true
		}
		return 0, false
	}
}

// nodeByteRange computes the raw byte range [start, end) for a node.
// This is the "tight" range covering only the node's content lines.
func nodeByteRange(node ast.Node, source []byte) (int, int) {
	minStart := len(source)
	maxEnd := 0

	_ = ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if n.Type() == ast.TypeBlock {
			lines := n.Lines()
			for i := 0; i < lines.Len(); i++ {
				seg := lines.At(i)
				if seg.Start < minStart {
					minStart = seg.Start
				}
				if seg.Stop > maxEnd {
					maxEnd = seg.Stop
				}
			}
			if hb, ok := n.(*ast.HTMLBlock); ok && hb.HasClosure() {
				cl := hb.ClosureLine
				if cl.Start < minStart {
					minStart = cl.Start
				}
				if cl.Stop > maxEnd {
					maxEnd = cl.Stop
				}
			}
		}
		if tn, ok := n.(*ast.Text); ok {
			seg := tn.Segment
			if seg.Start < minStart {
				minStart = seg.Start
			}
			if seg.Stop > maxEnd {
				maxEnd = seg.Stop
			}
		}
		return ast.WalkContinue, nil
	})

	// For heading nodes: include the ATX marker (# prefix)
	if _, ok := node.(*ast.Heading); ok && minStart < len(source) {
		for minStart > 0 && source[minStart-1] != '\n' {
			minStart--
		}
	}

	// For list nodes: include the list marker (-, *, 1., etc.)
	if _, ok := node.(*ast.List); ok && minStart < len(source) {
		for minStart > 0 && source[minStart-1] != '\n' {
			minStart--
		}
	}

	// For ThematicBreak: no lines, but we can find it by looking at prev/next siblings
	if _, ok := node.(*ast.ThematicBreak); ok && minStart > maxEnd {
		// Find position by looking at adjacent nodes
		start, end := findThematicBreakRange(node, source)
		return start, end
	}

	// Extend end to include trailing newline
	if maxEnd < len(source) && source[maxEnd] == '\n' {
		maxEnd++
	}

	if minStart > maxEnd {
		return 0, 0
	}

	return minStart, maxEnd
}

// findThematicBreakRange finds the byte range of a thematic break node
// by scanning source from a known reference point.
func findThematicBreakRange(node ast.Node, source []byte) (int, int) {
	// Use the previous sibling's end or the next sibling's start as reference
	var searchStart int

	if prev := node.PreviousSibling(); prev != nil {
		_, prevEnd := nodeByteRangeSimple(prev, source)
		searchStart = prevEnd
	}

	// Scan forward from searchStart for a thematic break line (---, ***, ___)
	pos := searchStart
	for pos < len(source) {
		// Skip blank lines
		if source[pos] == '\n' {
			pos++
			continue
		}
		// Check if this line is a thematic break
		lineStart := pos
		lineEnd := pos
		for lineEnd < len(source) && source[lineEnd] != '\n' {
			lineEnd++
		}
		if lineEnd < len(source) {
			lineEnd++ // include newline
		}
		line := source[lineStart:lineEnd]
		if isThematicBreakLine(line) {
			return lineStart, lineEnd
		}
		pos = lineEnd
	}

	return 0, 0
}

func isThematicBreakLine(line []byte) bool {
	// A thematic break is a line of 3+ -, *, or _ (possibly with spaces)
	ch := byte(0)
	count := 0
	for _, b := range line {
		if b == ' ' || b == '\t' || b == '\n' || b == '\r' {
			continue
		}
		if b == '-' || b == '*' || b == '_' {
			if ch == 0 {
				ch = b
			}
			if b == ch {
				count++
			} else {
				return false
			}
		} else {
			return false
		}
	}
	return count >= 3
}

// nodeByteRangeSimple returns a quick estimate of a node's byte range
// without recursive calls that might cause infinite loops.
func nodeByteRangeSimple(node ast.Node, source []byte) (int, int) {
	minStart := len(source)
	maxEnd := 0

	lines := node.Lines()
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		if seg.Start < minStart {
			minStart = seg.Start
		}
		if seg.Stop > maxEnd {
			maxEnd = seg.Stop
		}
	}

	// Check inline children
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		if tn, ok := child.(*ast.Text); ok {
			seg := tn.Segment
			if seg.Start < minStart {
				minStart = seg.Start
			}
			if seg.Stop > maxEnd {
				maxEnd = seg.Stop
			}
		}
	}

	// Extend end to include trailing newline
	if maxEnd > 0 && maxEnd < len(source) && source[maxEnd] == '\n' {
		maxEnd++
	}

	if minStart > maxEnd {
		return 0, 0
	}
	return minStart, maxEnd
}

func extractText(node ast.Node, source []byte) string {
	var buf []byte
	lines := node.Lines()
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		buf = append(buf, seg.Value(source)...)
	}
	if len(buf) > 0 {
		return string(buf)
	}
	return collectNodeText(node, source)
}

// collectNodeText collects text from a node's inline children, replacing
// the deprecated node.Text(source) API.
func collectNodeText(node ast.Node, source []byte) string {
	var buf []byte
	_ = ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if tn, ok := n.(*ast.Text); ok {
			buf = append(buf, tn.Value(source)...)
		}
		return ast.WalkContinue, nil
	})
	return string(buf)
}
