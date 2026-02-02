package dag

import (
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/thirdlf03/kire/internal/model"
)

// Edge represents a dependency between segments.
type Edge struct {
	From int    `json:"from"`
	To   int    `json:"to"`
	Type string `json:"type"` // "anchor", "link"
}

// DAG represents a dependency graph between segments.
type DAG struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

// Node represents a segment in the DAG.
type Node struct {
	ID      int      `json:"id"`
	Label   string   `json:"label"`
	Anchors []string `json:"anchors,omitempty"`
}

var (
	anchorRe = regexp.MustCompile(`(?i)<a\s+(?:name|id)\s*=\s*"([^"]+)"`)
	linkRe   = regexp.MustCompile(`\[([^\]]*)\]\(#([^)]+)\)`)
	headerRe = regexp.MustCompile(`(?m)^#{1,6}\s+(.+)$`)
)

// Build constructs a DAG from segments by detecting cross-references.
func Build(segments []model.Segment, source []byte) DAG {
	nodes := make([]Node, len(segments))

	// Collect anchors per segment
	segAnchors := make([]map[string]bool, len(segments))
	for i, seg := range segments {
		if seg.Range.Start < 0 || seg.Range.End > len(source) || seg.Range.Start > seg.Range.End {
			nodes[i] = Node{ID: i}
			continue
		}
		text := string(source[seg.Range.Start:seg.Range.End])
		label := extractLabel(seg, source)
		anchors := extractAnchors(text)
		nodes[i] = Node{
			ID:      i,
			Label:   label,
			Anchors: slices.Sorted(maps.Keys(anchors)),
		}
		segAnchors[i] = anchors
	}

	// Build anchor-to-segment index
	anchorIndex := make(map[string]int)
	for i, anchors := range segAnchors {
		for a := range anchors {
			anchorIndex[a] = i
		}
	}

	// Detect cross-references
	edges := make([]Edge, 0)
	for i, seg := range segments {
		if seg.Range.Start < 0 || seg.Range.End > len(source) || seg.Range.Start > seg.Range.End {
			continue
		}
		text := string(source[seg.Range.Start:seg.Range.End])
		refs := extractLinks(text)
		for _, ref := range refs {
			if targetSeg, ok := anchorIndex[ref]; ok && targetSeg != i {
				edges = append(edges, Edge{From: i, To: targetSeg, Type: "link"})
			}
		}
	}

	return DAG{Nodes: nodes, Edges: edges}
}

func extractLabel(seg model.Segment, source []byte) string {
	var baseLabel string

	// Use the first heading if available
	for _, b := range seg.Blocks {
		if b.Kind == model.BlockHeading {
			baseLabel = b.Text
			break
		}
	}

	if baseLabel == "" {
		// Fallback: first non-empty line
		text := string(source[seg.Range.Start:seg.Range.End])
		baseLabel = firstNonEmptyLine(text)
	}

	// Prepend the last element of HeadingPath as parent context,
	// if it differs from the base label itself.
	if len(seg.HeadingPath) > 0 {
		parent := seg.HeadingPath[len(seg.HeadingPath)-1]
		if parent != baseLabel {
			return normalizeLabel(parent + " > " + baseLabel)
		}
	}

	return normalizeLabel(baseLabel)
}

func firstNonEmptyLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) != "" {
			return line
		}
	}
	return strings.TrimSpace(text)
}

func normalizeLabel(label string) string {
	label = strings.TrimSpace(label)
	if label == "" {
		return label
	}
	// Collapse whitespace/newlines to keep the Markdown table intact.
	label = strings.Join(strings.Fields(label), " ")
	label = strings.ReplaceAll(label, "|", "\\|")
	const maxRunes = 60
	if utf8.RuneCountInString(label) > maxRunes {
		label = truncateRunes(label, maxRunes) + "..."
	}
	return label
}

func truncateRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	count := 0
	for i := range s {
		if count == n {
			return s[:i]
		}
		count++
	}
	return s
}

