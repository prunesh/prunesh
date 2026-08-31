// Package matchgroup groups grep/rg matches by file with shared caps.
package matchgroup

import (
	"fmt"
	"strings"
)

const (
	maxTotalMatches   = 200
	maxMatchesPerFile = 10
	maxFiles          = 20
	minLines          = 5
)

// Format groups match lines by file. Returns output unchanged when grouping
// would not shrink the text or the set is too small to group.
func Format(output string) string {
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

	byFile := parse(nonempty)

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
		sb.WriteString(fmt.Sprintf("... [prunesh: %d total matches, output truncated]\n", totalMatches))
	}

	result := sb.String()
	if len(result) >= len(output) {
		return output
	}
	return result
}

type fileMatches struct {
	order   []string
	matches map[string][]string
}

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

func parse(lines []string) fileMatches {
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
		return result
	}

	for _, l := range lines {
		file := flatFile(l)
		if _, seen := result.matches[file]; !seen {
			result.order = append(result.order, file)
		}
		result.matches[file] = append(result.matches[file], l)
	}
	return result
}

func flatFile(line string) string {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}
