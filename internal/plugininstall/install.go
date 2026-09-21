// Package plugininstall downloads, validates, and installs external plugin modules.
package plugininstall

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/prunesh/prunesh/internal/pluginmanifest"
	"github.com/prunesh/prunesh/internal/pluginregistry"
	"github.com/prunesh/prunesh/internal/pluginsubprocess"
	"github.com/prunesh/prunesh/internal/storage"
)

// Options configures filter installation.
type Options struct {
	Module      string
	Version     string
	CoreVersion string
	ReleaseRepo string
	Replace     bool   // allow replacing another active filter for the same argv0
	LocalDir    string // if set, use this local directory instead of downloading or building
}

// ParseRef splits a plugin ref into module and version.
// Marketplace refs (author/command or author/command@version) are accepted with
// an empty version meaning "latest". Go module refs require an explicit version.
func ParseRef(ref string) (module, version string, err error) {
	if ref == "" {
		return "", "", fmt.Errorf("ref is empty")
	}
	i := strings.LastIndex(ref, "@")
	if i < 0 {
		// No version — allowed for marketplace refs only.
		if !isMarketplaceRef(ref) {
			return "", "", fmt.Errorf("ref must be module@version")
		}
		return ref, "", nil
	}
	if i == 0 || i == len(ref)-1 {
		return "", "", fmt.Errorf("ref must be module@version")
	}
	return ref[:i], ref[i+1:], nil
}

