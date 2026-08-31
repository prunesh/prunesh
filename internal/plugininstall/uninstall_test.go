package plugininstall_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/prunesh/prunesh/internal/plugininstall"
	"github.com/prunesh/prunesh/internal/pluginmanifest"
	"github.com/prunesh/prunesh/internal/pluginregistry"
	"github.com/prunesh/prunesh/internal/testhome"
)

// minimalPluginSrc is a stdin/v1 compliant binary for tests.
const minimalPluginSrc = `package main

import (
	"encoding/json"
	"os"
)

func main() {
	var req struct {
		Operation string   ` + "`" + `json:"operation"` + "`" + `
		Args      []string ` + "`" + `json:"args"` + "`" + `
		Output    string   ` + "`" + `json:"output"` + "`" + `
		ExitCode  int      ` + "`" + `json:"exit_code"` + "`" + `
	}
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		os.Exit(1)
	}
	resp := map[string]interface{}{
		"args":    req.Args,
		"changed": false,
		"output":  req.Output,
	}
	json.NewEncoder(os.Stdout).Encode(resp)
}
`

// buildLocalPlugin compiles a stub binary and writes prunesh.json to dir,
// returning the dir path so it can be passed as Options.LocalDir.
func buildLocalPlugin(t *testing.T, id, argv0 string) string {
	t.Helper()
	dir := t.TempDir()

	srcFile := filepath.Join(dir, "main.go")
	if err := os.WriteFile(srcFile, []byte(minimalPluginSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	binName := filepath.Base(id)
	binPath := filepath.Join(dir, binName)
	out, err := exec.Command("go", "build", "-o", binPath, srcFile).CombinedOutput()
	if err != nil {
		t.Fatalf("build stub %s: %v\n%s", id, err, out)
	}

	manifest := pluginmanifest.Manifest{
		ID:       id,
		Command:  argv0,
		Contract: "stdin/v1",
		Platforms: []string{
			fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		},
		PruneshCoreVersion: pluginmanifest.PruneshCoreVersion{
			Version:    "0.1.0",
			Constraint: "min",
		},
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, pluginmanifest.ManifestFileName), data, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stderr
	os.Stderr = w
	fn()
	_ = w.Close()
	os.Stderr = old
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	_ = r.Close()
	return buf.String()
}

func TestInstallConflictAbortsWithoutReplace(t *testing.T) {
	testhome.Isolated(t)

	// Install first plugin for argv0 "date".
	localDir := buildLocalPlugin(t, "acme/date", "date")
	_, err := plugininstall.Install(plugininstall.Options{
		Module:      "github.com/acme/date",
		Version:     "v0.1.0",
		CoreVersion: "0.11.0",
		LocalDir:    localDir,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Attempt to install a second plugin for the same argv0 without --replace.
	localDir2 := buildLocalPlugin(t, "prunesh/date", "date")
	_, err = plugininstall.Install(plugininstall.Options{
		Module:      "github.com/prunesh/date",
		Version:     "v0.2.0",
		CoreVersion: "0.11.0",
		LocalDir:    localDir2,
	})
	if err == nil {
		t.Fatal("expected install to abort without --replace")
	}
	if !strings.Contains(err.Error(), "acme/date") || !strings.Contains(err.Error(), "--replace") {
		t.Fatalf("unexpected error: %v", err)
	}

	db, err := pluginregistry.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	active, err := db.Active("date")
	if err != nil {
		t.Fatal(err)
	}
	if active == nil || active.ID != "acme/date" {
		t.Fatalf("active filter must remain unchanged: %+v", active)
	}
}

func TestInstallConflictWithReplace(t *testing.T) {
	testhome.Isolated(t)

	localDir := buildLocalPlugin(t, "acme/date", "date")
	if _, err := plugininstall.Install(plugininstall.Options{
		Module:      "github.com/acme/date",
		Version:     "v0.1.0",
		CoreVersion: "0.11.0",
		LocalDir:    localDir,
	}); err != nil {
		t.Fatal(err)
	}

	localDir2 := buildLocalPlugin(t, "prunesh/date", "date")
	stderr := captureStderr(t, func() {
		if _, err := plugininstall.Install(plugininstall.Options{
			Module:      "github.com/prunesh/date",
			Version:     "v0.2.0",
			CoreVersion: "0.11.0",
			LocalDir:    localDir2,
			Replace:     true,
		}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(stderr, "replacing active filter") {
		t.Fatalf("expected replace notice on stderr, got %q", stderr)
	}

	db, err := pluginregistry.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	active, err := db.Active("date")
	if err != nil {
		t.Fatal(err)
	}
	if active == nil || active.ID != "prunesh/date" {
		t.Fatalf("active filter: %+v", active)
	}
	got, err := db.Get("acme/date")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("previous filter must remain installed but inactive")
	}
}

func TestUninstallRemovesInstallDir(t *testing.T) {
	testhome.Isolated(t)

	localDir := buildLocalPlugin(t, "prunesh/date", "date")
	rec, err := plugininstall.Install(plugininstall.Options{
		Module:      "github.com/prunesh/date",
		Version:     "v0.2.0",
		CoreVersion: "0.11.0",
		LocalDir:    localDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	installDir := filepath.Dir(rec.BinaryPath)
	if _, err := os.Stat(installDir); err != nil {
		t.Fatalf("install dir missing before uninstall: %v", err)
	}

	removed, err := plugininstall.Uninstall(rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if removed.ID != rec.ID {
		t.Fatalf("removed id %q", removed.ID)
	}
	if _, err := os.Stat(installDir); !os.IsNotExist(err) {
		t.Fatalf("install dir still present after uninstall: %v", err)
	}

	db, err := pluginregistry.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	active, err := db.Active("date")
	if err != nil {
		t.Fatal(err)
	}
	if active != nil {
		t.Fatalf("expected no active filter, got %+v", active)
	}
}

func TestUninstallPromotesPreviousFilter(t *testing.T) {
	testhome.Isolated(t)

	olderDir := buildLocalPlugin(t, "acme/date", "date")
	older, err := plugininstall.Install(plugininstall.Options{
		Module:      "github.com/acme/date",
		Version:     "v0.1.0",
		CoreVersion: "0.11.0",
		LocalDir:    olderDir,
	})
	if err != nil {
		t.Fatal(err)
	}

	newerDir := buildLocalPlugin(t, "prunesh/date", "date")
	newer, err := plugininstall.Install(plugininstall.Options{
		Module:      "github.com/prunesh/date",
		Version:     "v0.2.0",
		CoreVersion: "0.11.0",
		LocalDir:    newerDir,
		Replace:     true,
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := plugininstall.Uninstall(newer.ID); err != nil {
		t.Fatal(err)
	}

	db, err := pluginregistry.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	active, err := db.Active("date")
	if err != nil {
		t.Fatal(err)
	}
	if active == nil || active.ID != older.ID {
		t.Fatalf("active filter: %+v", active)
	}
}
