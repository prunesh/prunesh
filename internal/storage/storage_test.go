package storage_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/prunesh/prunesh/internal/storage"
)

func TestDir_createsDirectory(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	dir, err := storage.Dir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := filepath.Join(tmp, ".prunesh")
	if dir != want {
		t.Fatalf("got %q, want %q", dir, want)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("%q is not a directory", dir)
	}
}

func TestDir_idempotent(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	if _, err := storage.Dir(); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if _, err := storage.Dir(); err != nil {
		t.Fatalf("second call: %v", err)
	}
}

func TestDir_missingHOME(t *testing.T) {
	t.Setenv("HOME", "")

	_, err := storage.Dir()
	if err == nil {
		t.Fatal("expected error when HOME is empty")
	}
}
