package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/thirdlf03/kire/internal/model"
)

func TestAverageEmbedding_Basic(t *testing.T) {
	embeddings := []model.Embedding{
		{BlockIndex: 0, Vector: []float64{1.0, 0.0, 2.0}},
		{BlockIndex: 1, Vector: []float64{3.0, 4.0, 0.0}},
	}
	got := AverageEmbedding([]int{0, 1}, embeddings)
	want := []float64{2.0, 2.0, 1.0}
	if len(got) != len(want) {
		t.Fatalf("len: got %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("index %d: got %f, want %f", i, got[i], want[i])
		}
	}
}

func TestAverageEmbedding_Empty(t *testing.T) {
	embeddings := []model.Embedding{
		{BlockIndex: 0, Vector: []float64{1.0, 2.0}},
	}
	got := AverageEmbedding(nil, embeddings)
	if got != nil {
		t.Errorf("expected nil for empty indices, got %v", got)
	}
}

func TestSegmentBlockIndices(t *testing.T) {
	allBlocks := []model.Block{
		{Text: "a", Range: model.SourceRange{Start: 0, End: 10}},
		{Text: "b", Range: model.SourceRange{Start: 10, End: 20}},
		{Text: "c", Range: model.SourceRange{Start: 20, End: 30}},
		{Text: "d", Range: model.SourceRange{Start: 30, End: 40}},
	}
	segments := []model.Segment{
		{Blocks: allBlocks[0:2]},
		{Blocks: allBlocks[2:4]},
	}

	result := SegmentBlockIndices(segments, allBlocks)
	if len(result) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(result))
	}
	if len(result[0]) != 2 || result[0][0] != 0 || result[0][1] != 1 {
		t.Errorf("group 0: got %v, want [0 1]", result[0])
	}
	if len(result[1]) != 2 || result[1][0] != 2 || result[1][1] != 3 {
		t.Errorf("group 1: got %v, want [2 3]", result[1])
	}
}

func TestWriteJSONL_SingleRecord(t *testing.T) {
	records := []JSONLRecord{
		{
			Content: "hello world",
			Metadata: JSONLMetadata{
				Source:       "test.md",
				SegmentIndex: 0,
				Filename:     "01-hello.md",
				ByteRange:    [2]int{0, 11},
				HeadingPath:  []string{"Introduction"},
				TokenCount:   5,
				BlockCount:   1,
			},
			Embedding: []float64{0.1, 0.2},
		},
	}

	var buf bytes.Buffer
	if err := WriteJSONL(&buf, records); err != nil {
		t.Fatal(err)
	}

	// Should be exactly one line
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}

	// Verify JSON unmarshal
	var got JSONLRecord
	if err := json.Unmarshal([]byte(lines[0]), &got); err != nil {
		t.Fatal(err)
	}
	if got.Content != "hello world" {
		t.Errorf("content: got %q, want %q", got.Content, "hello world")
	}
	if got.Metadata.Source != "test.md" {
		t.Errorf("source: got %q, want %q", got.Metadata.Source, "test.md")
	}
	if len(got.Embedding) != 2 {
		t.Errorf("embedding len: got %d, want 2", len(got.Embedding))
	}
}

func TestWriteJSONL_AgentMetadata(t *testing.T) {
	coh := 0.85
	conf := 0.72
	records := []JSONLRecord{
		{
			Content: "test content",
			Metadata: JSONLMetadata{
				Source:             "test.md",
				SegmentIndex:       0,
				Filename:           "01-test.md",
				ByteRange:          [2]int{0, 12},
				HeadingPath:        []string{"Section"},
				TokenCount:         5,
				BlockCount:         1,
				SegmentID:          "abc123",
				PrevSegmentID:      "",
				NextSegmentID:      "def456",
				Coherence:          &coh,
				BoundaryConfidence: &conf,
			},
		},
	}

	var buf bytes.Buffer
	if err := WriteJSONL(&buf, records); err != nil {
		t.Fatal(err)
	}

	line := strings.TrimSpace(buf.String())

	var got JSONLRecord
	if err := json.Unmarshal([]byte(line), &got); err != nil {
		t.Fatal(err)
	}
	if got.Metadata.SegmentID != "abc123" {
		t.Errorf("segment_id: got %q, want %q", got.Metadata.SegmentID, "abc123")
	}
	if got.Metadata.NextSegmentID != "def456" {
		t.Errorf("next_segment_id: got %q, want %q", got.Metadata.NextSegmentID, "def456")
	}
	if got.Metadata.Coherence == nil || *got.Metadata.Coherence != 0.85 {
		t.Errorf("coherence: got %v, want 0.85", got.Metadata.Coherence)
	}
	if got.Metadata.BoundaryConfidence == nil || *got.Metadata.BoundaryConfidence != 0.72 {
		t.Errorf("boundary_confidence: got %v, want 0.72", got.Metadata.BoundaryConfidence)
	}

	// prev_segment_id should be omitted when empty
	if strings.Contains(line, `"prev_segment_id"`) {
		t.Error("empty prev_segment_id should be omitted")
	}
}

func TestWriteJSONL_AgentMetadataOmittedWhenEmpty(t *testing.T) {
	records := []JSONLRecord{
		{
			Content: "no agent metadata",
			Metadata: JSONLMetadata{
				Source:       "test.md",
				SegmentIndex: 0,
				Filename:     "01-test.md",
				ByteRange:    [2]int{0, 17},
				HeadingPath:  []string{},
				TokenCount:   3,
				BlockCount:   1,
			},
		},
	}

	var buf bytes.Buffer
	if err := WriteJSONL(&buf, records); err != nil {
		t.Fatal(err)
	}

	line := strings.TrimSpace(buf.String())
	for _, field := range []string{"segment_id", "coherence", "boundary_confidence"} {
		if strings.Contains(line, `"`+field+`"`) {
			t.Errorf("omitempty field %q should not appear in output", field)
		}
	}
}

func TestWriteJSONL_OmitEmbedding(t *testing.T) {
	records := []JSONLRecord{
		{
			Content: "no embedding",
			Metadata: JSONLMetadata{
				Source:       "test.md",
				SegmentIndex: 0,
				Filename:     "01-test.md",
				ByteRange:    [2]int{0, 12},
				HeadingPath:  []string{},
				TokenCount:   3,
				BlockCount:   1,
			},
			Embedding: nil,
		},
	}

	var buf bytes.Buffer
	if err := WriteJSONL(&buf, records); err != nil {
		t.Fatal(err)
	}

	line := strings.TrimSpace(buf.String())
	if strings.Contains(line, `"embedding"`) {
		t.Error("nil embedding should be omitted from JSON output")
	}
}
