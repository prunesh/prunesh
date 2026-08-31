// Package projectmarker manages the .prunesh project marker.
// The marker is an empty file that signals prunesh is active in the project.
// Hooks exit immediately when it is absent.
package projectmarker

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const MarkerName = ".prunesh"

// ProjectRoot returns the git root of dir, or dir itself when not in a repo.
func ProjectRoot(dir string) string {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err == nil {
		return strings.TrimSpace(string(out))
	}
	return dir
}

// Exists reports whether a .prunesh marker is present at root.
func Exists(root string) bool {
	_, err := os.Stat(filepath.Join(root, MarkerName))
	return err == nil
}

// Create writes an empty .prunesh marker at root. Idempotent.
func Create(root string) error {
	path := filepath.Join(root, MarkerName)
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	return f.Close()
}
