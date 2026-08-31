// Package storage resolves the prunesh data directory (~/.prunesh).
package storage

import (
	"fmt"
	"os"
	"path/filepath"
)

// Dir returns ~/.prunesh and ensures it exists.
func Dir() (string, error) {
	home := os.Getenv("HOME")
	if home == "" {
		return "", fmt.Errorf("HOME is not set")
	}
	dir := filepath.Join(home, ".prunesh")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}
