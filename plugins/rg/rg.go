// Module rg: filters `rg` (ripgrep) output for Claude Code.
package rg

import (
	"github.com/prunesh/prunesh/internal/matchgroup"
	"github.com/prunesh/prunesh/internal/registry"
)

func init() {
	registry.Register(&Module{})
}

type Module struct{}

func (m *Module) Name() string { return "rg" }

func (m *Module) Rewrite(args []string) ([]string, bool) {
	return nil, false
}

func (m *Module) FilterOutput(_ []string, output string, _ int) string {
	return matchgroup.Format(output)
}

func (m *Module) TokensBefore(output string) int {
	return registry.EstimateTokens(output)
}

func (m *Module) TokensAfter(filtered string) int {
	return registry.EstimateTokens(filtered)
}
