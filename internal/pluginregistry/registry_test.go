package pluginregistry_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/prunesh/prunesh/internal/pluginregistry"
	_ "modernc.org/sqlite"
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
		t.Fatalf("active plugin: %+v", active)
	}
	got, err := db.Get(rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.ID != rec.ID {
		t.Fatalf("get plugin: %+v", got)
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
		t.Fatalf("expected no active plugin, got %+v", active)
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
		t.Fatalf("active plugin: %+v", active)
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
		t.Fatal("expected error for missing plugin")
	}
}

func TestMigrateFromFiltersTable(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Build a pre-migration DB that only has the old `filters` table.
	dbDir := filepath.Join(home, ".prunesh")
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		t.Fatal(err)
	}
	legacyDB, err := sql.Open("sqlite", filepath.Join(dbDir, "plugins.db"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = legacyDB.Exec(`
		CREATE TABLE filters (
			id TEXT PRIMARY KEY, module TEXT NOT NULL, version TEXT NOT NULL,
			argv0 TEXT NOT NULL, contract TEXT NOT NULL,
			binary_path TEXT NOT NULL, manifest_path TEXT NOT NULL,
			installed_at INTEGER NOT NULL
		);
		INSERT INTO filters VALUES ('prunesh/date','github.com/prunesh/date','v0.1.0','date','stdin/v1','/tmp/date','/tmp/prunesh.json',1000);
	`)
	legacyDB.Close()
	if err != nil {
		t.Fatal(err)
	}

	db, err := pluginregistry.Open()
	if err != nil {
		t.Fatalf("Open after legacy seed: %v", err)
	}
	defer db.Close()

	recs, err := db.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || recs[0].ID != "prunesh/date" {
		t.Fatalf("expected 1 migrated plugin, got %+v", recs)
	}
}

func TestDirtyMigrationIsReapplied(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Simulate a crash: DB has the plugins table but migration 0001 is marked dirty.
	dbDir := filepath.Join(home, ".prunesh")
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		t.Fatal(err)
	}
	rawDB, err := sql.Open("sqlite", filepath.Join(dbDir, "plugins.db"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = rawDB.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY, checksum TEXT NOT NULL DEFAULT '',
			dirty INTEGER NOT NULL DEFAULT 0,
			applied_at TEXT NOT NULL DEFAULT (datetime('now'))
		);
		CREATE TABLE IF NOT EXISTS plugins (
			id TEXT PRIMARY KEY, module TEXT NOT NULL, version TEXT NOT NULL,
			argv0 TEXT NOT NULL, contract TEXT NOT NULL,
			binary_path TEXT NOT NULL, manifest_path TEXT NOT NULL,
			installed_at INTEGER NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_plugins_argv0 ON plugins(argv0, installed_at);
		INSERT INTO schema_migrations (version, checksum, dirty) VALUES ('0001', 'sha256:dummy', 1);
	`)
	rawDB.Close()
	if err != nil {
		t.Fatal(err)
	}

	// Open should recover: re-apply the dirty migration (idempotent) and clear dirty.
	db, err := pluginregistry.Open()
	if err != nil {
		t.Fatalf("Open after dirty migration: %v", err)
	}
	db.Close()
}

func TestChecksumMismatchErrors(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Simulate a migration that was applied with a different checksum.
	dbDir := filepath.Join(home, ".prunesh")
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		t.Fatal(err)
	}
	rawDB, err := sql.Open("sqlite", filepath.Join(dbDir, "plugins.db"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = rawDB.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY, checksum TEXT NOT NULL DEFAULT '',
			dirty INTEGER NOT NULL DEFAULT 0,
			applied_at TEXT NOT NULL DEFAULT (datetime('now'))
		);
		CREATE TABLE IF NOT EXISTS plugins (
			id TEXT PRIMARY KEY, module TEXT NOT NULL, version TEXT NOT NULL,
			argv0 TEXT NOT NULL, contract TEXT NOT NULL,
			binary_path TEXT NOT NULL, manifest_path TEXT NOT NULL,
			installed_at INTEGER NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_plugins_argv0 ON plugins(argv0, installed_at);
		INSERT INTO schema_migrations (version, checksum, dirty) VALUES ('0001', 'sha256:wrong', 0);
		INSERT INTO schema_migrations (version, checksum, dirty) VALUES ('0002', 'sha256:wrong', 0);
	`)
	rawDB.Close()
	if err != nil {
		t.Fatal(err)
	}

	_, err = pluginregistry.Open()
	if err == nil {
		t.Fatal("expected checksum mismatch error")
	}
	if !strings.Contains(err.Error(), "checksum changed") {
		t.Fatalf("unexpected error: %v", err)
	}
}
