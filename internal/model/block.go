package model

// BlockKind represents the type of a markdown block.
type BlockKind int

const (
	BlockHeading BlockKind = iota
	BlockParagraph
	BlockList
	BlockCodeBlock
	BlockThematicBreak
	BlockTable
	BlockBlockquote
	BlockHTMLBlock
	BlockMathBlock
)

func (k BlockKind) String() string {
	switch k {
	case BlockHeading:
		return "heading"
	case BlockParagraph:
		return "paragraph"
	case BlockList:
		return "list"
	case BlockCodeBlock:
		return "codeblock"
	case BlockThematicBreak:
		return "thematic_break"
	case BlockTable:
		return "table"
	case BlockBlockquote:
		return "blockquote"
	case BlockHTMLBlock:
		return "html_block"
	case BlockMathBlock:
		return "math_block"
	default:
		return "unknown"
	}
}

// SourceRange represents a byte range in the original source.
type SourceRange struct {
	Start int
	End   int
}

// BlockStartIndex builds a map from each block's Range.Start to its index.
// Useful for mapping segment blocks back to the global block list.
func BlockStartIndex(blocks []Block) map[int]int {
	m := make(map[int]int, len(blocks))
	for i, b := range blocks {
		m[b.Range.Start] = i
	}
	return m
}

// Block represents a single markdown block element.
type Block struct {
	Kind            BlockKind
	Text            string
	HeadingLevel    int
	HeadingPath     []string
	Range           SourceRange
	TokensApprox    int
	LinesApprox     int
	IsPseudoHeading bool
}
