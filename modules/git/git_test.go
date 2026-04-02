package git

import (
	"fmt"
	"strings"
	"testing"
)

func TestFilterStatusStagedVsModified(t *testing.T) {
	// M at col 0 = staged modified → Staged
	// space at col 0 + M at col 1 = unstaged modified → Modified
	// A at col 0 = staged added → Staged
	// Input is large enough for the length guard to allow compression.
	var sb strings.Builder
	sb.WriteString("M  staged_file.go\n")
	sb.WriteString(" M unstaged_file.go\n")
	sb.WriteString("A  added_file.go\n")
	for i := 0; i < 30; i++ {
		fmt.Fprintf(&sb, " M internal/pkg/module_%02d/handler_with_long_name.go\n", i)
	}
	raw := sb.String()

	out := filterStatus(raw)

	if out == raw {
		t.Fatal("filterStatus returned original — input not large enough to trigger compression")
	}
	if !strings.Contains(out, "Staged") {
		t.Errorf("expected Staged section, got: %s", out)
	}
	if !strings.Contains(out, "staged_file.go") {
		t.Errorf("M at col 0 should be in Staged, got: %s", out)
	}
	if !strings.Contains(out, "added_file.go") {
		t.Errorf("A  should be in Staged, got: %s", out)
	}
	if !strings.Contains(out, "Modified") {
		t.Errorf("expected Modified section, got: %s", out)
	}
	if !strings.Contains(out, "unstaged_file.go") {
		t.Errorf("M at col 1 should be in Modified, got: %s", out)
	}
}

func TestFilterStatusClean(t *testing.T) {
	out := filterStatus("")
	if out != "nothing to commit, working tree clean\n" {
		t.Errorf("unexpected output for clean status: %q", out)
	}
}

func TestFilterStatusUntrackedTruncated(t *testing.T) {
	// Use long filenames so the formatted output is reliably shorter than raw,
	// ensuring the length guard does not block the truncation path.
	var sb strings.Builder
	for i := 0; i < 15; i++ {
		fmt.Fprintf(&sb, "?? internal/pkg/some/deeply/nested/module_%02d/handler.go\n", i)
	}
	out := filterStatus(sb.String())
	if out == sb.String() {
		t.Fatal("filterStatus returned original — length guard blocked compression, filenames may be too short")
	}
	if !strings.Contains(out, "+5 more") {
		t.Errorf("expected truncation at 10 untracked, got: %s", out)
	}
}

func TestFilterStatusLengthGuard(t *testing.T) {
	// small input: reformatted result may be longer → guard returns original
	raw := "M  a.go\n M b.go\nA  c.go\n"
	out := filterStatus(raw)
	// either the guard kicked in (returns raw) or result is shorter — never longer
	if len(out) > len(raw) {
		t.Errorf("filterStatus should never return output longer than input: %d > %d", len(out), len(raw))
	}
}
