package output

import (
	"regexp"
	"strings"
)

var (
	// <!-- context: ... --> followed by optional blank lines
	reContextComment = regexp.MustCompile(`(?s)^<!--\s*context:.*?-->\s*\n*`)
	// ---\ncontext:\n  - ...\n---\n followed by optional blank lines
	reContextFrontMatter = regexp.MustCompile(`(?s)^---\ncontext:\n(?:\s+-\s+.+\n)+---\s*\n*`)
)

// StripContext removes context headers (comment or front-matter format) from segment content.
func StripContext(content string) string {
	if content == "" {
		return ""
	}

	// Try comment format
	if loc := reContextComment.FindStringIndex(content); loc != nil && loc[0] == 0 {
		content = content[loc[1]:]
	}

	// Try front-matter format
	if loc := reContextFrontMatter.FindStringIndex(content); loc != nil && loc[0] == 0 {
		content = content[loc[1]:]
	}

	return content
}

// MergeSegments joins segment contents into a single document.
// If stripContext is true, context headers are removed before joining.
func MergeSegments(contents []string, stripContext bool) string {
	if len(contents) == 0 {
		return ""
	}

	var b strings.Builder
	for i, c := range contents {
		if stripContext {
			c = StripContext(c)
		}
		b.WriteString(c)
		// Ensure segments are separated by at least one newline
		if i < len(contents)-1 && !strings.HasSuffix(c, "\n") {
			b.WriteByte('\n')
		}
	}

	return b.String()
}
