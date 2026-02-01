package output

import (
	"testing"

	"github.com/thirdlf03/kire/internal/model"
)

func TestSlugify_Basic(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Hello World", "hello-world"},
		{"Go Programming", "go-programming"},
		{"foo_bar-baz", "foo-bar-baz"},
		{"  leading and trailing  ", "leading-and-trailing"},
		{"UPPER CASE", "upper-case"},
		{"multiple   spaces", "multiple-spaces"},
		{"with/slash", "with-slash"},
		{"dots.and.more", "dots-and-more"},
		{"", ""},
	}
	for _, tt := range tests {
		got := Slugify(tt.input)
		if got != tt.want {
			t.Errorf("Slugify(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSlugify_Japanese(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"タスク管理API", "タスク管理api"},
		{"日本語テスト", "日本語テスト"},
		{"Hello 世界", "hello-世界"},
		{"正規化ルール", "正規化ルール"},
		{"データベース", "データベース"},
		{"人々の暮らし", "人々の暮らし"},
	}
	for _, tt := range tests {
		got := Slugify(tt.input)
		if got != tt.want {
			t.Errorf("Slugify(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSlugify_SpecialCharacters(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"C++ Programming", "c-programming"},
		{"foo@bar#baz", "foo-bar-baz"},
		{"---", ""},
		{"a---b", "a-b"},
	}
	for _, tt := range tests {
		got := Slugify(tt.input)
		if got != tt.want {
			t.Errorf("Slugify(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSlugify_LongText(t *testing.T) {
	long := "This is a very long heading that should be truncated to fifty runes at most"
	got := Slugify(long)
	if len([]rune(got)) > 50 {
		t.Errorf("Slugify produced %d runes, want <= 50", len([]rune(got)))
	}
	// Should not end with hyphen after truncation
	if got[len(got)-1] == '-' {
		t.Error("slug should not end with hyphen after truncation")
	}
}

func TestGenerateFilenames_WithHeadings(t *testing.T) {
	segments := []model.Segment{
		{Blocks: []model.Block{{Kind: model.BlockHeading, Text: "Introduction"}}},
		{Blocks: []model.Block{{Kind: model.BlockHeading, Text: "Methods"}}},
		{Blocks: []model.Block{{Kind: model.BlockHeading, Text: "Results"}}},
	}
	filenames := GenerateFilenames(segments, nil)
	expected := []string{"01-introduction.md", "02-methods.md", "03-results.md"}
	if len(filenames) != len(expected) {
		t.Fatalf("expected %d filenames, got %d", len(expected), len(filenames))
	}
	for i, want := range expected {
		if filenames[i] != want {
			t.Errorf("filenames[%d] = %q, want %q", i, filenames[i], want)
		}
	}
}

func TestGenerateFilenames_NoHeading(t *testing.T) {
	segments := []model.Segment{
		{
			Blocks:      []model.Block{{Kind: model.BlockParagraph, Text: "some text"}},
			HeadingPath: []string{"Parent", "Section"},
		},
		{
			Blocks: []model.Block{{Kind: model.BlockParagraph, Text: "other text"}},
		},
	}
	filenames := GenerateFilenames(segments, nil)
	if filenames[0] != "01-section.md" {
		t.Errorf("expected HeadingPath fallback, got %q", filenames[0])
	}
	if filenames[1] != "02-segment.md" {
		t.Errorf("expected 'segment' fallback, got %q", filenames[1])
	}
}

func TestGenerateFilenames_Dedup(t *testing.T) {
	segments := []model.Segment{
		{Blocks: []model.Block{{Kind: model.BlockHeading, Text: "Setup"}}},
		{Blocks: []model.Block{{Kind: model.BlockHeading, Text: "Setup"}}},
		{Blocks: []model.Block{{Kind: model.BlockHeading, Text: "Setup"}}},
	}
	filenames := GenerateFilenames(segments, nil)
	if filenames[0] != "01-setup.md" {
		t.Errorf("filenames[0] = %q, want %q", filenames[0], "01-setup.md")
	}
	if filenames[1] != "02-setup-2.md" {
		t.Errorf("filenames[1] = %q, want %q", filenames[1], "02-setup-2.md")
	}
	if filenames[2] != "03-setup-3.md" {
		t.Errorf("filenames[2] = %q, want %q", filenames[2], "03-setup-3.md")
	}
}
