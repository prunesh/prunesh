package plugininstall

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/prunesh/prunesh/internal/pluginregistry"
)

// Uninstall removes a filter by full id and deletes its install directory.
func Uninstall(id string) (*pluginregistry.Record, error) {
	if id == "" {
		return nil, fmt.Errorf("id is empty")
	}
	db, err := pluginregistry.Open()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rec, err := db.Uninstall(id)
	if err != nil {
		return nil, err
	}

	installDir := filepath.Dir(rec.BinaryPath)
	if err := os.RemoveAll(installDir); err != nil {
		return rec, fmt.Errorf("remove %s: %w", installDir, err)
	}
	return rec, nil
}
