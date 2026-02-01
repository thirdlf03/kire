package parser

import (
	"os"
	"testing"
)

func BenchmarkParse_Simple(b *testing.B) {
	source, err := os.ReadFile("../../testdata/simple.md")
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for b.Loop() {
		_, _ = Parse(source)
	}
}

func BenchmarkParse_NestedHeadings(b *testing.B) {
	source, err := os.ReadFile("../../testdata/nested_headings.md")
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for b.Loop() {
		_, _ = Parse(source)
	}
}

func BenchmarkParse_BenchLong(b *testing.B) {
	source, err := os.ReadFile("../../testdata/bench_long.md")
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for b.Loop() {
		_, _ = Parse(source)
	}
}
