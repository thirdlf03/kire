package segment

import (
	"testing"

	"github.com/thirdlf03/kire/internal/model"
)

func TestGenerateID(t *testing.T) {
	source := []byte("hello world this is test content")
	seg := model.Segment{
		Range: model.SourceRange{Start: 0, End: 11},
	}

	id := GenerateID("test.md", seg, source)

	if id == "" {
		t.Fatal("expected non-empty ID")
	}
	if len(id) != 64 { // SHA256 hex length
		t.Fatalf("expected 64 char hex, got %d chars: %s", len(id), id)
	}

	// Same input → same ID
	id2 := GenerateID("test.md", seg, source)
	if id != id2 {
		t.Fatalf("expected deterministic ID, got %s != %s", id, id2)
	}

	// Different source name → different ID
	id3 := GenerateID("other.md", seg, source)
	if id == id3 {
		t.Fatal("expected different ID for different source name")
	}

	// Different byte range → different ID
	seg2 := model.Segment{
		Range: model.SourceRange{Start: 6, End: 11},
	}
	id4 := GenerateID("test.md", seg2, source)
	if id == id4 {
		t.Fatal("expected different ID for different byte range")
	}
}

func TestAssignIDs(t *testing.T) {
	source := []byte("aaa bbb ccc ddd")
	segments := []model.Segment{
		{Range: model.SourceRange{Start: 0, End: 3}},
		{Range: model.SourceRange{Start: 4, End: 7}},
		{Range: model.SourceRange{Start: 8, End: 11}},
	}

	AssignIDs("test.md", segments, source)

	// All IDs should be non-empty and unique
	ids := make(map[string]bool)
	for i, seg := range segments {
		if seg.ID == "" {
			t.Fatalf("segment %d has empty ID", i)
		}
		if ids[seg.ID] {
			t.Fatalf("segment %d has duplicate ID: %s", i, seg.ID)
		}
		ids[seg.ID] = true
	}

	// Check prev/next links
	if segments[0].PrevSegmentID != "" {
		t.Fatalf("first segment should have empty PrevSegmentID, got %s", segments[0].PrevSegmentID)
	}
	if segments[0].NextSegmentID != segments[1].ID {
		t.Fatalf("first segment NextSegmentID mismatch: %s != %s", segments[0].NextSegmentID, segments[1].ID)
	}
	if segments[1].PrevSegmentID != segments[0].ID {
		t.Fatalf("middle segment PrevSegmentID mismatch")
	}
	if segments[1].NextSegmentID != segments[2].ID {
		t.Fatalf("middle segment NextSegmentID mismatch")
	}
	if segments[2].PrevSegmentID != segments[1].ID {
		t.Fatalf("last segment PrevSegmentID mismatch")
	}
	if segments[2].NextSegmentID != "" {
		t.Fatalf("last segment should have empty NextSegmentID, got %s", segments[2].NextSegmentID)
	}
}

func TestAssignIDsSingleSegment(t *testing.T) {
	source := []byte("only segment")
	segments := []model.Segment{
		{Range: model.SourceRange{Start: 0, End: 12}},
	}

	AssignIDs("test.md", segments, source)

	if segments[0].ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if segments[0].PrevSegmentID != "" {
		t.Fatal("single segment should have no prev")
	}
	if segments[0].NextSegmentID != "" {
		t.Fatal("single segment should have no next")
	}
}

func TestAssignIDsEmpty(t *testing.T) {
	AssignIDs("test.md", nil, nil) // should not panic
	AssignIDs("test.md", []model.Segment{}, nil)
}
