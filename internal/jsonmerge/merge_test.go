package jsonmerge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runMerge(t *testing.T, filePath, patchJSON string) (bool, error) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	if _, err := w.WriteString(patchJSON); err != nil {
		t.Fatalf("write pipe: %v", err)
	}
	w.Close()

	origStdin := os.Stdin
	os.Stdin = r
	defer func() {
		os.Stdin = origStdin
		r.Close()
	}()

	return MergeFile(filePath)
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return out
}

func TestMergeFile_EmptyPath(t *testing.T) {
	_, err := MergeFile("")
	if err == nil {
		t.Fatal("empty path must fail")
	}
}

func TestMergeFile_NewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.json")

	patch := `{"version":1,"hooks":{"preToolUse":[{"command":"/tmp/prunesh-pre.sh","matcher":"Shell"}]}}`
	changed, err := runMerge(t, path, patch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Error("expected changed=true for new file")
	}

	got := readJSON(t, path)
	hooks, ok := got["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("hooks not a map: %T", got["hooks"])
	}
	if _, exists := hooks["preToolUse"]; !exists {
		t.Error("expected hooks.preToolUse to exist")
	}
}

func TestMergeFile_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.json")

	patch := `{"hooks":{"preToolUse":[{"command":"/tmp/prunesh-pre.sh"}]}}`
	if _, err := runMerge(t, path, patch); err != nil {
		t.Fatalf("first merge: %v", err)
	}
	changed, err := runMerge(t, path, patch)
	if err != nil {
		t.Fatalf("second merge: %v", err)
	}
	if changed {
		t.Error("expected changed=false for idempotent merge")
	}
}

func TestMergeFile_ArrayDedup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.json")

	initial := `{"hooks":{"preToolUse":[{"command":"/existing/hook.sh"}]}}`
	if err := os.WriteFile(path, []byte(initial), 0644); err != nil {
		t.Fatalf("write initial: %v", err)
	}

	patch := `{"hooks":{"preToolUse":[{"command":"/tmp/prunesh-pre.sh"}]}}`
	if _, err := runMerge(t, path, patch); err != nil {
		t.Fatalf("merge: %v", err)
	}

	got := readJSON(t, path)
	hooks := got["hooks"].(map[string]any)
	pre := hooks["preToolUse"].([]any)
	if len(pre) != 2 {
		t.Errorf("expected 2 preToolUse hooks, got %d", len(pre))
	}

	changed, err := runMerge(t, path, patch)
	if err != nil {
		t.Fatalf("second merge: %v", err)
	}
	if changed {
		t.Error("expected no-op when hook already present")
	}
}

func TestMergeFile_InvalidStdin(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	_, err := runMerge(t, path, `not valid json`)
	if err == nil {
		t.Error("expected error for invalid stdin JSON")
	}
	if !strings.Contains(err.Error(), "read patch from stdin") {
		t.Errorf("unexpected error message: %v", err)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Error("file should not be created on stdin error")
	}
}

func TestMergeFile_ParentDirCreated(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "nested", "config.json")

	changed, err := runMerge(t, path, `{"key":"value"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Error("expected changed=true")
	}
}

func TestDeepMerge_ScalarPatchWins(t *testing.T) {
	target := map[string]any{"version": float64(1)}
	patch := map[string]any{"version": float64(2)}
	result := deepMerge(target, patch).(map[string]any)
	if result["version"] != float64(2) {
		t.Errorf("expected patch value 2, got %v", result["version"])
	}
}
