package npmtest

import (
	"strings"
	"testing"

	"github.com/prunesh/prunesh/internal/registry"
)

func TestFilterJestSuccess(t *testing.T) {
	raw := `> project@1.0.0 test
> jest

PASS src/a.test.ts
PASS src/b.test.ts
PASS src/c.test.ts

Test Suites: 3 passed, 3 total
Tests:       15 passed, 15 total
Snapshots:   0 total
Time:        2.345 s
`
	out := Filter(raw, 0)
	if !strings.Contains(out, "Test Suites: 3 passed") || !strings.Contains(out, "Tests:       15 passed") {
		t.Fatalf("got %q", out)
	}
	if strings.Contains(out, "PASS src/a") {
		t.Fatalf("pass lines should be removed, got %q", out)
	}
	if registry.EstimateTokens(out) >= registry.EstimateTokens(raw) {
		t.Fatal("filtered output should use fewer tokens")
	}
}

func TestFilterJestFailure(t *testing.T) {
	raw := `FAIL src/broken.test.ts
  ● adds numbers
    expect(received).toBe(expected)

Test Suites: 1 failed, 2 passed, 3 total
Tests:       1 failed, 14 passed, 15 total
`
	out := Filter(raw, 1)
	if !strings.Contains(out, "FAIL src/broken.test.ts") || !strings.Contains(out, "expect(received)") {
		t.Fatalf("got %q", out)
	}
	if !strings.Contains(out, "1 failed, 2 passed") {
		t.Fatalf("expected summary, got %q", out)
	}
}

func TestIsPackageTest(t *testing.T) {
	if !IsPackageTest([]string{"test"}) {
		t.Fatal("npm test")
	}
	if !IsPackageTest([]string{"run", "test"}) {
		t.Fatal("npm run test")
	}
	if IsPackageTest([]string{"install"}) {
		t.Fatal("npm install must not match")
	}
}

func TestIsNpxTest(t *testing.T) {
	if !IsNpxTest([]string{"vitest", "run"}) {
		t.Fatal("npx vitest")
	}
	if !IsNpxTest([]string{"jest"}) {
		t.Fatal("npx jest")
	}
	if IsNpxTest([]string{"create-react-app"}) {
		t.Fatal("npx create-react-app must not match")
	}
}

func TestNpmPassthroughNonTest(t *testing.T) {
	m := &npmModule{}
	raw := "added 1 package\n"
	out := m.FilterOutput([]string{"install", "lodash"}, raw, 0)
	if out != raw {
		t.Fatalf("got %q", out)
	}
}
