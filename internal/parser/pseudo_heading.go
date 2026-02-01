package parser

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/thirdlf03/kire/internal/model"
)

// PseudoHeadingConfig configures pseudo-heading detection.
type PseudoHeadingConfig struct {
	Enabled         bool
	MaxRuneLen      int      // Maximum rune length for candidate lines (default 40)
	ColonSuffixes   bool     // Detect lines ending with : or ：
	NumberedItems   bool     // Detect lines starting with \d+)
	ExcludePatterns []string // Partial-match patterns to exclude from promotion
	PrefixPatterns  []string // Regex patterns to match line start (bypasses MaxRuneLen)
}

// DefaultPseudoHeadingConfig returns a config with sensible defaults.
func DefaultPseudoHeadingConfig() PseudoHeadingConfig {
	return PseudoHeadingConfig{
		Enabled:       true,
		MaxRuneLen:    60,
		ColonSuffixes: true,
		NumberedItems: true,
		ExcludePatterns: []string{
			"バリデーション",
			"レスポンス",
			"処理",
			"エラー",
			"パラメータ",
			"ステータスコード",
		},
	}
}

// PseudoHeadingLevel is the heading level assigned to pseudo-headings.
const PseudoHeadingLevel = 7

var numberedItemRe = regexp.MustCompile(`^\d+\)`)

// CompilePrefixPatterns compiles PrefixPatterns from config, skipping invalid ones.
func CompilePrefixPatterns(patterns []string) []*regexp.Regexp {
	var res []*regexp.Regexp
	for _, pat := range patterns {
		re, err := regexp.Compile(pat)
		if err == nil {
			res = append(res, re)
		}
	}
	return res
}

// IsPseudoHeadingCandidate returns true if the given text looks like a
// pseudo-heading according to the config rules (ColonSuffix, NumberedItems,
// PrefixPatterns, MaxRuneLen, ExcludePatterns).
// The text should already be TrimSpace'd.
// prefixRes is the pre-compiled PrefixPatterns (use compilePrefixPatterns).
func IsPseudoHeadingCandidate(text string, cfg PseudoHeadingConfig, prefixRes []*regexp.Regexp) bool {
	if text == "" {
		return false
	}

	maxLen := cfg.MaxRuneLen
	if maxLen <= 0 {
		maxLen = 40
	}

	promoted := false

	// Check PrefixPatterns first (bypasses MaxRuneLen)
	for _, re := range prefixRes {
		if re.MatchString(text) {
			promoted = true
			break
		}
	}

	// Check standard patterns (subject to MaxRuneLen)
	if !promoted && utf8.RuneCountInString(text) <= maxLen {
		if cfg.ColonSuffixes && (strings.HasSuffix(text, ":") || strings.HasSuffix(text, "：")) {
			promoted = true
		}
		if cfg.NumberedItems && numberedItemRe.MatchString(text) {
			promoted = true
		}
	}

	if promoted && len(cfg.ExcludePatterns) > 0 {
		for _, pat := range cfg.ExcludePatterns {
			if strings.Contains(text, pat) {
				return false
			}
		}
	}

	return promoted
}

// DetectPseudoHeadings promotes short paragraph blocks that look like headings.
// Promoted blocks get Kind=BlockHeading, HeadingLevel=7, IsPseudoHeading=true.
// After promotion, heading paths are rebuilt.
func DetectPseudoHeadings(blocks []model.Block, cfg PseudoHeadingConfig) []model.Block {
	if !cfg.Enabled || len(blocks) == 0 {
		return blocks
	}

	prefixRes := CompilePrefixPatterns(cfg.PrefixPatterns)

	for i := range blocks {
		if blocks[i].Kind != model.BlockParagraph {
			continue
		}
		text := strings.TrimSpace(blocks[i].Text)

		if IsPseudoHeadingCandidate(text, cfg, prefixRes) {
			blocks[i].Kind = model.BlockHeading
			blocks[i].HeadingLevel = PseudoHeadingLevel
			blocks[i].IsPseudoHeading = true
		}
	}

	rebuildHeadingPaths(blocks)
	return blocks
}

// AutoSplitParagraphs splits paragraph blocks at lines that are pseudo-heading
// candidates. This ensures that pseudo-heading-like lines buried inside a large
// paragraph (due to goldmark merging consecutive non-blank lines) become
// independent blocks that DetectPseudoHeadings can then promote.
// The function is a no-op when cfg.Enabled is false.
func AutoSplitParagraphs(blocks []model.Block, cfg PseudoHeadingConfig, source []byte) []model.Block {
	if !cfg.Enabled {
		return blocks
	}

	prefixRes := CompilePrefixPatterns(cfg.PrefixPatterns)

	var result []model.Block
	for _, b := range blocks {
		if b.Kind != model.BlockParagraph {
			result = append(result, b)
			continue
		}

		text := b.Text
		lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
		if len(lines) <= 1 {
			result = append(result, b)
			continue
		}

		// Find line indices that are pseudo-heading candidates
		var splitPoints []int
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if IsPseudoHeadingCandidate(trimmed, cfg, prefixRes) {
				splitPoints = append(splitPoints, i)
			}
		}

		if len(splitPoints) == 0 {
			result = append(result, b)
			continue
		}

		splits := splitParagraphAtLines(b, splitPoints, source)
		result = append(result, splits...)
	}
	return result
}

// rebuildHeadingPaths reconstructs HeadingPath for all blocks using the same
// heading-stack algorithm as the parser.
func rebuildHeadingPaths(blocks []model.Block) {
	type entry struct {
		level int
		title string
	}
	var stack []entry

	for i := range blocks {
		b := &blocks[i]
		if b.Kind == model.BlockHeading {
			// Pop headings at same or deeper level
			for len(stack) > 0 && stack[len(stack)-1].level >= b.HeadingLevel {
				stack = stack[:len(stack)-1]
			}
			title := strings.TrimSpace(b.Text)
			stack = append(stack, entry{level: b.HeadingLevel, title: title})

			// Heading's own path = ancestors (excluding self)
			if len(stack) > 1 {
				path := make([]string, len(stack)-1)
				for j := 0; j < len(stack)-1; j++ {
					path[j] = stack[j].title
				}
				b.HeadingPath = path
			} else {
				b.HeadingPath = nil
			}
		} else {
			// Non-heading: path = full stack
			if len(stack) > 0 {
				path := make([]string, len(stack))
				for j, e := range stack {
					path[j] = e.title
				}
				b.HeadingPath = path
			} else {
				b.HeadingPath = nil
			}
		}
	}
}
