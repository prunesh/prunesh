package main_test

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/prunesh/prunesh/internal/proxy"
	"github.com/prunesh/prunesh/internal/testhome"
)

func captureProxyStdout(t *testing.T, fn func() int) string {
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

func TestFilterInstallAndProxyDate(t *testing.T) {
	home := installTestDatePlugin(t)
	_ = home

	// Verify that testhome.Isolated isolated the home so the plugin is visible.
	_ = testhome.Isolated

	out := captureProxyStdout(t, func() int { return proxy.Run("date", nil) })
	if len(out) < 20 {
		t.Fatalf("expected compact ISO date, got %q", out)
	}
	if out[4] != '-' || out[7] != '-' {
		t.Fatalf("expected ISO-8601 format, got %q", out)
	}
}
