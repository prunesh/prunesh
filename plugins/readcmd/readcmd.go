// Package readcmd proxies cat/head/tail and filters with read.FilterContent.
package readcmd

import (
	"strings"

	"github.com/prunesh/prunesh/internal/registry"
	readmod "github.com/prunesh/prunesh/plugins/read"
)

func init() {
	registry.Register(&Module{name: "cat"})
	registry.Register(&Module{name: "head"})
	registry.Register(&Module{name: "tail"})
}

type Module struct {
	name string
}

func (m *Module) Name() string { return m.name }

func (m *Module) Rewrite(_ []string) ([]string, bool) {
	return nil, false
}

func (m *Module) FilterOutput(args []string, output string, _ int) string {
	if strings.TrimSpace(output) == "" {
		return output
	}
	path := filePath(args)
	if path == "" {
		return output
	}
	filtered, changed := readmod.FilterContent(path, output)
	if !changed {
		return output
	}
	return registry.NeverWorse(output, filtered)
}

func (m *Module) TokensBefore(output string) int {
	return registry.EstimateTokens(output)
}

func (m *Module) TokensAfter(filtered string) int {
	return registry.EstimateTokens(filtered)
}

func filePath(args []string) string {
	var paths []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			paths = append(paths, args[i+1:]...)
			break
		}
		if strings.HasPrefix(a, "-") {
			if a == "-n" || a == "-c" {
				i++
			}
			continue
		}
		paths = append(paths, a)
	}
	if len(paths) != 1 {
		return ""
	}
	return paths[0]
}
