// Package docker filters read-only docker command output for Claude Code.
package docker

import (
	"fmt"
	"strings"

	"github.com/prunesh/prunesh/internal/registry"
)

const (
	maxPSRows       = 25
	maxImageRows    = 30
	maxLogLines     = 100
	maxFailureLines = 200
)

func init() {
	registry.Register(&Module{})
}

type Module struct{}

func (m *Module) Name() string { return "docker" }

func (m *Module) Rewrite(args []string) ([]string, bool) {
	return nil, false
}

func (m *Module) FilterOutput(args []string, output string, exitCode int) string {
	globals, sub, rest, ok := splitDockerArgs(args)
	if !ok {
		return output
	}
	_ = globals
	switch sub {
	case "ps":
		return filterPS(output)
	case "images":
		return filterImages(output)
	case "logs":
		return filterLogs(output)
	case "compose":
		if len(rest) == 0 {
			return output
		}
		switch rest[0] {
		case "ps":
			return filterPS(output)
		case "logs":
			return filterLogs(output)
		}
	}
	if exitCode != 0 && len(output) > 0 {
		return capLines(output, maxFailureLines)
	}
	return output
}

func (m *Module) TokensBefore(output string) int {
	return registry.EstimateTokens(output)
}

func (m *Module) TokensAfter(filtered string) int {
	return registry.EstimateTokens(filtered)
}

func filterPS(output string) string {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	if len(lines) <= 1 {
		return output
	}
	header := strings.ToUpper(lines[0])
	if !strings.Contains(header, "CONTAINER") && !strings.Contains(header, "NAME") {
		return capLines(output, maxPSRows+1)
	}
	var sb strings.Builder
	sb.WriteString("NAME\tSTATUS\tPORTS\n")
	dataRows := 0
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if dataRows >= maxPSRows {
			break
		}
		name, status, ports := parsePSLine(line)
		if name == "" {
			continue
		}
		fmt.Fprintf(&sb, "%s\t%s\t%s\n", name, status, ports)
		dataRows++
	}
	if dataRows < len(lines)-1 {
		fmt.Fprintf(&sb, "... +%d more\n", len(lines)-1-dataRows)
	}
	result := sb.String()
	if result == "NAME\tSTATUS\tPORTS\n" {
		return capLines(output, maxPSRows+1)
	}
	return result
}

func parsePSLine(line string) (name, status, ports string) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return "", "", ""
	}
	name = fields[len(fields)-1]
	for _, f := range fields {
		if strings.Contains(f, "->") || strings.Contains(f, "::") {
			ports = f
		}
	}
	statusStart := -1
	for i, f := range fields {
		switch f {
		case "Up", "Exited", "Created", "Restarting", "Paused", "Dead":
			statusStart = i
		}
	}
	if statusStart >= 0 {
		end := len(fields) - 1
		if ports != "" {
			for i := statusStart; i < len(fields); i++ {
				if fields[i] == ports {
					end = i
					break
				}
			}
		}
		if end > statusStart {
			status = strings.Join(fields[statusStart:end], " ")
		}
	}
	return name, status, ports
}

func filterImages(output string) string {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	if len(lines) <= 1 {
		return output
	}
	header := lines[0]
	if !strings.Contains(strings.ToUpper(header), "REPOSITORY") {
		return capLines(output, maxImageRows+1)
	}
	cols := findColumns(header, []string{"REPOSITORY", "TAG", "SIZE"})
	var sb strings.Builder
	sb.WriteString("REPOSITORY\tTAG\tSIZE\n")
	shown := 0
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if shown >= maxImageRows {
			break
		}
		fields := splitFields(line)
		repo := fieldAt(fields, cols, "REPOSITORY")
		tag := fieldAt(fields, cols, "TAG")
		size := fieldAt(fields, cols, "SIZE")
		fmt.Fprintf(&sb, "%s\t%s\t%s\n", repo, tag, size)
		shown++
	}
	if shown < len(lines)-1 {
		fmt.Fprintf(&sb, "... +%d more\n", len(lines)-1-shown)
	}
	return sb.String()
}

func filterLogs(output string) string {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	if len(lines) <= maxLogLines {
		return output
	}
	skipped := len(lines) - maxLogLines
	var sb strings.Builder
	fmt.Fprintf(&sb, "... (%d lines truncated)\n", skipped)
	for _, l := range lines[skipped:] {
		sb.WriteString(l)
		sb.WriteByte('\n')
	}
	return sb.String()
}

func capLines(output string, max int) string {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	if len(lines) <= max {
		return output
	}
	var sb strings.Builder
	for _, l := range lines[:max] {
		sb.WriteString(l)
		sb.WriteByte('\n')
	}
	sb.WriteString(fmt.Sprintf("... (%d lines truncated)\n", len(lines)-max))
	return sb.String()
}

func splitDockerArgs(args []string) (globals []string, sub string, rest []string, ok bool) {
	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case a == "--config", a == "--context", a == "--host", a == "--log-level":
			if i+1 >= len(args) {
				return nil, "", nil, false
			}
			globals = append(globals, a, args[i+1])
			i += 2
		case strings.HasPrefix(a, "--config=") || strings.HasPrefix(a, "--context="):
			globals = append(globals, a)
			i++
		case a == "-H", a == "--host":
			if i+1 >= len(args) {
				return nil, "", nil, false
			}
			globals = append(globals, a, args[i+1])
			i += 2
		default:
			if strings.HasPrefix(a, "-") {
				globals = append(globals, a)
				i++
				continue
			}
			return globals, a, args[i+1:], true
		}
	}
	return globals, "", nil, false
}

func findColumns(header string, names []string) map[string]int {
	fields := splitFields(header)
	col := make(map[string]int)
	for i, f := range fields {
		upper := strings.ToUpper(f)
		for _, name := range names {
			if upper == name {
				col[name] = i
			}
		}
	}
	return col
}

func fieldAt(fields []string, cols map[string]int, names ...string) string {
	for _, name := range names {
		if idx, ok := cols[name]; ok && idx < len(fields) {
			return fields[idx]
		}
	}
	return ""
}

func splitFields(line string) []string {
	return strings.Fields(line)
}
