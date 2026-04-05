// Module rg: filters `rg` (ripgrep) output for Claude Code.
// Handles both heading format (default) and flat format (--no-heading / file:line:content).
package rg

import (
	"fmt"
	"strings"

	"github.com/jmeiracorbal/gtk-ai/internal/registry"
)

const (
	maxTotalMatches   = 200
	maxMatchesPerFile = 10
	maxFiles          = 20
	minLines          = 5
)

func init() {
	registry.Register(&Module{})
}

type Module struct{}

func (m *Module) Name() string { return "rg" }

func (m *Module) Rewrite(args []string) ([]string, bool) {
	return nil, false
}

func (m *Module) FilterOutput(output string) string {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")

	var nonempty []string
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			nonempty = append(nonempty, l)
		}
	}

	if len(nonempty) < minLines {
		return output
	}

	byFile := parseRgOutput(nonempty)

	totalMatches := 0
	for _, matches := range byFile.matches {
		totalMatches += len(matches)
	}

	if totalMatches == 0 {
		return output
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🔍 %d matches in %d files\n\n", totalMatches, len(byFile.order)))

	filesShown := 0
	for _, file := range byFile.order {
		if filesShown >= maxFiles {
			remaining := len(byFile.order) - filesShown
			sb.WriteString(fmt.Sprintf("... +%d more files\n", remaining))
			break
		}
		matches := byFile.matches[file]
		sb.WriteString(file)
		sb.WriteString("\n")
		limit := len(matches)
		if limit > maxMatchesPerFile {
			limit = maxMatchesPerFile
		}
		for _, l := range matches[:limit] {
			sb.WriteString("  ")
			sb.WriteString(l)
			sb.WriteString("\n")
		}
		if len(matches) > maxMatchesPerFile {
			sb.WriteString(fmt.Sprintf("  ... +%d more matches\n", len(matches)-maxMatchesPerFile))
		}
		sb.WriteString("\n")
		filesShown++
	}

	if totalMatches > maxTotalMatches {
		sb.WriteString(fmt.Sprintf("... [gtkai: %d total matches, output truncated]\n", totalMatches))
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

// fileMatches preserves insertion order.
type fileMatches struct {
	order   []string
	matches map[string][]string
}

// isMatchLine reports whether a line is a rg match line in heading format.
// Match lines start with one or more digits followed immediately by ':' or '-'
// (e.g. "15:content" or "15-context"). File names that start with digits
// (e.g. "123.go") do not match this pattern.
func isMatchLine(l string) bool {
	i := 0
	for i < len(l) && l[i] >= '0' && l[i] <= '9' {
		i++
	}
	if i == 0 || i >= len(l) {
		return false
	}
	return l[i] == ':' || l[i] == '-'
}

// parseRgOutput handles both rg output formats:
//
// Heading format (rg default, with -n):
//
//	src/file.go
//	15:    return fmt.Errorf("error")
//
// Flat format (--no-heading):
//
//	src/file.go:15:    return fmt.Errorf("error")
//
// Detection: if any non-empty line matches the match-line pattern (digits followed
// by ':' or '-'), the output is in heading format. Otherwise flat.
func parseRgOutput(lines []string) fileMatches {
	result := fileMatches{matches: map[string][]string{}}

	headingFormat := false
	for _, l := range lines {
		if isMatchLine(l) {
			headingFormat = true
			break
		}
	}

	if headingFormat {
		currentFile := ""
		for _, l := range lines {
			if isMatchLine(l) {
				if currentFile != "" {
					result.matches[currentFile] = append(result.matches[currentFile], l)
				}
			} else {
				currentFile = l
				if _, seen := result.matches[currentFile]; !seen {
					result.order = append(result.order, currentFile)
					result.matches[currentFile] = nil
				}
			}
		}
	} else {
		// Flat format: file:line:content or file:content
		for _, l := range lines {
			file := flatFile(l)
			if _, seen := result.matches[file]; !seen {
				result.order = append(result.order, file)
			}
			result.matches[file] = append(result.matches[file], l)
		}
	}

	return result
}

// flatFile extracts the filename from a flat rg line (file:line:content or file:content).
func flatFile(line string) string {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}
