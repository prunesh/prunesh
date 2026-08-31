package proxy_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/prunesh/prunesh/internal/proxy"
	"github.com/prunesh/prunesh/plugins/gain"

	_ "github.com/prunesh/prunesh/plugins/git"
	_ "github.com/prunesh/prunesh/plugins/ls"
)

func TestRunGitStatus(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	run("init", "-q")
	run("config", "user.email", "t@t")
	run("config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "a.go")
	run("commit", "-q", "-m", "init")
	if err := os.WriteFile(filepath.Join(dir, "b.go"), []byte("package b\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	code := proxy.Run("git", []string{"status"})
	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	_ = r.Close()

	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	out := buf.String()
	if !strings.Contains(out, "* ") {
		t.Fatalf("expected branch line, got %q", out)
	}
	if !strings.Contains(out, "Untracked") {
		t.Fatalf("expected untracked group, got %q", out)
	}

	tr, err := gain.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer tr.Close()
	s, err := tr.GetSummary()
	if err != nil {
		t.Fatal(err)
	}
	if s.TotalCommands != 1 {
		t.Fatalf("gain records: %d", s.TotalCommands)
	}
}

func TestRunUnknown(t *testing.T) {
	code := proxy.Run("echo", []string{"hi"})
	if code != 1 {
		t.Fatalf("exit %d", code)
	}
}

func TestRunLs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := t.TempDir()
	for i := 0; i < 40; i++ {
		name := filepath.Join(dir, fmt.Sprintf("handler_%02d.go", i))
		if err := os.WriteFile(name, []byte("package p\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	out := captureStdout(t, func() int { return proxy.Run("ls", nil) })
	if !strings.Contains(out, "files") {
		t.Fatalf("expected compact ls, got %q", out)
	}
	if strings.Count(out, "handler_") > 12 {
		t.Fatalf("expected sampled names, got %q", out)
	}
}

func captureStdout(t *testing.T, fn func() int) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	code := fn()
	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	_ = r.Close()
	if code != 0 {
		t.Fatalf("exit %d, out %q", code, buf.String())
	}
	return buf.String()
}
