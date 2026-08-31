package pytest

import (
	"strings"

	"github.com/prunesh/prunesh/internal/registry"
)

const maxFailureDetail = 50

// Filter compacts pytest stdout: summary on success; failures + short traceback on error.
func Filter(output string, exitCode int) string {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	summary := extractSummary(lines)

	if exitCode == 0 {
		if summary != "" {
			return registry.NeverWorse(output, summary+"\n")
		}
		return output
	}

	filtered := filterFailure(lines, summary)
	if filtered == "" {
		return output
	}
	return registry.NeverWorse(output, filtered)
}

func filterFailure(lines []string, summary string) string {
	var sb strings.Builder
	inFailures := false
	inShort := false
	detailLines := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if isFailuresHeader(trimmed) {
			inFailures = true
			inShort = false
			continue
		}
		if isShortSummaryHeader(trimmed) {
			inFailures = false
			inShort = true
			continue
		}
		if inShort {
			if strings.HasPrefix(trimmed, "FAILED") || strings.HasPrefix(trimmed, "ERROR") {
				sb.WriteString(line)
				sb.WriteByte('\n')
			}
			if isBannerLine(trimmed) && !isShortSummaryHeader(trimmed) {
				inShort = false
			}
			continue
		}
		if inFailures {
			if isBannerLine(trimmed) && !isFailuresHeader(trimmed) {
				inFailures = false
				continue
			}
			if detailLines >= maxFailureDetail {
				if trimmed == "" {
					sb.WriteString("... (traceback truncated)\n")
					inFailures = false
				}
				continue
			}
			sb.WriteString(line)
			sb.WriteByte('\n')
			detailLines++
			continue
		}
		if strings.HasPrefix(trimmed, "FAILED") || strings.HasPrefix(trimmed, "ERROR") {
			sb.WriteString(line)
			sb.WriteByte('\n')
		}
	}

	if summary != "" {
		if sb.Len() > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(summary)
		sb.WriteByte('\n')
	}
	return sb.String()
}

func extractSummary(lines []string) string {
	for i := len(lines) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			continue
		}
		if isResultSummary(trimmed) {
			return trimmed
		}
	}
	return ""
}

func isResultSummary(line string) bool {
	if strings.Contains(line, " passed") || strings.Contains(line, " failed") ||
		strings.Contains(line, " error") {
		if isBannerLine(line) {
			return true
		}
		if strings.Contains(line, " in ") && (strings.Contains(line, "s") || strings.Contains(line, "ms")) {
			return true
		}
	}
	return false
}

func isBannerLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "=") && strings.HasSuffix(trimmed, "=")
}

func isFailuresHeader(line string) bool {
	return isBannerLine(line) && strings.Contains(line, "FAILURES")
}

func isShortSummaryHeader(line string) bool {
	return strings.Contains(strings.ToLower(line), "short test summary info")
}
