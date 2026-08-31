package pluginmanifest_test

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/prunesh/prunesh/internal/pluginmanifest"
)

func TestValidatePruneshCoreVersionMinPass(t *testing.T) {
	m := &pluginmanifest.Manifest{
		PruneshCoreVersion: pluginmanifest.PruneshCoreVersion{
			Version:    "0.10.0",
			Constraint: "min",
		},
	}
	if err := m.ValidatePruneshCoreVersion("0.10.0"); err != nil {
		t.Fatal(err)
	}
	if err := m.ValidatePruneshCoreVersion("0.11.0"); err != nil {
		t.Fatal(err)
	}
}

func TestValidatePruneshCoreVersionMinFail(t *testing.T) {
	m := &pluginmanifest.Manifest{
		PruneshCoreVersion: pluginmanifest.PruneshCoreVersion{
			Version:    "0.10.0",
			Constraint: "min",
		},
	}
	err := m.ValidatePruneshCoreVersion("0.9.0")
	if err == nil {
		t.Fatal("expected error for version below min")
	}
	if !strings.Contains(err.Error(), "< required min") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidatePruneshCoreVersionExactPass(t *testing.T) {
	m := &pluginmanifest.Manifest{
		PruneshCoreVersion: pluginmanifest.PruneshCoreVersion{
			Version:    "0.10.0",
			Constraint: "exact",
		},
	}
	if err := m.ValidatePruneshCoreVersion("0.10.0"); err != nil {
		t.Fatal(err)
	}
}

func TestValidatePruneshCoreVersionExactFail(t *testing.T) {
	m := &pluginmanifest.Manifest{
		PruneshCoreVersion: pluginmanifest.PruneshCoreVersion{
			Version:    "0.10.0",
			Constraint: "exact",
		},
	}
	err := m.ValidatePruneshCoreVersion("0.11.0")
	if err == nil {
		t.Fatal("expected error for exact mismatch")
	}
	if !strings.Contains(err.Error(), "!= required exact") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidatePruneshCoreVersionUnknownConstraint(t *testing.T) {
	m := &pluginmanifest.Manifest{
		PruneshCoreVersion: pluginmanifest.PruneshCoreVersion{
			Version:    "0.10.0",
			Constraint: "latest",
		},
	}
	err := m.ValidatePruneshCoreVersion("0.10.0")
	if err == nil {
		t.Fatal("expected error for unknown constraint")
	}
	if !strings.Contains(err.Error(), `unknown constraint "latest"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateCommandEmpty(t *testing.T) {
	m := &pluginmanifest.Manifest{
		ID:       "prunesh/date",
		Command:  "",
		Contract: "subprocess/v1",
		PruneshCoreVersion: pluginmanifest.PruneshCoreVersion{
			Version:    "0.11.0",
			Constraint: "min",
		},
		Platforms: []string{"linux/amd64"},
	}
	err := m.Validate("0.11.0", "linux/amd64")
	if err == nil {
		t.Fatal("expected error for empty command")
	}
	if !strings.Contains(err.Error(), "command must not be empty") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidatePruneshCoreVersionMinPrereleaseBase(t *testing.T) {
	m := &pluginmanifest.Manifest{
		PruneshCoreVersion: pluginmanifest.PruneshCoreVersion{
			Version:    "0.11.0",
			Constraint: "min",
		},
	}
	if err := m.ValidatePruneshCoreVersion("0.11.0-beta.2"); err != nil {
		t.Fatal(err)
	}
}

func TestValidatePruneshCoreVersionMinPrereleaseBelowBase(t *testing.T) {
	m := &pluginmanifest.Manifest{
		PruneshCoreVersion: pluginmanifest.PruneshCoreVersion{
			Version:    "0.11.0",
			Constraint: "min",
		},
	}
	err := m.ValidatePruneshCoreVersion("0.10.0-beta.1")
	if err == nil {
		t.Fatal("expected error for pre-release below required base")
	}
}

func TestParseDateManifest(t *testing.T) {
	// Requires github.com/prunesh/date to be published. Skip until marketplace migration completes.
	if _, err := downloadModuleDir("github.com/prunesh/date@v0.12.0"); err != nil {
		t.Skip("github.com/prunesh/date@v0.12.0 not yet published")
	}
	dir, err := downloadModuleDir("github.com/prunesh/date@v0.12.0")
	if err != nil {
		t.Fatal(err)
	}
	m, err := pluginmanifest.ParseFile(dir + "/prunesh.json")
	if err != nil {
		t.Fatal(err)
	}
	if m.ID != "prunesh/date" {
		t.Fatalf("id %q", m.ID)
	}
	if m.Command != "date" {
		t.Fatalf("command %q", m.Command)
	}
	if m.PruneshCoreVersion.Constraint != "min" {
		t.Fatalf("constraint %q", m.PruneshCoreVersion.Constraint)
	}
	if err := m.ValidatePruneshCoreVersion("0.11.0"); err != nil {
		t.Fatal(err)
	}
}

func downloadModuleDir(ref string) (string, error) {
	out, err := exec.Command("go", "mod", "download", "-json", ref).Output()
	if err != nil {
		return "", err
	}
	var info struct {
		Dir string `json:"Dir"`
	}
	if err := json.Unmarshal(out, &info); err != nil {
		return "", err
	}
	if info.Dir == "" {
		return "", fmt.Errorf("go mod download returned empty dir for %s", ref)
	}
	return info.Dir, nil
}