func extractAnchors(text string) map[string]bool {
	anchors := make(map[string]bool)

	// HTML anchors: <a name="xxx"> or <a id="xxx">
	for _, m := range anchorRe.FindAllStringSubmatch(text, -1) {
		anchors[m[1]] = true
	}

	// Auto-generated heading IDs (GitHub-style: lowercase, hyphens)
	for _, m := range headerRe.FindAllStringSubmatch(text, -1) {
		id := headingToID(m[1])
		anchors[id] = true
	}

	return anchors
}

func extractLinks(text string) []string {
	var refs []string
	for _, m := range linkRe.FindAllStringSubmatch(text, -1) {
		refs = append(refs, m[2])
	}
	return refs
}

func headingToID(heading string) string {
	heading = strings.ToLower(heading)
	heading = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		if r == ' ' {
			return '-'
		}
		return -1
	}, heading)
	return heading
}

// ExportIndexMarkdown exports a Markdown index listing all segments
// and their dependencies. filenames[i] is the filename for node i.
func (d DAG) ExportIndexMarkdown(filenames []string) string {
	var b strings.Builder
	b.WriteString("# Index\n\n")
	b.WriteString("| # | File | Section |\n")
	b.WriteString("|---|------|--------|\n")
	for _, n := range d.Nodes {
		fname := ""
		if n.ID < len(filenames) {
			fname = filenames[n.ID]
		}
		b.WriteString(fmt.Sprintf("| %d | [%s](%s) | %s |\n", n.ID+1, fname, fname, n.Label))
	}

	// Mermaid DAG graph
	if len(d.Nodes) > 0 {
		b.WriteString("\n## Graph\n\n")
		b.WriteString("```mermaid\ngraph LR\n")

		// Node definitions with short labels (truncated for readability)
		for _, n := range d.Nodes {
			label := mermaidEscape(mermaidShortLabel(n.Label))
			b.WriteString(fmt.Sprintf("  %d[\"%d. %s\"]\n", n.ID, n.ID+1, label))
		}

		// Sequential flow edges
		for i := 0; i < len(d.Nodes)-1; i++ {
			b.WriteString(fmt.Sprintf("  %d --> %d\n", i, i+1))
		}

		// Cross-reference edges (dashed)
		for _, e := range d.Edges {
			b.WriteString(fmt.Sprintf("  %d -.->|%s| %d\n", e.From, e.Type, e.To))
		}

		b.WriteString("```\n")
	}

	return b.String()
}

// mermaidEscape escapes characters that are special in Mermaid node labels.
func mermaidEscape(s string) string {
	s = strings.ReplaceAll(s, `"`, "#quot;")
	s = strings.ReplaceAll(s, "`", "'")
	return s
}

// mermaidShortLabel returns a short label for Mermaid graph nodes.
// For "Parent > Child" style labels, keeps only the last part.
// Truncates to 30 runes for readability.
func mermaidShortLabel(label string) string {
	// Use last segment of "A > B > C" path
	if idx := strings.LastIndex(label, " > "); idx >= 0 {
		label = label[idx+3:]
	}
	const maxRunes = 30
	if utf8.RuneCountInString(label) > maxRunes {
		label = truncateRunes(label, maxRunes) + "…"
	}
	return label
}

// ExportJSON exports the DAG as JSON.
func (d DAG) ExportJSON() ([]byte, error) {
	return json.MarshalIndent(d, "", "  ")
}

// ExportDOT exports the DAG as a Graphviz DOT string.
func (d DAG) ExportDOT() string {
	var b strings.Builder
	b.WriteString("digraph segments {\n")
	b.WriteString("  rankdir=LR;\n")
	for _, n := range d.Nodes {
		label := strings.ReplaceAll(n.Label, `"`, `\"`)
		b.WriteString(fmt.Sprintf("  %d [label=\"%s\"];\n", n.ID, label))
	}
	for _, e := range d.Edges {
		b.WriteString(fmt.Sprintf("  %d -> %d [label=\"%s\"];\n", e.From, e.To, e.Type))
	}
	b.WriteString("}\n")
	return b.String()
}
