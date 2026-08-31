// Package testhome isolates HOME for tests that write ~/.prunesh without polluting t.TempDir with Go module cache.
package testhome

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// Isolated returns an isolated HOME and GOMODCACHE for filter install tests.
func Isolated(t *testing.T) string {
	t.Helper()
	home, err := os.MkdirTemp("", "prunesh-test-home-*")
	if err != nil {
		t.Fatal(err)
	}
	modCache, err := os.MkdirTemp("", "prunesh-test-modcache-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		chmodTree(home)
		chmodTree(modCache)
		_ = os.RemoveAll(home)
		_ = os.RemoveAll(modCache)
	})
	t.Setenv("HOME", home)
	t.Setenv("GOMODCACHE", modCache)
	return home
}

func chmodTree(root string) {
	_ = filepath.WalkDir(root, func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		_ = os.Chmod(path, 0o700)
		return nil
	})
}
