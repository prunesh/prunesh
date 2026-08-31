package shell

import (
	"testing"

	_ "github.com/prunesh/prunesh/plugins/cargo"
	_ "github.com/prunesh/prunesh/plugins/find"
	_ "github.com/prunesh/prunesh/plugins/go"
	_ "github.com/prunesh/prunesh/plugins/git"
	_ "github.com/prunesh/prunesh/plugins/grep"
	_ "github.com/prunesh/prunesh/plugins/ls"
	_ "github.com/prunesh/prunesh/plugins/docker"
	_ "github.com/prunesh/prunesh/plugins/npmtest"
	_ "github.com/prunesh/prunesh/plugins/pytest"
	_ "github.com/prunesh/prunesh/plugins/python"
	_ "github.com/prunesh/prunesh/plugins/readcmd"
	_ "github.com/prunesh/prunesh/plugins/rg"
	_ "github.com/prunesh/prunesh/plugins/tree"
)

func TestRewriteNpmTest(t *testing.T) {
	got, ok := Rewrite("npm test", "prunesh")
	if !ok || got != "prunesh npm test" {
		t.Fatalf("got %q ok=%v", got, ok)
	}
}

func TestRewriteDockerPS(t *testing.T) {
	got, ok := Rewrite("docker ps", "prunesh")
	if !ok || got != "prunesh docker ps" {
		t.Fatalf("got %q ok=%v", got, ok)
	}
}

func TestRewritePytest(t *testing.T) {
	got, ok := Rewrite("pytest -v", "prunesh")
	if !ok || got != "prunesh pytest -v" {
		t.Fatalf("got %q ok=%v", got, ok)
	}
}

func TestRewritePythonMPytest(t *testing.T) {
	got, ok := Rewrite("python -m pytest", "prunesh")
	if !ok || got != "prunesh python -m pytest" {
		t.Fatalf("got %q ok=%v", got, ok)
	}
	got, ok = Rewrite("python3 -m pytest tests/", "prunesh")
	if !ok || got != "prunesh python3 -m pytest tests/" {
		t.Fatalf("got %q ok=%v", got, ok)
	}
}

func TestRewriteCargoTest(t *testing.T) {
	got, ok := Rewrite("cargo test", "prunesh")
	if !ok || got != "prunesh cargo test" {
		t.Fatalf("got %q ok=%v", got, ok)
	}
}

func TestRewriteGoTest(t *testing.T) {
	got, ok := Rewrite("go test ./...", "prunesh")
	if !ok || got != "prunesh go test ./..." {
		t.Fatalf("got %q ok=%v", got, ok)
	}
}

func TestRewriteGitStatus(t *testing.T) {
	got, ok := Rewrite("git status", "prunesh")
	if !ok || got != "prunesh git status" {
		t.Fatalf("got %q ok=%v", got, ok)
	}
}

func TestRewritePrefixes(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"/usr/bin/git status", "prunesh git status"},
		{"sudo git status", "sudo prunesh git status"},
		{"VAR=1 git status", "VAR=1 prunesh git status"},
		{"git -C /tmp status", "prunesh git -C /tmp status"},
		{"sudo /usr/bin/git -C /tmp status", "sudo prunesh git -C /tmp status"},
	}
	for _, tc := range cases {
		got, ok := Rewrite(tc.in, "prunesh")
		if !ok || got != tc.want {
			t.Errorf("%q: got %q ok=%v want %q", tc.in, got, ok, tc.want)
		}
	}
}

func TestRewriteUnregistered(t *testing.T) {
	_, ok := Rewrite("echo hi", "prunesh")
	if ok {
		t.Fatal("echo should not be rewritten")
	}
}

func TestRewriteAlreadyGtkai(t *testing.T) {
	_, ok := Rewrite("prunesh git status", "prunesh")
	if ok {
		t.Fatal("already-proxied command should not be rewritten")
	}
}

func TestRewriteEmptyBin(t *testing.T) {
	_, ok := Rewrite("git status", "")
	if ok {
		t.Fatal("empty prunesh path must not rewrite")
	}
}

func TestRewritePipelineLastGrep(t *testing.T) {
	got, ok := Rewrite("cat foo | grep bar", "prunesh")
	if !ok || got != "cat foo | prunesh grep bar" {
		t.Fatalf("got %q ok=%v", got, ok)
	}
}

func TestRewritePipelineGitPass(t *testing.T) {
	_, ok := Rewrite("git status | head", "prunesh")
	if ok {
		t.Fatal("unsafe pipeline last stage must pass through")
	}
}

func TestRewriteAnd(t *testing.T) {
	got, ok := Rewrite("cd /tmp && git status", "prunesh")
	if !ok || got != "cd /tmp && prunesh git status" {
		t.Fatalf("got %q ok=%v", got, ok)
	}
}

func TestRewriteRedirectPass(t *testing.T) {
	_, ok := Rewrite("git status > /tmp/out", "prunesh")
	if ok {
		t.Fatal("redirects must pass through")
	}
}

func TestRewriteRegisteredModules(t *testing.T) {
	cases := []struct{ in, want string }{
		{"ls", "prunesh ls"},
		{"ls -la /tmp", "prunesh ls -la /tmp"},
		{"find . -name '*.go'", "prunesh find . -name '*.go'"},
		{"grep -n foo src", "prunesh grep -n foo src"},
		{"rg Error src", "prunesh rg Error src"},
		{"cat main.go", "prunesh cat main.go"},
		{"head -n 10 main.go", "prunesh head -n 10 main.go"},
		{"tree -L 2", "prunesh tree -L 2"},
	}
	for _, tc := range cases {
		got, ok := Rewrite(tc.in, "prunesh")
		if !ok || got != tc.want {
			t.Errorf("%q: got %q ok=%v want %q", tc.in, got, ok, tc.want)
		}
	}
}
