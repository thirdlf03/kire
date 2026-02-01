package output

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/thirdlf03/kire/internal/model"
)

// Slugify converts text to a URL-safe slug.
// Lowercase, spaces/symbols → hyphens, consecutive hyphens collapsed,
// CJK characters preserved, max 50 runes.
func Slugify(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}

	var b strings.Builder
	prevHyphen := false
	for _, r := range text {
		if isSlugSafe(r) {
			b.WriteRune(unicode.ToLower(r))
			prevHyphen = false
		} else if r == ' ' || r == '_' || r == '-' || unicode.IsPunct(r) || unicode.IsSymbol(r) {
			if !prevHyphen && b.Len() > 0 {
				b.WriteByte('-')
				prevHyphen = true
			}
		} else if isCJK(r) {
			// prevHyphen before CJK is intentionally kept
			b.WriteRune(r)
			prevHyphen = false
		}
		// skip other characters
	}

	result := b.String()
	// Trim trailing hyphens
	result = strings.TrimRight(result, "-")

	// Truncate to 50 runes
	if utf8.RuneCountInString(result) > 50 {
		count := 0
		for i := range result {
			if count == 50 {
				result = result[:i]
				break
			}
			count++
		}
		result = strings.TrimRight(result, "-")
	}

	return result
}

func isSlugSafe(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

func isCJK(r rune) bool {
	return unicode.Is(unicode.Han, r) ||
		unicode.Is(unicode.Hiragana, r) ||
		unicode.Is(unicode.Katakana, r) ||
		unicode.Is(unicode.Hangul, r) ||
		(r >= 0x3000 && r <= 0x30FF) // CJK記号・ひらがな・カタカナブロック全体 (ー々〇・等を含む)
}

// GenerateFilenames produces semantic filenames for segments.
// Each filename is "NN-slug.md" where NN is zero-padded index and slug
// is derived from the segment's first heading or HeadingPath.
func GenerateFilenames(segments []model.Segment, source []byte) []string {
	width := len(fmt.Sprintf("%d", len(segments)))
	if width < 2 {
		width = 2
	}

	used := make(map[string]int) // slug → count for dedup
	filenames := make([]string, len(segments))
	for i, seg := range segments {
		slug := segmentSlug(seg)
		if slug == "" {
			slug = "segment"
		}

		// Dedup
		key := slug
		if count, ok := used[key]; ok {
			used[key] = count + 1
			slug = fmt.Sprintf("%s-%d", slug, count+1)
		} else {
			used[key] = 1
		}

		filenames[i] = fmt.Sprintf("%0*d-%s.md", width, i+1, slug)
	}
	return filenames
}

func segmentSlug(seg model.Segment) string {
	// Try first heading block
	for _, b := range seg.Blocks {
		if b.Kind == model.BlockHeading {
			return Slugify(b.Text)
		}
	}
	// Fallback: last element of HeadingPath
	if len(seg.HeadingPath) > 0 {
		return Slugify(seg.HeadingPath[len(seg.HeadingPath)-1])
	}
	return ""
}
