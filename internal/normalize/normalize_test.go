package normalize

import (
	"testing"
)

func TestNormalize_Unicode(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"NFC composition", "e\u0301", "é"}, // e + combining acute → é
		{"already NFC", "café", "café"},
		{"empty", "", ""},
	}

	cfg := Config{Unicode: true}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Normalize(tt.input, cfg)
			if got != tt.want {
				t.Errorf("Normalize(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalize_CompactSpace(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"multiple spaces", "hello    world", "hello world"},
		{"tabs and newlines", "hello\t\n\r  world", "hello world"},
		{"leading/trailing", "  hello world  ", "hello world"},
		{"single space", "hello world", "hello world"},
		{"empty", "", ""},
	}

	cfg := Config{CompactSpace: true}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Normalize(tt.input, cfg)
			if got != tt.want {
				t.Errorf("Normalize(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalize_Lowercase(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"mixed case", "Hello World", "hello world"},
		{"already lower", "hello", "hello"},
		{"empty", "", ""},
	}

	cfg := Config{Lowercase: true}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Normalize(tt.input, cfg)
			if got != tt.want {
				t.Errorf("Normalize(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalize_StripMarkdown(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"headings", "# Hello\n## World", "Hello\nWorld"},
		{"emphasis", "**bold** and *italic*", "bold and italic"},
		{"links", "[text](http://example.com)", "text"},
		{"images", "![alt](image.png)", "alt"},
		{"inline code", "use `code` here", "use code here"},
		{"blockquote", "> quoted text", "quoted text"},
		{"list unordered", "- item one\n* item two", "item one\nitem two"},
		{"list ordered", "1. first\n2. second", "first\nsecond"},
		{"horizontal rule", "text\n---\nmore", "text\n\nmore"},
		{"html tags", "<div>content</div>", "content"},
		{"code block marker", "```go\ncode\n```", "\ncode\n"},
		{"empty", "", ""},
	}

	cfg := Config{StripMarkdown: true}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Normalize(tt.input, cfg)
			if got != tt.want {
				t.Errorf("Normalize(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalize_Combined(t *testing.T) {
	cfg := Config{
		Unicode:       true,
		CompactSpace:  true,
		StripMarkdown: true,
		Lowercase:     true,
	}

	input := "# Café\n\n**Bold**  and   *italic*"
	want := "café bold and italic"

	got := Normalize(input, cfg)
	if got != want {
		t.Errorf("Normalize(%q) = %q, want %q", input, got, want)
	}
}

func TestNormalize_Idempotent(t *testing.T) {
	cfg := Config{
		Unicode:       true,
		CompactSpace:  true,
		StripMarkdown: true,
		Lowercase:     true,
	}

	input := "# Hello World"
	first := Normalize(input, cfg)
	second := Normalize(first, cfg)

	if first != second {
		t.Errorf("not idempotent: first=%q, second=%q", first, second)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if !cfg.Unicode {
		t.Error("default should enable Unicode")
	}
	if !cfg.CompactSpace {
		t.Error("default should enable CompactSpace")
	}
	if cfg.StripMarkdown {
		t.Error("default should disable StripMarkdown")
	}
	if cfg.Lowercase {
		t.Error("default should disable Lowercase")
	}
}

func TestNormalizer(t *testing.T) {
	cfg := Config{Lowercase: true}
	fn := Normalizer(cfg)
	got := fn("HELLO")
	if got != "hello" {
		t.Errorf("Normalizer function returned %q, want %q", got, "hello")
	}
}

func BenchmarkNormalize_Simple(b *testing.B) {
	cfg := DefaultConfig()
	text := "Hello World"
	for i := 0; i < b.N; i++ {
		Normalize(text, cfg)
	}
}

func BenchmarkNormalize_Full(b *testing.B) {
	cfg := Config{
		Unicode:       true,
		CompactSpace:  true,
		StripMarkdown: true,
		Lowercase:     true,
	}
	text := "# Heading\n\n**Bold** text with `code` and [links](url) and   extra   spaces."
	for i := 0; i < b.N; i++ {
		Normalize(text, cfg)
	}
}
