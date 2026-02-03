// Package normalize provides text normalization utilities for embedding preprocessing.
package normalize

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Config holds normalization options.
type Config struct {
	Unicode       bool // Apply NFC Unicode normalization
	CompactSpace  bool // Collapse multiple whitespaces into single spaces
	StripMarkdown bool // Remove Markdown syntax symbols
	Lowercase     bool // Convert to lowercase
}

// DefaultConfig returns a reasonable default configuration.
func DefaultConfig() Config {
	return Config{
		Unicode:       true,
		CompactSpace:  true,
		StripMarkdown: false,
		Lowercase:     false,
	}
}

// Normalize applies configured transformations to the input text.
// The order of operations is: Unicode → StripMarkdown → CompactSpace → Lowercase.
func Normalize(text string, cfg Config) string {
	if text == "" {
		return text
	}

	// Step 1: Unicode NFC normalization
	if cfg.Unicode {
		text = norm.NFC.String(text)
	}

	// Step 2: Strip Markdown symbols
	if cfg.StripMarkdown {
		text = stripMarkdown(text)
	}

	// Step 3: Compact whitespace
	if cfg.CompactSpace {
		text = compactWhitespace(text)
	}

	// Step 4: Lowercase
	if cfg.Lowercase {
		text = strings.ToLower(text)
	}

	return text
}

// stripMarkdown removes common Markdown syntax elements while preserving content.
func stripMarkdown(text string) string {
	// Remove code block markers
	text = reCodeBlockMarker.ReplaceAllString(text, "")

	// Remove inline code backticks
	text = reInlineCode.ReplaceAllString(text, "$1")

	// Remove emphasis markers (**, __, *, _)
	text = reStrongEmphasis.ReplaceAllString(text, "$1")
	text = reEmphasis.ReplaceAllString(text, "$1")

	// Remove link/image syntax, keeping text
	text = reImageLink.ReplaceAllString(text, "$1")
	text = reLink.ReplaceAllString(text, "$1")

	// Remove heading markers (# ## ### etc.)
	text = reHeading.ReplaceAllString(text, "$1")

	// Remove horizontal rules
	text = reHorizontalRule.ReplaceAllString(text, "")

	// Remove blockquote markers
	text = reBlockquote.ReplaceAllString(text, "")

	// Remove list markers (-, *, +, 1., 2., etc.)
	text = reListMarker.ReplaceAllString(text, "")

	// Remove HTML tags
	text = reHTMLTag.ReplaceAllString(text, "")

	return text
}

// Precompiled regex patterns for Markdown stripping
var (
	reCodeBlockMarker = regexp.MustCompile("(?m)^```[a-zA-Z]*\\s*$")
	reInlineCode      = regexp.MustCompile("`([^`]+)`")
	reStrongEmphasis  = regexp.MustCompile(`\*\*([^*]+)\*\*|__([^_]+)__`)
	reEmphasis        = regexp.MustCompile(`\*([^*]+)\*|_([^_]+)_`)
	reImageLink       = regexp.MustCompile(`!\[([^\]]*)\]\([^)]+\)`)
	reLink            = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
	reHeading         = regexp.MustCompile(`(?m)^#{1,6}\s+(.+)$`)
	reHorizontalRule  = regexp.MustCompile(`(?m)^[-*_]{3,}\s*$`)
	reBlockquote      = regexp.MustCompile(`(?m)^>\s*`)
	reListMarker      = regexp.MustCompile(`(?m)^[\s]*[-*+]\s+|^[\s]*\d+\.\s+`)
	reHTMLTag         = regexp.MustCompile(`<[^>]+>`)
)

// compactWhitespace collapses runs of whitespace into single spaces.
func compactWhitespace(text string) string {
	var builder strings.Builder
	builder.Grow(len(text))

	lastWasSpace := false
	for _, r := range text {
		if unicode.IsSpace(r) {
			if !lastWasSpace {
				builder.WriteRune(' ')
				lastWasSpace = true
			}
		} else {
			builder.WriteRune(r)
			lastWasSpace = false
		}
	}

	return strings.TrimSpace(builder.String())
}

// Normalizer returns a normalizing function configured with the given options.
// This is useful for passing to embedding options.
func Normalizer(cfg Config) func(string) string {
	return func(text string) string {
		return Normalize(text, cfg)
	}
}
