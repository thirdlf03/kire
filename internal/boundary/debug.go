package boundary

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/thirdlf03/kire/internal/model"
)

// WriteDebugTSV writes boundary scoring details as a TSV table.
// previewLen limits the text preview column length (0 = no limit).
func WriteDebugTSV(w io.Writer, blocks []model.Block, result BoundaryResult, previewLen int) error {
	boundarySet := make(map[int]bool, len(result.Boundaries))
	for _, b := range result.Boundaries {
		boundarySet[b] = true
	}

	// Header
	if _, err := fmt.Fprintln(w, "block_index\tkind\ttext_preview\tsimilarity\tdepth_score\tboundary"); err != nil {
		return err
	}

	for i, b := range blocks {
		preview := strings.TrimSpace(b.Text)
		if previewLen > 0 && utf8.RuneCountInString(preview) > previewLen {
			runes := []rune(preview)
			preview = string(runes[:previewLen]) + "..."
		}
		// Replace tabs/newlines in preview
		preview = strings.ReplaceAll(preview, "\t", " ")
		preview = strings.ReplaceAll(preview, "\n", " ")

		sim := ""
		if i < len(result.Similarities) {
			sim = fmt.Sprintf("%.4f", result.Similarities[i])
		}
		depth := ""
		if i < len(result.DepthScores) {
			depth = fmt.Sprintf("%.4f", result.DepthScores[i])
		}
		marker := ""
		if boundarySet[i] {
			marker = "***"
		}

		if _, err := fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\n",
			i, b.Kind, preview, sim, depth, marker); err != nil {
			return err
		}
	}
	return nil
}
