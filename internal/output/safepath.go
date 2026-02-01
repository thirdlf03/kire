package output

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ValidateSubPath checks that the given path, when joined with baseDir,
// stays within baseDir. This prevents path traversal attacks.
func ValidateSubPath(baseDir, sub string) (string, error) {
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("resolve base dir: %w", err)
	}

	joined := filepath.Join(absBase, sub)
	absJoined, err := filepath.Abs(joined)
	if err != nil {
		return "", fmt.Errorf("resolve sub path: %w", err)
	}

	// Ensure the resolved path is within the base directory
	if !strings.HasPrefix(absJoined, absBase+string(filepath.Separator)) && absJoined != absBase {
		return "", fmt.Errorf("path traversal detected: %q escapes base %q", sub, baseDir)
	}

	return absJoined, nil
}
