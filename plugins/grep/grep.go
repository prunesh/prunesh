// Module grep: filters `grep` output for Claude Code.
package grep

import (
	"strings"

	"github.com/prunesh/prunesh/internal/matchgroup"
	"github.com/prunesh/prunesh/internal/registry"
)

func init() {
	registry.Register(&Module{})
}

type Module struct{}

func (m *Module) Name() string { return "grep" }

func (m *Module) Rewrite(args []string) ([]string, bool) {
	if hasArg(args, "--help") {
		return nil, false
	}
	if hasShortOrLong(args, 'c', "--count") || hasShortOrLong(args, 'l', "--files-with-matches") ||
		hasArg(args, "-L") || hasArg(args, "--files-without-match") ||
		hasShortOrLong(args, 'o', "--only-matching") {
		return nil, false
	}

	var extra []string
	if !hasShortOrLong(args, 'n', "--line-number") {
		extra = append(extra, "-n")
	}
	if !hasShortOrLong(args, 'H', "--with-filename") {
		extra = append(extra, "-H")
	}
	if len(extra) == 0 {
		return nil, false
	}
	return append(extra, args...), true
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

func hasArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

func hasShortOrLong(args []string, short byte, long string) bool {
	for _, a := range args {
		if a == long || strings.HasPrefix(a, long+"=") {
			return true
		}
		if len(a) >= 2 && a[0] == '-' && a[1] != '-' {
			if strings.IndexByte(a, short) >= 0 {
				return true
			}
		}
	}
	return false
}
