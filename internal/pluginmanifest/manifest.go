// Package pluginmanifest parses and validates external plugin manifests (prunesh.json).
package pluginmanifest

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"golang.org/x/mod/semver"
)

var idRegex = regexp.MustCompile(`^[a-z0-9_-]+/[a-z0-9_-]+$`)

// availableContracts lists the plugin communication protocols this version of prunesh supports.
// Add new entries here when a new protocol (e.g. grpc/v1) is implemented.
var availableContracts = []string{"stdin/v1"}

func contractSupported(c string) bool {
	for _, v := range availableContracts {
		if v == c {
			return true
		}
	}
	return false
}

// Validate checks manifest fields, platform, and core version compatibility.
func (m *Manifest) Validate(runningPrunesh, platform string) error {
	if !idRegex.MatchString(m.ID) {
		return fmt.Errorf("id %q does not match naming rule", m.ID)
	}
	if strings.TrimSpace(m.Command) == "" {
		return fmt.Errorf("command must not be empty")
	}
	if !contractSupported(m.Contract) {
		return fmt.Errorf("unsupported contract %q (supported: %s)", m.Contract, strings.Join(availableContracts, ", "))
	}
	if !semver.IsValid(normalizeSemver(m.PruneshCoreVersion.Version)) {
		return fmt.Errorf("prunesh-core-version.version %q is not valid semver", m.PruneshCoreVersion.Version)
	}
	if m.PruneshCoreVersion.Constraint != "min" && m.PruneshCoreVersion.Constraint != "exact" {
		return fmt.Errorf("unknown constraint %q", m.PruneshCoreVersion.Constraint)
	}
	if !platformListed(m.Platforms, platform) {
		return fmt.Errorf("platform %q not in manifest platforms", platform)
	}
	return m.ValidatePruneshCoreVersion(runningPrunesh)
}

func platformListed(platforms []string, platform string) bool {
	for _, p := range platforms {
		if p == platform {
			return true
		}
	}
	return false
}

// ManifestFileName is the required manifest filename at the root of a filter repository.
const ManifestFileName = "prunesh.json"

// Manifest is the required prunesh.json schema for external plugins.
type Manifest struct {
	ID               string           `json:"id"`
	Command          string           `json:"command"`
	Platforms        []string         `json:"platforms"`
	Contract         string           `json:"contract"`
	PruneshCoreVersion PruneshCoreVersion `json:"prunesh-core-version"`
}

// PruneshCoreVersion declares which prunesh core versions may run this filter.
type PruneshCoreVersion struct {
	Version    string `json:"version"`
	Constraint string `json:"constraint"`
}

// ParseFile reads and unmarshals prunesh.json at path.
func ParseFile(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read prunesh.json: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse prunesh.json: %w", err)
	}
	return &m, nil
}

// ValidatePruneshCoreVersion checks that runningPrunesh satisfies the manifest constraint.
func (m *Manifest) ValidatePruneshCoreVersion(runningPrunesh string) error {
	switch m.PruneshCoreVersion.Constraint {
	case "min":
		if !satisfiesMin(runningPrunesh, m.PruneshCoreVersion.Version) {
			return fmt.Errorf("prunesh %s < required min %s", runningPrunesh, m.PruneshCoreVersion.Version)
		}
	case "exact":
		if runningPrunesh != m.PruneshCoreVersion.Version {
			return fmt.Errorf("prunesh %s != required exact %s", runningPrunesh, m.PruneshCoreVersion.Version)
		}
	default:
		return fmt.Errorf("unknown constraint %q", m.PruneshCoreVersion.Constraint)
	}
	return nil
}

func satisfiesMin(running, required string) bool {
	runningNorm := normalizeSemver(running)
	requiredNorm := normalizeSemver(required)
	if semver.Compare(runningNorm, requiredNorm) >= 0 {
		return true
	}
	// pre-release of the required base satisfies min (0.11.0-beta.2 >= min 0.11.0)
	if semver.Prerelease(runningNorm) != "" && semver.Prerelease(requiredNorm) == "" {
		if i := strings.Index(runningNorm, "-"); i > 0 {
			base := runningNorm[:i]
			if semver.Compare(base, requiredNorm) >= 0 {
				return true
			}
		}
	}
	return false
}

func normalizeSemver(v string) string {
	if strings.HasPrefix(v, "v") || strings.HasPrefix(v, "V") {
		return "v" + strings.TrimPrefix(strings.TrimPrefix(v, "V"), "v")
	}
	return "v" + v
}
