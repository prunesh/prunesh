// Package npmtest filters npm/pnpm/npx test runner output for Claude Code.
package npmtest

import (
	"github.com/prunesh/prunesh/internal/registry"
)

func init() {
	registry.Register(&npmModule{})
	registry.Register(&pnpmModule{})
	registry.Register(&npxModule{})
}

type npmModule struct{}

func (m *npmModule) Name() string { return "npm" }

func (m *npmModule) Rewrite(args []string) ([]string, bool) {
	return nil, false
}

func (m *npmModule) FilterOutput(args []string, output string, exitCode int) string {
	if !IsPackageTest(args) {
		return output
	}
	return Filter(output, exitCode)
}

func (m *npmModule) TokensBefore(output string) int { return registry.EstimateTokens(output) }
func (m *npmModule) TokensAfter(filtered string) int {
	return registry.EstimateTokens(filtered)
}

type pnpmModule struct{}

func (m *pnpmModule) Name() string { return "pnpm" }

func (m *pnpmModule) Rewrite(args []string) ([]string, bool) {
	return nil, false
}

func (m *pnpmModule) FilterOutput(args []string, output string, exitCode int) string {
	if !IsPackageTest(args) {
		return output
	}
	return Filter(output, exitCode)
}

func (m *pnpmModule) TokensBefore(output string) int { return registry.EstimateTokens(output) }
func (m *pnpmModule) TokensAfter(filtered string) int {
	return registry.EstimateTokens(filtered)
}

type npxModule struct{}

func (m *npxModule) Name() string { return "npx" }

func (m *npxModule) Rewrite(args []string) ([]string, bool) {
	return nil, false
}

func (m *npxModule) FilterOutput(args []string, output string, exitCode int) string {
	if !IsNpxTest(args) {
		return output
	}
	return Filter(output, exitCode)
}

func (m *npxModule) TokensBefore(output string) int { return registry.EstimateTokens(output) }
func (m *npxModule) TokensAfter(filtered string) int {
	return registry.EstimateTokens(filtered)
}
