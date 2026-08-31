-- prunesh:when-table-exists filters
INSERT OR IGNORE INTO plugins
    SELECT id, module, version, argv0, contract, binary_path, manifest_path, installed_at
    FROM filters;
DROP TABLE IF EXISTS filters;
DROP INDEX IF EXISTS idx_filters_argv0;
