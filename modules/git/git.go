// Module git: filters `git` subcommand output for Claude Code.
// Handles: diff, log, status, branch.
package git

import (
	"fmt"
	"strings"

	"github.com/jmeiracorbal/gtk-ai/internal/registry"
)

const (
	maxDiffLines   = 300
	maxLogEntries  = 50
	maxStatusLines = 100
)

func init() {
	registry.Register(&Module{})
}

type Module struct{}

func (m *Module) Name() string { return "git" }

func (m *Module) Rewrite(args []string) ([]string, bool) {
	return nil, false
}

func (m *Module) FilterOutput(output string) string {
	// We need the subcommand to know how to filter.
	// Without it, return as-is — subcommand context is passed via FilterOutputWithArgs.
	return output
}

// FilterOutputWithArgs filters git output based on the subcommand.
func FilterOutputWithArgs(subcommand, output string) string {
	switch subcommand {
	case "diff":
		return filterDiff(output)
	case "log":
		return filterLog(output)
	case "status":
		return filterStatus(output)
	case "branch":
		return filterBranch(output)
	default:
		return output
	}
}

func filterDiff(output string) string {
	lines := strings.Split(output, "\n")
	if len(lines) <= maxDiffLines {
		return output
	}
	var sb strings.Builder
	for _, l := range lines[:maxDiffLines] {
		sb.WriteString(l)
		sb.WriteString("\n")
	}
	sb.WriteString(fmt.Sprintf("... +%d lines truncated (use git diff <file> for specific files)\n", len(lines)-maxDiffLines))
	return sb.String()
}

func filterLog(output string) string {
	// Each log entry starts with "commit "
	entries := splitLogEntries(output)
	if len(entries) <= maxLogEntries {
		return output
	}
	var sb strings.Builder
	for _, e := range entries[:maxLogEntries] {
		sb.WriteString(e)
	}
	sb.WriteString(fmt.Sprintf("... +%d commits truncated\n", len(entries)-maxLogEntries))
	return sb.String()
}

func filterStatus(output string) string {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
		return "nothing to commit, working tree clean\n"
	}

	var modified, untracked, staged []string
	for _, l := range lines {
		if len(l) < 2 {
			continue
		}
		switch {
		case strings.HasPrefix(l, "A ") || strings.HasPrefix(l, "M "):
			staged = append(staged, strings.TrimSpace(l[2:]))
		case strings.HasPrefix(l, " M"):
			modified = append(modified, strings.TrimSpace(l[2:]))
		case strings.HasPrefix(l, "??"):
			untracked = append(untracked, strings.TrimSpace(l[3:]))
		}
	}

	var sb strings.Builder
	if len(staged) > 0 {
		sb.WriteString(fmt.Sprintf("Staged (%d): %s\n", len(staged), strings.Join(staged, ", ")))
	}
	if len(modified) > 0 {
		sb.WriteString(fmt.Sprintf("Modified (%d): %s\n", len(modified), strings.Join(modified, ", ")))
	}
	if len(untracked) > 0 {
		if len(untracked) > 10 {
			sb.WriteString(fmt.Sprintf("Untracked (%d): %s ... +%d more\n", len(untracked), strings.Join(untracked[:10], ", "), len(untracked)-10))
		} else {
			sb.WriteString(fmt.Sprintf("Untracked (%d): %s\n", len(untracked), strings.Join(untracked, ", ")))
		}
	}
	if sb.Len() == 0 {
		if len(lines) > maxStatusLines {
			return strings.Join(lines[:maxStatusLines], "\n") + fmt.Sprintf("\n... +%d lines\n", len(lines)-maxStatusLines)
		}
		return output
	}
	result := sb.String()
	if len(result) >= len(output) {
		return output
	}
	return result
}

func filterBranch(output string) string {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	var branches []string
	var current string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		if strings.HasPrefix(l, "* ") {
			current = strings.TrimPrefix(l, "* ")
		} else {
			// Skip remote-tracking branches to reduce noise
			if !strings.Contains(l, "->") {
				branches = append(branches, l)
			}
		}
	}
	var sb strings.Builder
	if current != "" {
		sb.WriteString(fmt.Sprintf("current: %s\n", current))
	}
	if len(branches) > 0 {
		sb.WriteString(fmt.Sprintf("local (%d): %s\n", len(branches), strings.Join(branches, ", ")))
	}
	return sb.String()
}

func splitLogEntries(output string) []string {
	var entries []string
	var current strings.Builder
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "commit ") && current.Len() > 0 {
			entries = append(entries, current.String())
			current.Reset()
		}
		current.WriteString(line)
		current.WriteString("\n")
	}
	if current.Len() > 0 {
		entries = append(entries, current.String())
	}
	return entries
}

func (m *Module) TokensBefore(output string) int {
	return registry.EstimateTokens(output)
}

func (m *Module) TokensAfter(filtered string) int {
	return registry.EstimateTokens(filtered)
}
