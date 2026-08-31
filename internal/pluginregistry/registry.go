// Package pluginregistry persists installed external plugins in ~/.prunesh/plugins.db.
package pluginregistry

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"time"

	"github.com/prunesh/prunesh/internal/storage"
	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS filters (
	id           TEXT PRIMARY KEY,
	module       TEXT NOT NULL,
	version      TEXT NOT NULL,
	argv0        TEXT NOT NULL,
	contract     TEXT NOT NULL,
	binary_path  TEXT NOT NULL,
	manifest_path TEXT NOT NULL,
	installed_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_filters_argv0 ON filters(argv0, installed_at);
`

// Record is one installed filter.
type Record struct {
	ID           string
	Module       string
	Version      string
	Argv0        string
	Contract     string
	BinaryPath   string
	ManifestPath string
	InstalledAt  time.Time
}

// DB manages the filters database.
type DB struct {
	db *sql.DB
}

// Open opens or creates ~/.prunesh/plugins.db.
func Open() (*DB, error) {
	dir, err := storage.Dir()
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", filepath.Join(dir, "plugins.db"))
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	return &DB{db: db}, nil
}

func (d *DB) Close() { d.db.Close() }

// Install records a filter, replacing any previous install with the same id.
func (d *DB) Install(rec Record) error {
	if rec.ID == "" || rec.Module == "" || rec.Version == "" || rec.Argv0 == "" {
		return fmt.Errorf("incomplete filter record")
	}
	if rec.BinaryPath == "" || rec.ManifestPath == "" {
		return fmt.Errorf("incomplete filter record")
	}
	_, err := d.db.Exec(`
		INSERT INTO filters (id, module, version, argv0, contract, binary_path, manifest_path, installed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			module=excluded.module,
			version=excluded.version,
			argv0=excluded.argv0,
			contract=excluded.contract,
			binary_path=excluded.binary_path,
			manifest_path=excluded.manifest_path,
			installed_at=excluded.installed_at
	`, rec.ID, rec.Module, rec.Version, rec.Argv0, rec.Contract, rec.BinaryPath, rec.ManifestPath, rec.InstalledAt.Unix())
	return err
}

// Get returns one installed filter by id.
func (d *DB) Get(id string) (*Record, error) {
	if id == "" {
		return nil, fmt.Errorf("id is empty")
	}
	row := d.db.QueryRow(`
		SELECT id, module, version, argv0, contract, binary_path, manifest_path, installed_at
		FROM filters WHERE id = ?
	`, id)
	var rec Record
	var ts int64
	if err := row.Scan(&rec.ID, &rec.Module, &rec.Version, &rec.Argv0, &rec.Contract, &rec.BinaryPath, &rec.ManifestPath, &ts); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	rec.InstalledAt = time.Unix(ts, 0)
	return &rec, nil
}

// Uninstall removes a filter by id.
func (d *DB) Uninstall(id string) (*Record, error) {
	rec, err := d.Get(id)
	if err != nil {
		return nil, err
	}
	if rec == nil {
		return nil, fmt.Errorf("filter %q is not installed", id)
	}
	if _, err := d.db.Exec(`DELETE FROM filters WHERE id = ?`, id); err != nil {
		return nil, err
	}
	return rec, nil
}

// Active returns the most recently installed filter for argv0.
func (d *DB) Active(argv0 string) (*Record, error) {
	row := d.db.QueryRow(`
		SELECT id, module, version, argv0, contract, binary_path, manifest_path, installed_at
		FROM filters WHERE argv0 = ? ORDER BY installed_at DESC LIMIT 1
	`, argv0)
	var rec Record
	var ts int64
	if err := row.Scan(&rec.ID, &rec.Module, &rec.Version, &rec.Argv0, &rec.Contract, &rec.BinaryPath, &rec.ManifestPath, &ts); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	rec.InstalledAt = time.Unix(ts, 0)
	return &rec, nil
}

// HasActive reports whether an external filter is installed for argv0.
func HasActive(argv0 string) bool {
	db, err := Open()
	if err != nil {
		return false
	}
	defer db.Close()
	rec, err := db.Active(argv0)
	return err == nil && rec != nil
}

// List returns all installed filters ordered by argv0 and install time.
func (d *DB) List() ([]Record, error) {
	rows, err := d.db.Query(`
		SELECT id, module, version, argv0, contract, binary_path, manifest_path, installed_at
		FROM filters ORDER BY argv0, installed_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Record
	for rows.Next() {
		var rec Record
		var ts int64
		if err := rows.Scan(&rec.ID, &rec.Module, &rec.Version, &rec.Argv0, &rec.Contract, &rec.BinaryPath, &rec.ManifestPath, &ts); err != nil {
			return nil, err
		}
		rec.InstalledAt = time.Unix(ts, 0)
		out = append(out, rec)
	}
	return out, rows.Err()
}
