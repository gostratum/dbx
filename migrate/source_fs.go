package migrate

import (
	"fmt"
	"os"
	"path/filepath"
)

// newFileSourceURL creates a file source URL from a filesystem directory
func newFileSourceURL(dir string) (string, error) {
	// Validate directory exists
	absPath, err := filepath.Abs(dir)
	if err != nil {
		return "", WrapError(err, fmt.Sprintf("failed to resolve absolute path for %s", dir))
	}

	// Check if directory exists
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", WrapError(err, fmt.Sprintf("migration directory does not exist: %s", absPath))
		}
		return "", WrapError(err, fmt.Sprintf("failed to stat migration directory: %s", absPath))
	}

	if !info.IsDir() {
		return "", fmt.Errorf("migration path is not a directory: %s", absPath)
	}

	// Create file source URL (file:// protocol with absolute path)
	sourceURL := fmt.Sprintf("file://%s", absPath)

	return sourceURL, nil
}
