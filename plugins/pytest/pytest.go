// Package pytest filters pytest command output for Claude Code.
package pytest

import (
	"github.com/prunesh/prunesh/internal/registry"
)

func init() {
	registry.Register(&Module{})
}

type Module struct{}

func (m *Module) Name() string { return "pytest" }

func (m *Module) Rewrite(args []string) ([]string, bool) {
	return nil, false
}

func (m *Module) FilterOutput(args []string, output string, exitCode int) string {
	return Filter(output, exitCode)
}

func (m *Module) TokensBefore(output string) int {
	return registry.EstimateTokens(output)
}

func (m *Module) TokensAfter(filtered string) int {
	return registry.EstimateTokens(filtered)
}
