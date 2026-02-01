package output

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/thirdlf03/kire/internal/ctxheader"
	"github.com/thirdlf03/kire/internal/model"
)

// RenderConfig configures segment rendering.
type RenderConfig struct {
	ContextFormat          string
	ContextMaxDepth        int
	OverlapBlocks          []model.Block
	ContextExcludePatterns []string
}

// Render produces the output string for a segment.
// Order: context → overlap → body (from source using SourceRange).
func Render(seg model.Segment, source []byte, config RenderConfig) string {
	var b strings.Builder

	// Context header
	if config.ContextFormat != "" && config.ContextFormat != "none" {
		ctx := ctxheader.InjectFiltered(seg.HeadingPath, config.ContextFormat, config.ContextMaxDepth, config.ContextExcludePatterns)
		b.WriteString(ctx)
	}

	// Overlap blocks
	if len(config.OverlapBlocks) > 0 {
		for _, ob := range config.OverlapBlocks {
			if ob.Range.Start >= 0 && ob.Range.End <= len(source) && ob.Range.Start <= ob.Range.End {
				b.Write(source[ob.Range.Start:ob.Range.End])
			}
		}
	}

	// Body: extract from source using segment's first block start to last block end
	if len(seg.Blocks) > 0 {
		start := seg.Blocks[0].Range.Start
		end := seg.Blocks[len(seg.Blocks)-1].Range.End
		if start >= 0 && end <= len(source) && start <= end {
			b.Write(source[start:end])
		}
	}

	return b.String()
}

// ComputeOverlapBlocks selects blocks from the end of a segment to provide
// as overlap context for the next segment. Selects blocks from the end
// that cover at least overlapLines lines.
func ComputeOverlapBlocks(blocks []model.Block, source []byte, overlapLines int) []model.Block {
	if overlapLines <= 0 || len(blocks) == 0 {
		return nil
	}

	lineCount := 0
	startIdx := len(blocks)
	for i := len(blocks) - 1; i >= 0; i-- {
		blockText := string(source[blocks[i].Range.Start:blocks[i].Range.End])
		lines := strings.Count(blockText, "\n")
		if lines == 0 {
			lines = 1
		}
		lineCount += lines
		startIdx = i
		if lineCount >= overlapLines {
			break
		}
	}

	if startIdx >= len(blocks) {
		return nil
	}

	return slices.Clone(blocks[startIdx:])
}

// WriteFiles writes rendered segment contents to files in the given directory.
func WriteFiles(dir, prefix string, contents []string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	for i, content := range contents {
		filename := fmt.Sprintf("%s%d.md", prefix, i+1)
		path := filepath.Join(dir, filename)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}

	return nil
}

// WriteNamedFiles writes rendered segment contents to files with given filenames.
func WriteNamedFiles(dir string, filenames, contents []string) error {
	if len(filenames) != len(contents) {
		return fmt.Errorf("filenames length (%d) != contents length (%d)", len(filenames), len(contents))
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	for i, content := range contents {
		safe := filepath.Base(filenames[i])
		if _, err := ValidateSubPath(dir, safe); err != nil {
			return fmt.Errorf("invalid filename %q: %w", filenames[i], err)
		}
		path := filepath.Join(dir, safe)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}

	return nil
}
