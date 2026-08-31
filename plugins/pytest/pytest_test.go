package pytest

import (
	"fmt"
	"strings"
	"testing"

	"github.com/prunesh/prunesh/internal/registry"
)

func TestFilterSuccessKeepsSummary(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("============================= test session starts ==============================\n")
	sb.WriteString("collected 42 items\n")
	for i := 0; i < 42; i++ {
		fmt.Fprintf(&sb, "tests/test_%d.py .                                                             [ %2d%%]\n", i, (i+1)*100/42)
	}
	sb.WriteString("============================== 42 passed in 1.23s ==============================\n")

	out := Filter(sb.String(), 0)
	if !strings.Contains(out, "42 passed in 1.23s") {
		t.Fatalf("got %q", out)
	}
	if strings.Contains(out, "collected") || strings.Contains(out, "test_0") {
		t.Fatalf("noise should be removed, got %q", out)
	}
	if registry.EstimateTokens(out) >= registry.EstimateTokens(sb.String()) {
		t.Fatal("filtered output should use fewer tokens")
	}
}

func TestFilterFailureKeepsTracebackAndSummary(t *testing.T) {
	raw := `============================= test session starts ==============================
collected 2 items

tests/test_fail.py F.                                                            [100%]

=================================== FAILURES ===================================
__________________________________ test_broken __________________________________

    def test_broken():
>       assert False
E       AssertionError: assert False

tests/test_fail.py:2: AssertionError
=========================== short test summary info ============================
FAILED tests/test_fail.py::test_broken - AssertionError: assert False
========================= 1 failed, 1 passed in 0.05s =========================
`
	out := Filter(raw, 1)
	if !strings.Contains(out, "AssertionError") {
		t.Fatalf("expected traceback, got %q", out)
	}
	if !strings.Contains(out, "FAILED tests/test_fail.py::test_broken") {
		t.Fatalf("expected short summary, got %q", out)
	}
	if !strings.Contains(out, "1 failed, 1 passed") {
		t.Fatalf("expected result summary, got %q", out)
	}
	if strings.Contains(out, "collected 2 items") {
		t.Fatalf("noise should be removed, got %q", out)
	}
}

func TestModuleRewritePassthrough(t *testing.T) {
	m := &Module{}
	_, ok := m.Rewrite([]string{"-v"})
	if ok {
		t.Fatal("pytest must not rewrite")
	}
}
