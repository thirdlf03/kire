package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/thirdlf03/kire/internal/output"
)

var mergeCmd = &cobra.Command{
	Use:   "merge [flags] <input-dir|files...>",
	Short: "Merge split segments back into a single Markdown file",
	Long: `Merge previously split segments back into a single Markdown document.

If a directory is given, reads all .md files (excluding index.md).
If index.md exists, uses its ordering; otherwise sorts by filename.

Examples:
  kire merge docs/my-spec/
  kire merge --strip-context -o merged.md docs/my-spec/
  kire merge segment1.md segment2.md segment3.md`,
	Args: cobra.MinimumNArgs(1),
	RunE: runMerge,
}

var (
	flMergeOut          *string
	flMergeStripContext *bool
)

func init() {
	f := mergeCmd.Flags()
	flMergeOut = f.StringP("out", "o", "", "Output file (default: stdout)")
	flMergeStripContext = f.Bool("strip-context", false, "Remove context headers from segments")
	rootCmd.AddCommand(mergeCmd)
}

func runMerge(cmd *cobra.Command, args []string) error {
	// Collect input files
	var files []string

	for _, arg := range args {
		info, err := os.Stat(arg)
		if err != nil {
			return fmt.Errorf("stat %s: %w", arg, err)
		}
		if info.IsDir() {
			dirFiles, err := collectDirFiles(arg)
			if err != nil {
				return err
			}
			files = append(files, dirFiles...)
		} else {
			files = append(files, arg)
		}
	}

	if len(files) == 0 {
		return fmt.Errorf("no .md files found")
	}

	// Read all files
	contents := make([]string, len(files))
	for i, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return fmt.Errorf("read %s: %w", f, err)
		}
		contents[i] = string(data)
	}

	// Merge
	merged := output.MergeSegments(contents, *flMergeStripContext)

	// Output
	if *flMergeOut != "" {
		if err := os.WriteFile(*flMergeOut, []byte(merged), 0644); err != nil {
			return fmt.Errorf("write %s: %w", *flMergeOut, err)
		}
		fmt.Fprintf(os.Stderr, "Merged %d files to %s\n", len(files), *flMergeOut)
	} else {
		fmt.Print(merged)
	}

	return nil
}

// listMDFiles returns sorted .md files from a directory, excluding index.md.
func listMDFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir %s: %w", dir, err)
	}
	var files []string
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() && strings.HasSuffix(name, ".md") && name != "index.md" {
			files = append(files, filepath.Join(dir, name))
		}
	}
	slices.Sort(files)
	return files, nil
}

// collectDirFiles reads .md files from a directory, using index.md ordering if available.
func collectDirFiles(dir string) ([]string, error) {
	indexPath := filepath.Join(dir, "index.md")
	if _, err := os.Stat(indexPath); err == nil {
		return parseIndexOrder(dir, indexPath)
	}
	return listMDFiles(dir)
}

// parseIndexOrder extracts file ordering from index.md.
// Looks for markdown links like [filename.md](filename.md).
func parseIndexOrder(dir, indexPath string) ([]string, error) {
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	var files []string
	seen := make(map[string]bool)

	for _, line := range lines {
		for {
			start := strings.Index(line, "](")
			if start < 0 {
				break
			}
			end := strings.Index(line[start+2:], ")")
			if end < 0 {
				break
			}
			ref := line[start+2 : start+2+end]
			line = line[start+2+end+1:]

			if strings.HasSuffix(ref, ".md") && ref != "index.md" && !seen[ref] {
				safe := filepath.Base(ref)
				fullPath := filepath.Join(dir, safe)
				if _, err := os.Stat(fullPath); err == nil {
					files = append(files, fullPath)
					seen[ref] = true
				}
			}
		}
	}

	if len(files) == 0 {
		return listMDFiles(dir)
	}

	return files, nil
}
