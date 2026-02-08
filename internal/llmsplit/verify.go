package llmsplit

import (
	"fmt"

	"github.com/thirdlf03/kire/internal/model"
)

// VerifyLossless checks that segments tile the source with no gaps or overlaps.
func VerifyLossless(segments []model.Segment, sourceLen int) error {
	if len(segments) == 0 {
		if sourceLen == 0 {
			return nil
		}
		return fmt.Errorf("no segments for source of length %d", sourceLen)
	}
	if segments[0].Range.Start != 0 {
		return fmt.Errorf("first segment starts at %d, expected 0", segments[0].Range.Start)
	}
	for i := 1; i < len(segments); i++ {
		if segments[i].Range.Start != segments[i-1].Range.End {
			return fmt.Errorf("gap between segment %d (end=%d) and %d (start=%d)",
				i-1, segments[i-1].Range.End, i, segments[i].Range.Start)
		}
	}
	last := segments[len(segments)-1]
	if last.Range.End != sourceLen {
		return fmt.Errorf("last segment ends at %d, expected %d", last.Range.End, sourceLen)
	}
	return nil
}
