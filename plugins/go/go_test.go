package gocmd

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/prunesh/prunesh/internal/registry"
)

func TestRewriteTestInjectsJSON(t *testing.T) {
	m := &Module{}
	got, ok := m.Rewrite([]string{"test", "./..."})
	if !ok {
		t.Fatal("expected rewrite")
	}
	want := []string{"test", "-json", "./..."}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestRewriteTestKeepsGlobals(t *testing.T) {
	m := &Module{}
	got, ok := m.Rewrite([]string{"-C", "/tmp", "test", "./..."})
	if !ok {
		t.Fatal("expected rewrite")
	}
	want := []string{"-C", "/tmp", "test", "-json", "./..."}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestRewriteTestSkipsJSON(t *testing.T) {
	m := &Module{}
	_, ok := m.Rewrite([]string{"test", "-json", "./..."})
	if ok {
		t.Fatal("user -json must not be rewritten")
	}
}

func TestRewriteTestSkipsBench(t *testing.T) {
	m := &Module{}
	_, ok := m.Rewrite([]string{"test", "-bench=.", "./..."})
	if ok {
		t.Fatal("bench must not inject -json")
	}
	_, ok = m.Rewrite([]string{"test", "-bench", "."})
	if ok {
		t.Fatal("-bench must not inject -json")
	}
}

func TestRewriteNonTest(t *testing.T) {
	m := &Module{}
	_, ok := m.Rewrite([]string{"build", "./..."})
	if ok {
		t.Fatal("build must not inject -json")
	}
}

func TestFilterTestJSONManyOkOneFail(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 40; i++ {
		pkg := fmt.Sprintf("example.com/ok%d", i)
		writeEvent(&sb, testEvent{Action: "pass", Package: pkg, Elapsed: 0.001})
	}
	writeEvent(&sb, testEvent{Action: "output", Package: "example.com/fail", Output: "--- FAIL: TestBroken (0.00s)\n"})
	writeEvent(&sb, testEvent{Action: "output", Package: "example.com/fail", Output: "    fail_test.go:10: boom\n"})
	writeEvent(&sb, testEvent{Action: "fail", Package: "example.com/fail", Elapsed: 0.002})

	m := &Module{}
	out := m.FilterOutput([]string{"test", "./..."}, sb.String(), 1)
	if !strings.Contains(out, "40 packages ok") {
		t.Fatalf("expected ok count, got %q", out)
	}
	if !strings.Contains(out, "FAIL\texample.com/fail") {
		t.Fatalf("expected FAIL package, got %q", out)
	}
	if !strings.Contains(out, "TestBroken") || !strings.Contains(out, "boom") {
		t.Fatalf("expected failure details, got %q", out)
	}
	if registry.EstimateTokens(out) >= registry.EstimateTokens(sb.String()) {
		t.Fatal("filtered output should use fewer tokens than raw JSON")
	}
}

func TestFilterTestClassicManyOkOneFail(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&sb, "ok  \texample.com/ok%d\t0.001s\n", i)
	}
	sb.WriteString("FAIL\texample.com/fail\t0.002s\n")
	sb.WriteString("--- FAIL: TestBroken (0.00s)\n")
	sb.WriteString("    fail_test.go:10: boom\n")
	sb.WriteString("FAIL\n")
	sb.WriteString("FAIL\texample.com/fail\t0.002s\n")

	m := &Module{}
	out := m.FilterOutput([]string{"test", "./..."}, sb.String(), 1)
	if !strings.Contains(out, "40 packages ok") {
		t.Fatalf("expected ok count, got %q", out)
	}
	if !strings.Contains(out, "TestBroken") {
		t.Fatalf("expected failure details, got %q", out)
	}
}

func TestFilterBuildVetSuccess(t *testing.T) {
	m := &Module{}
	out := m.FilterOutput([]string{"build", "./..."}, "", 0)
	if out != "ok\n" {
		t.Fatalf("got %q", out)
	}
	out = m.FilterOutput([]string{"vet", "./..."}, "", 0)
	if out != "ok\n" {
		t.Fatalf("got %q", out)
	}
}

func TestFilterBuildFailurePassthrough(t *testing.T) {
	raw := "# example.com/fail\n./main.go:1:1: syntax error\n"
	m := &Module{}
	out := m.FilterOutput([]string{"build", "./..."}, raw, 1)
	if !strings.Contains(out, "syntax error") {
		t.Fatalf("got %q", out)
	}
}

func writeEvent(sb *strings.Builder, ev testEvent) {
	b, err := json.Marshal(ev)
	if err != nil {
		panic(err)
	}
	sb.Write(b)
	sb.WriteByte('\n')
}
