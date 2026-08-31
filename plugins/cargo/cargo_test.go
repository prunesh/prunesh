package cargo

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/prunesh/prunesh/internal/registry"
)

func TestRewriteTestInjectsQuiet(t *testing.T) {
	m := &Module{}
	got, ok := m.Rewrite([]string{"test"})
	if !ok {
		t.Fatal("expected rewrite")
	}
	want := []string{"test", "-q"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestRewriteBuildClippyCheck(t *testing.T) {
	m := &Module{}
	for _, sub := range []string{"build", "check", "clippy"} {
		got, ok := m.Rewrite([]string{sub})
		if !ok {
			t.Fatalf("%s: expected rewrite", sub)
		}
		want := []string{sub, "-q"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: got %v want %v", sub, got, want)
		}
	}
}

func TestRewriteSkipsVerbose(t *testing.T) {
	m := &Module{}
	_, ok := m.Rewrite([]string{"test", "-v"})
	if ok {
		t.Fatal("verbose must not rewrite")
	}
	_, ok = m.Rewrite([]string{"-v", "test"})
	if ok {
		t.Fatal("global verbose must not rewrite")
	}
}

func TestRewriteSkipsQuiet(t *testing.T) {
	m := &Module{}
	_, ok := m.Rewrite([]string{"test", "-q"})
	if ok {
		t.Fatal("already quiet must not rewrite")
	}
}

func TestRewriteNonHandled(t *testing.T) {
	m := &Module{}
	_, ok := m.Rewrite([]string{"run"})
	if ok {
		t.Fatal("run must not rewrite")
	}
}

func TestFilterSuccessCollapsesCompiling(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 20; i++ {
		fmt.Fprintf(&sb, "   Compiling crate%d v0.1.0\n", i)
	}
	sb.WriteString("    Finished `dev` profile [unoptimized + debuginfo] target(s) in 5.00s\n")
	sb.WriteString("     Running unittests src/lib.rs (target/debug/deps/foo)\n")
	sb.WriteString("running 10 tests\n")
	for i := 0; i < 10; i++ {
		fmt.Fprintf(&sb, "test test_%d ... ok\n", i)
	}
	sb.WriteString("test result: ok. 10 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.01s\n")

	m := &Module{}
	out := m.FilterOutput([]string{"test"}, sb.String(), 0)
	if !strings.Contains(out, "Compiling 20 crates") {
		t.Fatalf("expected compile count, got %q", out)
	}
	if strings.Contains(out, "test test_0") {
		t.Fatalf("passing test lines should be collapsed, got %q", out)
	}
	if !strings.Contains(out, "test result: ok. 10 passed") {
		t.Fatalf("expected test result, got %q", out)
	}
	if registry.EstimateTokens(out) >= registry.EstimateTokens(sb.String()) {
		t.Fatal("filtered output should use fewer tokens")
	}
}

func TestFilterFailureKeepsErrors(t *testing.T) {
	raw := `   Compiling foo v0.1.0
   Compiling bar v0.1.0
error[E0425]: cannot find value ` + "`x`" + ` in this scope
 --> src/main.rs:1:5
  |
1 |     x
  |     ^ not found in this scope

error: test failed, to rerun pass ` + "`--bin foo`" + `
`
	m := &Module{}
	out := m.FilterOutput([]string{"test"}, raw, 101)
	if !strings.Contains(out, "Compiling 2 crates") {
		t.Fatalf("expected compile count, got %q", out)
	}
	if !strings.Contains(out, "error[E0425]") || !strings.Contains(out, "not found in this scope") {
		t.Fatalf("expected error details, got %q", out)
	}
}

func TestFilterBuildSuccessMinimal(t *testing.T) {
	raw := "   Compiling foo v0.1.0\n    Finished `dev` profile [unoptimized + debuginfo] target(s) in 1.00s\n"
	m := &Module{}
	out := m.FilterOutput([]string{"build"}, raw, 0)
	if !strings.Contains(out, "Compiling 1 crates") || !strings.Contains(out, "Finished") {
		t.Fatalf("got %q", out)
	}
}
