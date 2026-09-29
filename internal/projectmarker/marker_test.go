package projectmarker_test

import (
	"os"
	"os/exec"
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

func TestInActiveProjectFalseWhenNotGitRepo(t *testing.T) {
	dir := t.TempDir()
	if projectmarker.InActiveProject(dir) {
		t.Fatal("expected false for non-git directory")
	}
}

func TestInActiveProjectFalseWhenMarkerAbsent(t *testing.T) {
	dir := t.TempDir()
	mustGitInit(t, dir)
	if projectmarker.InActiveProject(dir) {
		t.Fatal("expected false when git repo has no .prunesh marker")
	}
}

func TestInActiveProjectTrueWhenMarkerPresent(t *testing.T) {
	dir := t.TempDir()
	mustGitInit(t, dir)
	if err := projectmarker.Create(dir); err != nil {
		t.Fatal(err)
	}
	if !projectmarker.InActiveProject(dir) {
		t.Fatal("expected true when git repo has .prunesh marker")
	}
}

func mustGitInit(t *testing.T, dir string) {
	t.Helper()
	out, err := exec.Command("git", "init", dir).CombinedOutput()
	if err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
}
