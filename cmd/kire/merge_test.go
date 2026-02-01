package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListMDFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.md"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(dir, "b.md"), []byte("b"), 0644)
	os.WriteFile(filepath.Join(dir, "index.md"), []byte("idx"), 0644)
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("txt"), 0644)

	files, err := listMDFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files (excluding index.md), got %d: %v", len(files), files)
	}
	for _, f := range files {
		base := filepath.Base(f)
		if base == "index.md" {
			t.Error("index.md should be excluded")
		}
		if base == "readme.txt" {
			t.Error("non-.md files should be excluded")
		}
	}
}

func TestListMDFiles_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	files, err := listMDFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}
}

func TestCollectDirFiles_WithIndex(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "part1.md"), []byte("p1"), 0644)
	os.WriteFile(filepath.Join(dir, "part2.md"), []byte("p2"), 0644)
	os.WriteFile(filepath.Join(dir, "index.md"), []byte("- [part2.md](part2.md)\n- [part1.md](part1.md)\n"), 0644)

	files, err := collectDirFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	// Should follow index.md ordering: part2 before part1
	if filepath.Base(files[0]) != "part2.md" {
		t.Errorf("expected part2.md first, got %s", filepath.Base(files[0]))
	}
}

func TestCollectDirFiles_WithoutIndex(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "b.md"), []byte("b"), 0644)
	os.WriteFile(filepath.Join(dir, "a.md"), []byte("a"), 0644)

	files, err := collectDirFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	// Should be sorted alphabetically
	if filepath.Base(files[0]) != "a.md" {
		t.Errorf("expected a.md first, got %s", filepath.Base(files[0]))
	}
}

func TestParseIndexOrder_NoLinks(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "part1.md"), []byte("p1"), 0644)
	os.WriteFile(filepath.Join(dir, "index.md"), []byte("# Index\nNo links here.\n"), 0644)

	files, err := parseIndexOrder(dir, filepath.Join(dir, "index.md"))
	if err != nil {
		t.Fatal(err)
	}
	// Should fall back to listMDFiles
	if len(files) != 1 {
		t.Fatalf("expected 1 file (fallback), got %d", len(files))
	}
}

func TestRunMerge_Directory(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "01.md"), []byte("# Part 1\n\nContent 1.\n"), 0644)
	os.WriteFile(filepath.Join(dir, "02.md"), []byte("# Part 2\n\nContent 2.\n"), 0644)

	outFile := filepath.Join(t.TempDir(), "merged.md")

	rootCmd.SetArgs([]string{"merge", "--out", outFile, dir})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty merged output")
	}
}

func TestRunMerge_Files(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "a.md")
	f2 := filepath.Join(dir, "b.md")
	os.WriteFile(f1, []byte("Part A.\n"), 0644)
	os.WriteFile(f2, []byte("Part B.\n"), 0644)

	outFile := filepath.Join(t.TempDir(), "merged.md")

	rootCmd.SetArgs([]string{"merge", "--out", outFile, f1, f2})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseIndexOrder_PathTraversal(t *testing.T) {
	dir := t.TempDir()

	// Create a valid .md file inside the dir
	validFile := filepath.Join(dir, "part1.md")
	if err := os.WriteFile(validFile, []byte("# Part 1"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create an index.md with a traversal reference
	indexContent := `# Index
- [part1.md](part1.md)
- [evil.md](../evil.md)
- [sneaky.md](sub/../../../etc/passwd.md)
`
	indexPath := filepath.Join(dir, "index.md")
	if err := os.WriteFile(indexPath, []byte(indexContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a file outside the dir that the traversal tries to reference
	outsideFile := filepath.Join(filepath.Dir(dir), "evil.md")
	if err := os.WriteFile(outsideFile, []byte("evil"), 0644); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(outsideFile) }()

	files, err := parseIndexOrder(dir, indexPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should only find part1.md (traversal paths are neutralized by filepath.Base)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d: %v", len(files), files)
	}

	if filepath.Base(files[0]) != "part1.md" {
		t.Errorf("expected part1.md, got %s", files[0])
	}
}
