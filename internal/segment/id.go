package segment

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/thirdlf03/kire/internal/model"
)

// GenerateID produces a content-addressable SHA256 ID for a segment.
// The ID is derived from the source filename, byte range, and content.
func GenerateID(sourceName string, seg model.Segment, source []byte) string {
	h := sha256.New()
	h.Write([]byte(sourceName))
	_, _ = fmt.Fprintf(h, ":%d:%d:", seg.Range.Start, seg.Range.End)
	if seg.Range.Start < seg.Range.End && seg.Range.End <= len(source) {
		h.Write(source[seg.Range.Start:seg.Range.End])
	}
	return hex.EncodeToString(h.Sum(nil))
}

// AssignIDs generates IDs for all segments and sets prev/next links.
func AssignIDs(sourceName string, segments []model.Segment, source []byte) {
	if len(segments) == 0 {
		return
	}

	for i := range segments {
		segments[i].ID = GenerateID(sourceName, segments[i], source)
	}

	for i := range segments {
		if i > 0 {
			segments[i].PrevSegmentID = segments[i-1].ID
		}
		if i < len(segments)-1 {
			segments[i].NextSegmentID = segments[i+1].ID
		}
	}
}
