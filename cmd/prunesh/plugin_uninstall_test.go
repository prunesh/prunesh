package main_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/prunesh/prunesh/internal/testhome"
)

func TestFilterUninstallCLI(t *testing.T) {
	testhome.Isolated(t)

	// Install the local test date plugin.
	home := installTestDatePlugin(t)
	_ = home

	bin := buildBinary(t)
	cmd := exec.Command(bin, "plugin", "uninstall", "prunesh/date")
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("plugin uninstall: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "uninstalled prunesh/date") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestFilterListMarksActive(t *testing.T) {
	home := installTestDatePlugin(t)
	_ = home

	bin := buildBinary(t)
	cmd := exec.Command(bin, "plugin", "list")
	cmd.Env = os.Environ()
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	if !strings.Contains(text, "prunesh/date") {
		t.Fatalf("expected plugin in list: %s", text)
	}
	if !strings.Contains(text, "active") {
		t.Fatalf("expected active marker in list: %s", text)
	}
}
