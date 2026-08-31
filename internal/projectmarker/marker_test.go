package projectmarker_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/prunesh/prunesh/internal/projectmarker"
)

func TestExistsFalseWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	if projectmarker.Exists(dir) {
		t.Fatal("expected false for dir without marker")
	}
}

func TestCreateWritesEmptyFile(t *testing.T) {
	dir := t.TempDir()
	if err := projectmarker.Create(dir); err != nil {
		t.Fatal(err)
	}
	if !projectmarker.Exists(dir) {
		t.Fatal("expected marker to exist after Create")
	}
	info, err := os.Stat(filepath.Join(dir, projectmarker.MarkerName))
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != 0 {
		t.Fatalf("marker must be empty, got %d bytes", info.Size())
	}
}

func TestCreateIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	if err := projectmarker.Create(dir); err != nil {
		t.Fatal(err)
	}
	if err := projectmarker.Create(dir); err != nil {
		t.Fatalf("second Create must not fail: %v", err)
	}
	if !projectmarker.Exists(dir) {
		t.Fatal("marker must still exist after second Create")
	}
}

func TestProjectRootFallsBackToDir(t *testing.T) {
	dir := t.TempDir()
	root := projectmarker.ProjectRoot(dir)
	if root == "" {
		t.Fatal("ProjectRoot must not return empty string")
	}
}
