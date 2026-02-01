package ctxheader

import (
	"fmt"
	"strings"
)

const (
	FormatComment     = "comment"
	FormatFrontMatter = "front-matter"
	FormatHeading     = "heading"
	FormatNone        = "none"
)

// InjectFiltered generates a context header, filtering out heading path entries
// that contain any of the exclude patterns (partial match).
func InjectFiltered(headingPath []string, format string, maxDepth int, excludePatterns []string) string {
	if len(excludePatterns) > 0 && len(headingPath) > 0 {
		filtered := make([]string, 0, len(headingPath))
		for _, h := range headingPath {
			excluded := false
			for _, pat := range excludePatterns {
				if strings.Contains(h, pat) {
					excluded = true
					break
				}
			}
			if !excluded {
				filtered = append(filtered, h)
			}
		}
		headingPath = filtered
	}
	return Inject(headingPath, format, maxDepth)
}

// Inject generates a context header from the heading path.
// maxDepth of 0 means no limit.
func Inject(headingPath []string, format string, maxDepth int) string {
	if len(headingPath) == 0 || format == FormatNone {
		return ""
	}

	path := headingPath
	if maxDepth > 0 && len(path) > maxDepth {
		path = path[:maxDepth]
	}

	switch format {
	case FormatComment:
		return fmt.Sprintf("<!-- context: %s -->\n\n", strings.Join(path, " > "))
	case FormatFrontMatter:
		var b strings.Builder
		b.WriteString("---\ncontext:\n")
		for _, h := range path {
			b.WriteString("  - ")
			b.WriteString(h)
			b.WriteString("\n")
		}
		b.WriteString("---\n\n")
		return b.String()
	case FormatHeading:
		var b strings.Builder
		for i, h := range path {
			level := i + 1
			if level > 6 {
				level = 6
			}
			for j := 0; j < level; j++ {
				b.WriteByte('#')
			}
			b.WriteByte(' ')
			b.WriteString(h)
			b.WriteString("\n\n")
		}
		return b.String()
	default:
		return ""
	}
}
