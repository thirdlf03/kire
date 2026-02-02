package dag_test

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/thirdlf03/kire/internal/dag"
	"github.com/thirdlf03/kire/internal/model"
)

func TestBuild_NoReferences(t *testing.T) {
	source := []byte("# Part 1\n\nContent.\n\n# Part 2\n\nMore content.\n")
	segments := []model.Segment{
		{
			Blocks: []model.Block{{Kind: model.BlockHeading, Text: "Part 1"}},
			Range:  model.SourceRange{Start: 0, End: 20},
		},
		{
			Blocks: []model.Block{{Kind: model.BlockHeading, Text: "Part 2"}},
			Range:  model.SourceRange{Start: 20, End: 44},
		},
	}

	d := dag.Build(segments, source)
	if len(d.Nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(d.Nodes))
	}
	if len(d.Edges) != 0 {
		t.Errorf("expected 0 edges, got %d", len(d.Edges))
	}
	if d.Edges == nil {
		t.Error("edges should be empty slice, not nil")
	}

	// JSON should serialize as [] not null
	data, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"edges":null`) {
		t.Errorf("edges should be [] in JSON, got null: %s", string(data))
	}
	if !strings.Contains(string(data), `"edges":[]`) {
		t.Errorf("expected edges:[] in JSON, got: %s", string(data))
	}
}

func TestBuild_CrossReference(t *testing.T) {
	source := []byte("# Part 1\n\nSee [Part 2](#part-2).\n\n# Part 2\n\nContent here.\n")
	segments := []model.Segment{
		{
			Blocks: []model.Block{
				{Kind: model.BlockHeading, Text: "Part 1"},
				{Kind: model.BlockParagraph, Text: "See [Part 2](#part-2)."},
			},
			Range: model.SourceRange{Start: 0, End: 34},
		},
		{
			Blocks: []model.Block{
				{Kind: model.BlockHeading, Text: "Part 2"},
				{Kind: model.BlockParagraph, Text: "Content here."},
			},
			Range: model.SourceRange{Start: 34, End: len(source)},
		},
	}

	d := dag.Build(segments, source)
	if len(d.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(d.Edges))
	}
	if d.Edges[0].From != 0 || d.Edges[0].To != 1 {
		t.Errorf("expected edge 0→1, got %d→%d", d.Edges[0].From, d.Edges[0].To)
	}
}

func TestExportJSON(t *testing.T) {
	d := dag.DAG{
		Nodes: []dag.Node{{ID: 0, Label: "Test"}},
		Edges: []dag.Edge{{From: 0, To: 1, Type: "link"}},
	}
	data, err := d.ExportJSON()
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	if _, ok := parsed["nodes"]; !ok {
		t.Error("expected 'nodes' in JSON")
	}
}

func TestExportIndexMarkdown(t *testing.T) {
	d := dag.DAG{
		Nodes: []dag.Node{
			{ID: 0, Label: "Task管理API 仕様書"},
			{ID: 1, Label: "バリデーション"},
		},
		Edges: []dag.Edge{{From: 0, To: 1, Type: "link"}},
	}
	filenames := []string{"split1.md", "split2.md"}
	got := d.ExportIndexMarkdown(filenames)

	// Header
	if !strings.Contains(got, "# Index") {
		t.Error("expected '# Index' header")
	}

	// Table rows
	if !strings.Contains(got, "| 1 | [split1.md](split1.md) | Task管理API 仕様書 |") {
		t.Errorf("expected row for node 0, got:\n%s", got)
	}
	if !strings.Contains(got, "| 2 | [split2.md](split2.md) | バリデーション |") {
		t.Errorf("expected row for node 1, got:\n%s", got)
	}

	// Mermaid graph
	if !strings.Contains(got, "```mermaid") {
		t.Error("expected mermaid code block")
	}
	if !strings.Contains(got, "graph LR") {
		t.Error("expected 'graph LR' in mermaid")
	}
	// Node definitions (short label: last part of path, truncated)
	if !strings.Contains(got, `0["1. Task管理API 仕様書"]`) {
		t.Errorf("expected mermaid node 0, got:\n%s", got)
	}
	// Edge
	if !strings.Contains(got, "0 --> 1") {
		t.Errorf("expected mermaid edge 0-->1, got:\n%s", got)
	}
}

func TestExportIndexMarkdown_NoDeps(t *testing.T) {
	d := dag.DAG{
		Nodes: []dag.Node{
			{ID: 0, Label: "Section A"},
			{ID: 1, Label: "Section B"},
		},
		Edges: nil,
	}
	filenames := []string{"part1.md", "part2.md"}
	got := d.ExportIndexMarkdown(filenames)

	// Table should exist
	if !strings.Contains(got, "| 1 | [part1.md](part1.md) | Section A |") {
		t.Errorf("expected row for node 0, got:\n%s", got)
	}
	if !strings.Contains(got, "| 2 | [part2.md](part2.md) | Section B |") {
		t.Errorf("expected row for node 1, got:\n%s", got)
	}

	// Mermaid graph should still exist (nodes only, no edges)
	if !strings.Contains(got, "```mermaid") {
		t.Error("expected mermaid code block even without edges")
	}
	// Sequential flow edges (0 --> 1) should be present
	if !strings.Contains(got, "0 --> 1") {
		t.Errorf("expected sequential edge in mermaid, got:\n%s", got)
	}
}