// Install downloads or builds the filter, validates prunesh.json, and registers it.
func Install(opts Options) (*pluginregistry.Record, error) {
	if opts.Module == "" {
		return nil, fmt.Errorf("module is empty")
	}
	if opts.Version == "" && !isMarketplaceRef(opts.Module) {
		return nil, fmt.Errorf("version is empty")
	}
	if opts.CoreVersion == "" {
		return nil, fmt.Errorf("core version is empty")
	}

	platform := runtime.GOOS + "/" + runtime.GOARCH
	srcDir, binary, resolvedVersion, err := resolveSource(opts, platform)
	if err != nil {
		return nil, err
	}
	if resolvedVersion != "" {
		opts.Version = resolvedVersion
	}

	manifestPath := filepath.Join(srcDir, pluginmanifest.ManifestFileName)
	manifest, err := pluginmanifest.ParseFile(manifestPath)
	if err != nil {
		return nil, err
	}
	if err := manifest.Validate(opts.CoreVersion, platform); err != nil {
		return nil, err
	}
	if err := pluginsubprocess.ContractCheck(binary); err != nil {
		return nil, fmt.Errorf("liveness: %w", err)
	}

	db, err := pluginregistry.Open()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	if err := checkReplaceConflict(db, manifest.ID, manifest.Command, opts.Replace); err != nil {
		return nil, err
	}

	destDir, err := installDir(manifest.ID)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return nil, err
	}

	binName := pluginsubprocess.BinaryNameFromID(manifest.ID)
	destBin := filepath.Join(destDir, binName)
	if err := copyFile(binary, destBin, 0o755); err != nil {
		return nil, err
	}
	destManifest := filepath.Join(destDir, pluginmanifest.ManifestFileName)
	if err := copyFile(manifestPath, destManifest, 0o644); err != nil {
		return nil, err
	}

	rec := pluginregistry.Record{
		ID:           manifest.ID,
		Module:       opts.Module,
		Version:      opts.Version,
		Argv0:        manifest.Command,
		Contract:     manifest.Contract,
		BinaryPath:   destBin,
		ManifestPath: destManifest,
		InstalledAt:  time.Now(),
	}

	if err := db.Install(rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

func checkReplaceConflict(db *pluginregistry.DB, id, argv0 string, replace bool) error {
	prev, err := db.Active(argv0)
	if err != nil {
		return err
	}
	if prev == nil || prev.ID == id {
		return nil
	}
	if !replace {
		return fmt.Errorf("active filter %s already handles %q; use --replace to install %s (or filter uninstall %s)", prev.ID, argv0, id, prev.ID)
	}
	fmt.Fprintf(os.Stderr, "replacing active filter %s with %s for command %q\n", prev.ID, id, argv0)
	return nil
}

// resolveSource returns (srcDir, binaryPath, resolvedVersion, error).
// resolvedVersion is non-empty only for marketplace refs where the version was
// determined by fetching the index (i.e. opts.Version was empty or "latest").
func resolveSource(opts Options, platform string) (srcDir, binary, resolvedVersion string, err error) {
	if opts.LocalDir != "" {
		binName := filepath.Base(opts.Module)
		if binName == "" || binName == "." {
			return "", "", "", fmt.Errorf("cannot derive binary name from module %q", opts.Module)
		}
		bin := filepath.Join(opts.LocalDir, binName)
		return opts.LocalDir, bin, "", nil
	}
	if isMarketplaceRef(opts.Module) {
		srcDir, binary, resolvedVersion, err = resolveFromMarketplace(opts, platform)
		return
	}
	if prebuilt, ok := tryPrebuilt(opts.Module, opts.Version, platform); ok {
		srcDir, err = fetchGoModule(opts.Module, opts.Version)
		if err != nil {
			return "", "", "", err
		}
		return srcDir, prebuilt, "", nil
	}
	srcDir, err = fetchGoModule(opts.Module, opts.Version)
	if err != nil {
		return "", "", "", err
	}
	binary, err = buildModule(opts.Module, opts.Version)
	if err != nil {
		return "", "", "", err
	}
	return srcDir, binary, "", nil
}

func tryPrebuilt(module, version, platform string) (path string, ok bool) {
	if module == "" {
		return "", false
	}
	repo := strings.TrimPrefix(module, "github.com/")
	binName := filepath.Base(module)
	osName, arch := splitPlatform(platform)
	tag := version
	if !strings.HasPrefix(tag, "v") {
		tag = "v" + tag
	}
	url := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s-%s-%s", repo, tag, binName, osName, arch)
	tmp := filepath.Join(os.TempDir(), fmt.Sprintf("prunesh-filter-prebuilt-%d", time.Now().UnixNano()))
	if err := downloadFile(url, tmp); err != nil {
		return "", false
	}
	if err := os.Chmod(tmp, 0o755); err != nil {
		os.Remove(tmp)
		return "", false
	}
	return tmp, true
}

func splitPlatform(platform string) (osName, arch string) {
	parts := strings.Split(platform, "/")
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

func execGo(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	cmd.Env = os.Environ()
	return cmd.CombinedOutput()
}

func fetchGoModule(module, version string) (string, error) {
	modVersion := module + "@" + version
	out, err := execGo("", "mod", "download", "-json", modVersion)
	if err != nil {
		return "", fmt.Errorf("go mod download %s: %w", modVersion, err)
	}
	var info struct {
		Dir string `json:"Dir"`
	}
	if err := json.Unmarshal(out, &info); err != nil {
		return "", fmt.Errorf("parse go mod download json: %w", err)
	}
	if info.Dir == "" {
		return "", fmt.Errorf("go mod download returned empty dir for %s", modVersion)
	}
	return info.Dir, nil
}

func buildModule(module, version string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "prunesh-filter-build-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	init := exec.Command("go", "mod", "init", "prunesh-filter-build")
	init.Dir = tmpDir
	init.Env = os.Environ()
	if out, err := init.CombinedOutput(); err != nil {
		return "", fmt.Errorf("go mod init: %w\n%s", err, out)
	}
	modVersion := module + "@" + version
	get := exec.Command("go", "get", modVersion)
	get.Dir = tmpDir
	get.Env = os.Environ()
	if out, err := get.CombinedOutput(); err != nil {
		return "", fmt.Errorf("go get %s: %w\n%s", modVersion, err, out)
	}
	tmpBin := filepath.Join(os.TempDir(), fmt.Sprintf("prunesh-filter-bin-%d", time.Now().UnixNano()))
	pkg := module + "/cmd"
	if err := goBuildDir(tmpDir, pkg, tmpBin); err != nil {
		return "", err
	}
	return tmpBin, nil
}

func goBuildDir(dir, pkg, out string) error {
	cmd := exec.Command("go", "build", "-o", out, pkg)
	cmd.Dir = dir
	cmd.Env = os.Environ()
	outBytes, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go build %s: %w\n%s", pkg, err, outBytes)
	}
	return nil
}

func downloadFile(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

func installDir(id string) (string, error) {
	base, err := storage.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "plugins", id), nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

