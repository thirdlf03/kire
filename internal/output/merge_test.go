package output

import (
	"testing"
)

func TestStripContext_Comment(t *testing.T) {
	input := "<!-- context: Architecture > Authentication -->\n\n# Authentication\n\nContent here."
	want := "# Authentication\n\nContent here."
	got := StripContext(input)
	if got != want {
		t.Errorf("StripContext comment:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestStripContext_FrontMatter(t *testing.T) {
	input := "---\ncontext:\n  - Architecture\n  - Authentication\n---\n\n# Authentication\n\nContent here."
	want := "# Authentication\n\nContent here."
	got := StripContext(input)
	if got != want {
		t.Errorf("StripContext front-matter:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestStripContext_None(t *testing.T) {
	input := "# Title\n\nJust content."
	got := StripContext(input)
	if got != input {
		t.Errorf("StripContext none: modified content that has no context header")
	}
}

func TestStripContext_Empty(t *testing.T) {
	got := StripContext("")
	if got != "" {
		t.Error("expected empty string")
	}
}

func TestMergeSegments(t *testing.T) {
	contents := []string{
		"<!-- context: A -->\n\n# Title\n\nFirst paragraph.\n",
		"<!-- context: A > B -->\n\n## Section\n\nSecond paragraph.\n",
		"# Standalone\n\nThird paragraph.\n",
	}

	got := MergeSegments(contents, true)

	// Should strip context headers and join
	if got == "" {
		t.Fatal("expected non-empty result")
	}

	// Should not contain context comments
	if contains(got, "<!-- context:") {
		t.Error("merged output should not contain context comments")
	}

	// Should contain all real content
	for _, s := range []string{"Title", "First paragraph", "Section", "Second paragraph", "Standalone", "Third paragraph"} {
		if !contains(got, s) {
			t.Errorf("merged output missing %q", s)
		}
	}
}

func TestMergeSegmentsNoStrip(t *testing.T) {
	contents := []string{
		"<!-- context: A -->\n\n# Title\n",
		"Content\n",
	}

	got := MergeSegments(contents, false)

	if !contains(got, "<!-- context:") {
		t.Error("without strip, context should be preserved")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
