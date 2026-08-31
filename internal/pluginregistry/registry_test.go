package pluginregistry_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/prunesh/prunesh/internal/pluginregistry"
)

func TestInstallAndActive(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	db, err := pluginregistry.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	rec := pluginregistry.Record{
		ID:           "prunesh/date",
		Module:       "github.com/prunesh/date",
		Version:      "v0.11.0",
		Argv0:        "date",
		Contract:     "subprocess/v1",
		BinaryPath:   filepath.Join(home, "date"),
		ManifestPath: filepath.Join(home, "prunesh.json"),
		InstalledAt:  time.Now(),
	}
	if err := db.Install(rec); err != nil {
		t.Fatal(err)
	}
	active, err := db.Active("date")
	if err != nil {
		t.Fatal(err)
	}
	if active == nil || active.ID != rec.ID {
		t.Fatalf("active filter: %+v", active)
	}
	got, err := db.Get(rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.ID != rec.ID {
		t.Fatalf("get filter: %+v", got)
	}
}

func TestUninstallRemovesFilter(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	db, err := pluginregistry.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	rec := pluginregistry.Record{
		ID:           "prunesh/date",
		Module:       "github.com/prunesh/date",
		Version:      "v0.11.0",
		Argv0:        "date",
		Contract:     "subprocess/v1",
		BinaryPath:   filepath.Join(home, "date"),
		ManifestPath: filepath.Join(home, "prunesh.json"),
		InstalledAt:  time.Now(),
	}
	if err := db.Install(rec); err != nil {
		t.Fatal(err)
	}
	removed, err := db.Uninstall(rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if removed.ID != rec.ID {
		t.Fatalf("removed id %q", removed.ID)
	}
	active, err := db.Active("date")
	if err != nil {
		t.Fatal(err)
	}
	if active != nil {
		t.Fatalf("expected no active filter, got %+v", active)
	}
}

func TestUninstallPromotesPreviousActive(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	db, err := pluginregistry.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	older := pluginregistry.Record{
		ID:           "acme/date",
		Module:       "github.com/acme/date",
		Version:      "v0.1.0",
		Argv0:        "date",
		Contract:     "subprocess/v1",
		BinaryPath:   filepath.Join(home, "acme-date"),
		ManifestPath: filepath.Join(home, "acme-prunesh.json"),
		InstalledAt:  time.Now().Add(-time.Hour),
	}
	newer := pluginregistry.Record{
		ID:           "prunesh/date",
		Module:       "github.com/prunesh/date",
		Version:      "v0.11.0",
		Argv0:        "date",
		Contract:     "subprocess/v1",
		BinaryPath:   filepath.Join(home, "date"),
		ManifestPath: filepath.Join(home, "prunesh.json"),
		InstalledAt:  time.Now(),
	}
	if err := db.Install(older); err != nil {
		t.Fatal(err)
	}
	if err := db.Install(newer); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Uninstall(newer.ID); err != nil {
		t.Fatal(err)
	}
	active, err := db.Active("date")
	if err != nil {
		t.Fatal(err)
	}
	if active == nil || active.ID != older.ID {
		t.Fatalf("active filter: %+v", active)
	}
}

func TestUninstallMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	db, err := pluginregistry.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.Uninstall("missing/date"); err == nil {
		t.Fatal("expected error for missing filter")
	}
}
