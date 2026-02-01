package pipeline

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"

	"github.com/thirdlf03/kire/internal/model"
)

// IncrementalState stores the state of a previous pipeline run for change detection.
type IncrementalState struct {
	SourceHash string            `json:"source_hash"`
	Segments   []SegmentSnapshot `json:"segments"`
}

// SegmentSnapshot captures the identity and content hash of a segment.
type SegmentSnapshot struct {
	ID        string `json:"id"`
	ByteRange [2]int `json:"byte_range"`
	Hash      string `json:"hash"`
}

// ChangeSet describes what changed between two pipeline runs.
type ChangeSet struct {
	Added     []string // Segment IDs that are new
	Removed   []string // Segment IDs that no longer exist
	Modified  []string // Segment IDs whose content changed
	Unchanged []string // Segment IDs that are identical
}

// SourceHash computes a SHA256 hash of the source content.
func SourceHash(source []byte) string {
	h := sha256.Sum256(source)
	return hex.EncodeToString(h[:])
}

// contentHash computes a SHA256 hash of a content slice.
func contentHash(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// LoadState loads a previously saved incremental state from a file.
// Returns nil, nil if the file does not exist.
func LoadState(path string) (*IncrementalState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var state IncrementalState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

// SaveState saves the current incremental state to a file.
func SaveState(path string, state *IncrementalState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// BuildState creates an IncrementalState from the current pipeline output.
func BuildState(source []byte, segments []model.Segment) *IncrementalState {
	snapshots := make([]SegmentSnapshot, len(segments))
	for i, seg := range segments {
		content := []byte{}
		if seg.Range.Start < seg.Range.End && seg.Range.End <= len(source) {
			content = source[seg.Range.Start:seg.Range.End]
		}
		snapshots[i] = SegmentSnapshot{
			ID:        seg.ID,
			ByteRange: [2]int{seg.Range.Start, seg.Range.End},
			Hash:      contentHash(content),
		}
	}
	return &IncrementalState{
		SourceHash: SourceHash(source),
		Segments:   snapshots,
	}
}

// DetectChanges compares the previous state with current segments and returns a ChangeSet.
// If prev is nil, all current segments are considered Added.
func DetectChanges(prev *IncrementalState, current []model.Segment, source []byte) ChangeSet {
	if prev == nil {
		cs := ChangeSet{}
		for _, seg := range current {
			cs.Added = append(cs.Added, seg.ID)
		}
		return cs
	}

	// Build maps for comparison
	prevByID := make(map[string]SegmentSnapshot, len(prev.Segments))
	for _, s := range prev.Segments {
		prevByID[s.ID] = s
	}

	currentByID := make(map[string]string, len(current)) // ID → content hash
	for _, seg := range current {
		content := []byte{}
		if seg.Range.Start < seg.Range.End && seg.Range.End <= len(source) {
			content = source[seg.Range.Start:seg.Range.End]
		}
		currentByID[seg.ID] = contentHash(content)
	}

	var cs ChangeSet

	// Check current segments against previous
	for _, seg := range current {
		prevSnap, exists := prevByID[seg.ID]
		if !exists {
			cs.Added = append(cs.Added, seg.ID)
		} else if currentByID[seg.ID] != prevSnap.Hash {
			cs.Modified = append(cs.Modified, seg.ID)
		} else {
			cs.Unchanged = append(cs.Unchanged, seg.ID)
		}
	}

	// Check for removed segments
	for _, s := range prev.Segments {
		if _, exists := currentByID[s.ID]; !exists {
			cs.Removed = append(cs.Removed, s.ID)
		}
	}

	return cs
}
