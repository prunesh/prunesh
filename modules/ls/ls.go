// Module ls: filters `ls` output for Claude Code.
// Removes noise, groups by type, adds size suffixes.
package ls

import (
	"fmt"
	"strings"

	"github.com/jmeiracorbal/gtk-ai/internal/registry"
)

func init() {
	registry.Register(&Module{})
}

type Module struct{}

func (m *Module) Name() string { return "ls" }

func (m *Module) Rewrite(args []string) ([]string, bool) {
	// No rewrite needed — we filter output instead
	return nil, false
}

func (m *Module) FilterOutput(output string) string {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	if len(lines) == 0 {
		return output
	}

	var dirs, files []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasSuffix(line, "/") {
			dirs = append(dirs, line)
		} else {
			files = append(files, line)
		}
	}

	var sb strings.Builder

	if len(dirs) > 0 {
		sb.WriteString(fmt.Sprintf("📁 %d dirs: %s\n", len(dirs), strings.Join(dirs, " ")))
	}
	if len(files) > 0 {
		// Group files by extension
		byExt := map[string][]string{}
		for _, f := range files {
			ext := extOf(f)
			byExt[ext] = append(byExt[ext], f)
		}
		sb.WriteString(fmt.Sprintf("📄 %d files\n", len(files)))
		for ext, group := range byExt {
			if ext == "" {
				ext = "no-ext"
			}
			sb.WriteString(fmt.Sprintf("  .%s(%d): %s\n", ext, len(group), strings.Join(group, " ")))
		}
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

func extOf(name string) string {
	dot := strings.LastIndex(name, ".")
	if dot < 0 || dot == len(name)-1 {
		return ""
	}
	return name[dot+1:]
}
