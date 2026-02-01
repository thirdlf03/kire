package ctxheader_test

import (
	"strings"
	"testing"

	"github.com/thirdlf03/kire/internal/ctxheader"
)

func TestInject_Comment(t *testing.T) {
	path := []string{"Top Level", "Section A"}
	result := ctxheader.Inject(path, ctxheader.FormatComment, 0)
	expected := "<!-- context: Top Level > Section A -->\n\n"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestInject_FrontMatter(t *testing.T) {
	path := []string{"Top Level", "Section A"}
	result := ctxheader.Inject(path, ctxheader.FormatFrontMatter, 0)
	expected := "---\ncontext:\n  - Top Level\n  - Section A\n---\n\n"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestInject_Heading(t *testing.T) {
	path := []string{"Top Level", "Section A"}
	result := ctxheader.Inject(path, ctxheader.FormatHeading, 0)
	expected := "# Top Level\n\n## Section A\n\n"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestInject_None(t *testing.T) {
	path := []string{"Top Level", "Section A"}
	result := ctxheader.Inject(path, ctxheader.FormatNone, 0)
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestInject_EmptyPath(t *testing.T) {
	result := ctxheader.Inject(nil, ctxheader.FormatComment, 0)
	if result != "" {
		t.Errorf("expected empty for nil path, got %q", result)
	}
}

func TestInject_MaxDepth(t *testing.T) {
	path := []string{"Level1", "Level2", "Level3"}
	result := ctxheader.Inject(path, ctxheader.FormatComment, 2)
	expected := "<!-- context: Level1 > Level2 -->\n\n"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestInjectFiltered_ExcludesPatterns(t *testing.T) {
	path := []string{"API Reference", "バリデーション:", "フィールド定義"}
	result := ctxheader.InjectFiltered(path, ctxheader.FormatComment, 0, []string{"バリデーション"})
	expected := "<!-- context: API Reference > フィールド定義 -->\n\n"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestInjectFiltered_NoExclusions(t *testing.T) {
	path := []string{"API Reference", "バリデーション:", "フィールド定義"}
	result := ctxheader.InjectFiltered(path, ctxheader.FormatComment, 0, nil)
	expected := "<!-- context: API Reference > バリデーション: > フィールド定義 -->\n\n"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestInject_Heading_MaxLevel6(t *testing.T) {
	// 8 levels deep — heading level should cap at 6
	path := []string{"L1", "L2", "L3", "L4", "L5", "L6", "L7", "L8"}
	result := ctxheader.Inject(path, ctxheader.FormatHeading, 0)
	// L7 and L8 should both be ###### (level 6)
	if count := strings.Count(result, "######"); count < 2 {
		t.Errorf("expected at least 2 occurrences of '######', got %d in:\n%s", count, result)
	}
	// Should not contain ####### (7 hashes)
	if strings.Count(result, "#######") > 0 {
		t.Errorf("expected no 7-level headings in:\n%s", result)
	}
}

func TestInjectFiltered_AllExcluded(t *testing.T) {
	path := []string{"バリデーション:", "処理:"}
	result := ctxheader.InjectFiltered(path, ctxheader.FormatComment, 0, []string{"バリデーション", "処理"})
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}