func TestExportDOT(t *testing.T) {
	d := dag.DAG{
		Nodes: []dag.Node{{ID: 0, Label: "Node A"}, {ID: 1, Label: "Node B"}},
		Edges: []dag.Edge{{From: 0, To: 1, Type: "link"}},
	}
	dot := d.ExportDOT()
	if !strings.Contains(dot, "digraph") {
		t.Error("expected 'digraph' in DOT output")
	}
	if !strings.Contains(dot, "0 -> 1") {
		t.Error("expected edge '0 -> 1' in DOT output")
	}
}

func TestExtractLabel_HeadingPathContext(t *testing.T) {
	// When a segment has HeadingPath context, the label should include it
	source := []byte("## GET /v1/tasks\n\nReturns tasks.\n")
	segments := []model.Segment{
		{
			Blocks: []model.Block{
				{Kind: model.BlockHeading, Text: "GET /v1/tasks", HeadingLevel: 2},
				{Kind: model.BlockParagraph, Text: "Returns tasks."},
			},
			HeadingPath: []string{"Task管理API", "機能要件"},
			Range:       model.SourceRange{Start: 0, End: len(source)},
		},
	}

	d := dag.Build(segments, source)
	label := d.Nodes[0].Label
	// Should include the deepest ancestor that differs from the heading itself
	if !strings.Contains(label, "機能要件") {
		t.Errorf("expected label to include parent context '機能要件', got %q", label)
	}
	if !strings.Contains(label, "GET /v1/tasks") {
		t.Errorf("expected label to include heading text, got %q", label)
	}
}

func TestExtractLabel_HeadingPathEmpty(t *testing.T) {
	// Top-level segment: no HeadingPath, should just use heading text
	source := []byte("# Top\n\nContent.\n")
	segments := []model.Segment{
		{
			Blocks: []model.Block{
				{Kind: model.BlockHeading, Text: "Top", HeadingLevel: 1},
				{Kind: model.BlockParagraph, Text: "Content."},
			},
			HeadingPath: nil,
			Range:       model.SourceRange{Start: 0, End: len(source)},
		},
	}

	d := dag.Build(segments, source)
	if d.Nodes[0].Label != "Top" {
		t.Errorf("expected 'Top', got %q", d.Nodes[0].Label)
	}
}

func TestExtractLabel_FallbackWithHeadingPath(t *testing.T) {
	// Fallback (no heading block) but HeadingPath is set
	source := []byte("- item 1\n- item 2\n")
	segments := []model.Segment{
		{
			Blocks: []model.Block{
				{Kind: model.BlockList, Text: "item 1\nitem 2"},
			},
			HeadingPath: []string{"Parent", "Section"},
			Range:       model.SourceRange{Start: 0, End: len(source)},
		},
	}

	d := dag.Build(segments, source)
	label := d.Nodes[0].Label
	if !strings.Contains(label, "Section") {
		t.Errorf("expected label to include HeadingPath context 'Section', got %q", label)
	}
}

func TestBuild_OutOfBoundsRange(t *testing.T) {
	source := []byte("short")
	segments := []model.Segment{
		{
			Blocks: []model.Block{{Kind: model.BlockParagraph, Text: "ok"}},
			Range:  model.SourceRange{Start: 0, End: 100}, // End > len(source)
		},
	}
	// Should not panic
	d := dag.Build(segments, source)
	if len(d.Nodes) != 1 {
		t.Errorf("expected 1 node, got %d", len(d.Nodes))
	}
}

func TestExtractLabel_FallbackUnicodeLine(t *testing.T) {
	line := "a" + strings.Repeat("私", 20) // 61 bytes; old byte slicing would corrupt UTF-8
	source := []byte(line + "\n\nSecond line.\n")
	segments := []model.Segment{
		{
			Blocks: []model.Block{
				{Kind: model.BlockParagraph, Text: "dummy", Range: model.SourceRange{Start: 0, End: len(source)}},
			},
			Range: model.SourceRange{Start: 0, End: len(source)},
		},
	}

	d := dag.Build(segments, source)
	if len(d.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(d.Nodes))
	}
	label := d.Nodes[0].Label
	if label != line {
		t.Fatalf("expected label %q, got %q", line, label)
	}
	if strings.Contains(label, "\n") {
		t.Fatalf("label should be single line, got %q", label)
	}
	if !utf8.ValidString(label) {
		t.Fatalf("label should be valid UTF-8, got %q", label)
	}
}
