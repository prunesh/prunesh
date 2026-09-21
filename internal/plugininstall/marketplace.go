package plugininstall

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/prunesh/prunesh/internal/pluginmanifest"
)

const defaultMarketplaceBase = "https://prunesh.github.io/marketplace"

// marketplaceVersionFile is the schema of site/modules/<author>/<command>/<version>.json.
type marketplaceVersionFile struct {
	ID         string            `json:"id"`
	Command    string            `json:"command"`
	Version    string            `json:"version"`
	Repo       string            `json:"repo"`
	BinaryURLs map[string]string `json:"binary_urls"` // platform -> download URL
	Platforms  []string          `json:"platforms"`
	Contract   string            `json:"contract"`
	PruneshCoreVersion struct {
		Version    string `json:"version"`
		Constraint string `json:"constraint"`
	} `json:"prunesh_core_version"`
	PublishedAt string `json:"published_at"`
}

type marketplaceIndex struct {
	Latest string `json:"latest"`
}

// isMarketplaceRef returns true for author/command refs (no dots before the slash).
// Go module refs always contain dots (github.com, golang.org, …).
func isMarketplaceRef(module string) bool {
	parts := strings.SplitN(module, "/", 2)
	return len(parts) == 2 && !strings.Contains(parts[0], ".")
}

func marketplaceBase() string {
	if v := os.Getenv("PRUNESH_MARKETPLACE_URL"); v != "" {
		return strings.TrimRight(v, "/")
	}
	return defaultMarketplaceBase
}

// resolveFromMarketplace fetches the version file from the marketplace, downloads the
// platform binary, writes a temporary directory with a synthetic prunesh.json, and
// returns (srcDir, binaryPath, resolvedVersion).
func resolveFromMarketplace(opts Options, platform string) (srcDir, binary, resolvedVersion string, err error) {
	author, command, ok := strings.Cut(opts.Module, "/")
	if !ok {
		return "", "", "", fmt.Errorf("invalid marketplace ref %q", opts.Module)
	}

	base := marketplaceBase()
	version := opts.Version

	if version == "" || version == "latest" {
		indexURL := fmt.Sprintf("%s/modules/%s/%s/index.json", base, author, command)
		var idx marketplaceIndex
		if err := fetchJSON(indexURL, &idx); err != nil {
			return "", "", "", fmt.Errorf("marketplace: resolve latest for %s: %w", opts.Module, err)
		}
		if idx.Latest == "" {
			return "", "", "", fmt.Errorf("marketplace: no published version found for %s", opts.Module)
		}
		version = idx.Latest
	}

	versionURL := fmt.Sprintf("%s/modules/%s/%s/%s.json", base, author, command, version)
	var vf marketplaceVersionFile
	if err := fetchJSON(versionURL, &vf); err != nil {
		return "", "", "", fmt.Errorf("marketplace: fetch version file for %s@%s: %w", opts.Module, version, err)
	}

	binURL, ok := vf.BinaryURLs[platform]
	if !ok {
		return "", "", "", fmt.Errorf("marketplace: no binary for platform %s in %s@%s", platform, opts.Module, version)
	}

	tmp := filepath.Join(os.TempDir(), fmt.Sprintf("prunesh-marketplace-%d", time.Now().UnixNano()))
	if err := downloadFile(binURL, tmp); err != nil {
		return "", "", "", fmt.Errorf("marketplace: download binary for %s@%s: %w", opts.Module, version, err)
	}
	if err := os.Chmod(tmp, 0o755); err != nil {
		os.Remove(tmp)
		return "", "", "", err
	}

	srcDir, err = writeMarketplaceSrcDir(vf)
	if err != nil {
		os.Remove(tmp)
		return "", "", "", err
	}

	return srcDir, tmp, version, nil
}

// writeMarketplaceSrcDir creates a temp directory containing a prunesh.json derived
// from the marketplace version file. This lets the existing manifest validation path
// in Install work unchanged.
func writeMarketplaceSrcDir(vf marketplaceVersionFile) (string, error) {
	dir, err := os.MkdirTemp("", "prunesh-marketplace-src-*")
	if err != nil {
		return "", err
	}

	manifest := pluginmanifest.Manifest{
		ID:        vf.ID,
		Command:   vf.Command,
		Platforms: vf.Platforms,
		Contract:  vf.Contract,
		PruneshCoreVersion: pluginmanifest.PruneshCoreVersion{
			Version:    vf.PruneshCoreVersion.Version,
			Constraint: vf.PruneshCoreVersion.Constraint,
		},
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		os.RemoveAll(dir)
		return "", err
	}

	manifestPath := filepath.Join(dir, pluginmanifest.ManifestFileName)
	if err := os.WriteFile(manifestPath, data, 0o644); err != nil {
		os.RemoveAll(dir)
		return "", err
	}

	return dir, nil
}

func fetchJSON(url string, v any) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d fetching %s", resp.StatusCode, url)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}
