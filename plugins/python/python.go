// Package python filters python -m pytest invocations for Claude Code.
package python

import (
	"github.com/prunesh/prunesh/internal/registry"
	"github.com/prunesh/prunesh/plugins/pytest"
)

func init() {
	registry.Register(&Module{name: "python"})
	registry.Register(&Module{name: "python3"})
}

type Module struct {
	name string
}

func (m *Module) Name() string { return m.name }

func (m *Module) Rewrite(args []string) ([]string, bool) {
	return nil, false
}

func (m *Module) FilterOutput(args []string, output string, exitCode int) string {
	if !IsPytestInvocation(args) {
		return output
	}
	return pytest.Filter(output, exitCode)
}

func (m *Module) TokensBefore(output string) int {
	return registry.EstimateTokens(output)
}

func (m *Module) TokensAfter(filtered string) int {
	return registry.EstimateTokens(filtered)
}

// IsPytestInvocation reports whether args invoke pytest via python -m.
func IsPytestInvocation(args []string) bool {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "-m" && args[i+1] == "pytest" {
			return true
		}
	}
	return false
}
