CREATE TABLE IF NOT EXISTS plugins (
    id            TEXT PRIMARY KEY,
    module        TEXT NOT NULL,
    version       TEXT NOT NULL,
    argv0         TEXT NOT NULL,
    contract      TEXT NOT NULL,
    binary_path   TEXT NOT NULL,
    manifest_path TEXT NOT NULL,
    installed_at  INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_plugins_argv0 ON plugins(argv0, installed_at);
