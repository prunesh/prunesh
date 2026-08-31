// Package tree filters `tree` output for Claude Code.
package tree

import (
	"fmt"
	"strings"

	"github.com/prunesh/prunesh/internal/registry"
)

const (
	minCompact = 12
	maxLines   = 40
)

func init() {
	registry.Register(&Module{})
}

type Module struct{}

func (m *Module) Name() string { return "tree" }

func (m *Module) Rewrite(_ []string) ([]string, bool) {
	return nil, false
}

func (m *Module) FilterOutput(_ []string, output string, _ int) string {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	var entries []string
	for _, l := range lines {
		l = strings.TrimRight(l, "\r")
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		entries = append(entries, l)
	}
	if len(entries) < minCompact {
		return output
	}

	dirs, files := 0, 0
	for _, e := range entries {
		trimmed := strings.TrimSpace(e)
		if strings.HasSuffix(trimmed, "/") {
			dirs++
		} else {
			files++
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("tree: %d entries (%d dirs, %d files)\n\n", len(entries), dirs, files))
	limit := len(entries)
	if limit > maxLines {
		limit = maxLines
	}
	for _, e := range entries[:limit] {
		sb.WriteString(e)
		sb.WriteByte('\n')
	}
	if len(entries) > maxLines {
		sb.WriteString(fmt.Sprintf("... +%d more\n", len(entries)-maxLines))
	}

	result := sb.String()
	if len(result) >= len(output) {
		return output
	}
	return result
}

func (m *Module) TokensBefore(output string) int {
	return registry.EstimateTokens(output)
}

func (m *Module) TokensAfter(filtered string) int {
	return registry.EstimateTokens(filtered)
}
