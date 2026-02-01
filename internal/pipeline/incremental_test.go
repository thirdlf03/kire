package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/thirdlf03/kire/internal/model"
)

func TestSourceHash(t *testing.T) {
	h1 := SourceHash([]byte("hello"))
	h2 := SourceHash([]byte("hello"))
	h3 := SourceHash([]byte("world"))

	if h1 != h2 {
		t.Error("same content should produce same hash")
	}
	if h1 == h3 {
		t.Error("different content should produce different hash")
	}
	if len(h1) != 64 {
		t.Errorf("expected 64 hex chars, got %d", len(h1))
	}
}

func TestSaveLoadState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	state := &IncrementalState{
		SourceHash: "abc123",
		Segments: []SegmentSnapshot{
			{ID: "seg-1", ByteRange: [2]int{0, 100}, Hash: "hash1"},
			{ID: "seg-2", ByteRange: [2]int{100, 200}, Hash: "hash2"},
		},
	}

	if err := SaveState(path, state); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadState(path)
	if err != nil {
		t.Fatal(err)
	}

	if loaded.SourceHash != "abc123" {
		t.Errorf("SourceHash: got %q, want %q", loaded.SourceHash, "abc123")
	}
	if len(loaded.Segments) != 2 {
		t.Fatalf("segments: got %d, want 2", len(loaded.Segments))
	}
	if loaded.Segments[0].ID != "seg-1" {
		t.Errorf("segment[0].ID: got %q, want %q", loaded.Segments[0].ID, "seg-1")
	}
}

func TestLoadStateNotExist(t *testing.T) {
	loaded, err := LoadState("/nonexistent/path")
	if err != nil {
		t.Fatal(err)
	}
	if loaded != nil {
		t.Error("expected nil for non-existent file")
	}
}

func TestDetectChanges(t *testing.T) {
	prev := &IncrementalState{
		SourceHash: "old_hash",
		Segments: []SegmentSnapshot{
			{ID: "seg-1", ByteRange: [2]int{0, 10}, Hash: "h1"},
			{ID: "seg-2", ByteRange: [2]int{10, 20}, Hash: "h2"},
			{ID: "seg-3", ByteRange: [2]int{20, 30}, Hash: "h3"},
		},
	}

	source := []byte("0123456789" + "ABCDEFGHIJ" + "klmnopqrst" + "NEW_CONTENT")
	current := []model.Segment{
		{ID: "seg-1", Range: model.SourceRange{Start: 0, End: 10}},   // unchanged content
		{ID: "seg-2a", Range: model.SourceRange{Start: 10, End: 20}}, // changed content (new ID)
		{ID: "seg-3", Range: model.SourceRange{Start: 20, End: 30}},  // unchanged content
		{ID: "seg-4", Range: model.SourceRange{Start: 30, End: 41}},  // new segment
	}

	cs := DetectChanges(prev, current, source)

	if len(cs.Added) == 0 {
		t.Error("expected added segments")
	}
	if len(cs.Removed) == 0 {
		t.Error("expected removed segments")
	}
}

func TestDetectChangesNoPrev(t *testing.T) {
	source := []byte("hello")
	current := []model.Segment{
		{ID: "seg-1", Range: model.SourceRange{Start: 0, End: 5}},
	}

	cs := DetectChanges(nil, current, source)
	if len(cs.Added) != 1 {
		t.Errorf("expected 1 added, got %d", len(cs.Added))
	}
	if len(cs.Removed) != 0 {
		t.Errorf("expected 0 removed, got %d", len(cs.Removed))
	}
}

func TestBuildState(t *testing.T) {
	source := []byte("hello world goodbye")
	segments := []model.Segment{
		{ID: "seg-1", Range: model.SourceRange{Start: 0, End: 5}},
		{ID: "seg-2", Range: model.SourceRange{Start: 6, End: 11}},
	}

	state := BuildState(source, segments)
	if state.SourceHash == "" {
		t.Error("SourceHash should not be empty")
	}
	if len(state.Segments) != 2 {
		t.Fatalf("segments: got %d, want 2", len(state.Segments))
	}
	if state.Segments[0].ID != "seg-1" {
		t.Errorf("ID: got %q, want %q", state.Segments[0].ID, "seg-1")
	}

	// Verify state file roundtrip
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	if err := SaveState(path, state); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadState(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.SourceHash != state.SourceHash {
		t.Error("roundtrip hash mismatch")
	}

	// Cleanup
	_ = os.Remove(path)
}
