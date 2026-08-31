// Package pluginregistry persists installed external plugins in ~/.prunesh/plugins.db.
package pluginregistry

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/prunesh/prunesh/internal/storage"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

var migrationFileRe = regexp.MustCompile(`^(\d{4})-[a-z0-9][a-z0-9-]*\.sql$`)

const metaDDL = `CREATE TABLE IF NOT EXISTS schema_migrations (
    version    TEXT PRIMARY KEY,
    applied_at TEXT NOT NULL DEFAULT (datetime('now'))
);`

// Record is one installed plugin.
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

// DB manages the plugins database.
type DB struct {
	db *sql.DB
}

// Open opens or creates ~/.prunesh/plugins.db and applies pending migrations.
func Open() (*DB, error) {
	dir, err := storage.Dir()
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", filepath.Join(dir, "plugins.db"))
	if err != nil {
		return nil, err
	}
	if err := applyMigrations(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate plugins.db: %w", err)
	}
	return &DB{db: db}, nil
}

func (d *DB) Close() { d.db.Close() }

// Install records a plugin, replacing any previous install with the same id.
func (d *DB) Install(rec Record) error {
	if rec.ID == "" || rec.Module == "" || rec.Version == "" || rec.Argv0 == "" {
		return fmt.Errorf("incomplete plugin record")
	}
	if rec.BinaryPath == "" || rec.ManifestPath == "" {
		return fmt.Errorf("incomplete plugin record")
	}
	_, err := d.db.Exec(`
		INSERT INTO plugins (id, module, version, argv0, contract, binary_path, manifest_path, installed_at)
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

// Get returns one installed plugin by id.
func (d *DB) Get(id string) (*Record, error) {
	if id == "" {
		return nil, fmt.Errorf("id is empty")
	}
	row := d.db.QueryRow(`
		SELECT id, module, version, argv0, contract, binary_path, manifest_path, installed_at
		FROM plugins WHERE id = ?
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

// Uninstall removes a plugin by id.
func (d *DB) Uninstall(id string) (*Record, error) {
	rec, err := d.Get(id)
	if err != nil {
		return nil, err
	}
	if rec == nil {
		return nil, fmt.Errorf("plugin %q is not installed", id)
	}
	if _, err := d.db.Exec(`DELETE FROM plugins WHERE id = ?`, id); err != nil {
		return nil, err
	}
	return rec, nil
}

// Active returns the most recently installed plugin for argv0.
func (d *DB) Active(argv0 string) (*Record, error) {
	row := d.db.QueryRow(`
		SELECT id, module, version, argv0, contract, binary_path, manifest_path, installed_at
		FROM plugins WHERE argv0 = ? ORDER BY installed_at DESC LIMIT 1
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

// HasActive reports whether an external plugin is installed for argv0.
func HasActive(argv0 string) bool {
	db, err := Open()
	if err != nil {
		return false
	}
	defer db.Close()
	rec, err := db.Active(argv0)
	return err == nil && rec != nil
}

// List returns all installed plugins ordered by argv0 and install time.
func (d *DB) List() ([]Record, error) {
	rows, err := d.db.Query(`
		SELECT id, module, version, argv0, contract, binary_path, manifest_path, installed_at
		FROM plugins ORDER BY argv0, installed_at DESC
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

// ── Migration runner ──────────────────────────────────────────────────────────

type migrationFile struct {
	version string
	content string
}

func applyMigrations(db *sql.DB) error {
	if _, err := db.Exec(metaDDL); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	files, err := loadMigrationFiles()
	if err != nil {
		return err
	}
	for _, f := range files {
		var count int
		if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, f.version).Scan(&count); err != nil {
			return fmt.Errorf("check migration %s: %w", f.version, err)
		}
		if count > 0 {
			continue
		}
		shouldRun, err := guardPasses(db, f.content)
		if err != nil {
			return fmt.Errorf("migration %s guard: %w", f.version, err)
		}
		if shouldRun {
			tx, err := db.Begin()
			if err != nil {
				return fmt.Errorf("begin migration %s: %w", f.version, err)
			}
			if _, err := tx.Exec(f.content); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("apply migration %s: %w", f.version, err)
			}
			if _, err := tx.Exec(`INSERT INTO schema_migrations (version) VALUES (?)`, f.version); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("record migration %s: %w", f.version, err)
			}
			if err := tx.Commit(); err != nil {
				return fmt.Errorf("commit migration %s: %w", f.version, err)
			}
		} else {
			if _, err := db.Exec(`INSERT INTO schema_migrations (version) VALUES (?)`, f.version); err != nil {
				return fmt.Errorf("record skipped migration %s: %w", f.version, err)
			}
		}
	}
	return nil
}

func loadMigrationFiles() ([]migrationFile, error) {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("read embedded migrations: %w", err)
	}
	var files []migrationFile
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		if !migrationFileRe.MatchString(entry.Name()) {
			return nil, fmt.Errorf("invalid migration filename %q", entry.Name())
		}
		content, err := fs.ReadFile(migrationsFS, "migrations/"+entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}
		files = append(files, migrationFile{version: entry.Name()[:4], content: string(content)})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].version < files[j].version })
	return files, nil
}

// guardPasses evaluates the optional -- prunesh:when-table-exists <table> guard.
// If no guard is present, the migration always runs.
func guardPasses(db *sql.DB, content string) (bool, error) {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "-- prunesh:when-table-exists ") {
			continue
		}
		table := strings.TrimSpace(strings.TrimPrefix(line, "-- prunesh:when-table-exists "))
		if table == "" {
			return false, fmt.Errorf("invalid guard: empty table name")
		}
		var count int
		_ = db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&count)
		return count > 0, nil
	}
	return true, nil
}
