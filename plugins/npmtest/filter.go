package npmtest

import (
	"strings"

	"github.com/prunesh/prunesh/internal/registry"
)

const (
	maxFailureDetail = 60
	maxSummaryScan   = 30
)

// Filter compacts vitest/jest/npm test output: summary on success, failures on error.
func Filter(output string, exitCode int) string {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	summary := extractSummary(lines)

	if exitCode == 0 {
		filtered := filterSuccess(lines, summary)
		if filtered == "" {
			return output
		}
		return registry.NeverWorse(output, filtered)
	}

	filtered := filterFailure(lines, summary)
	if filtered == "" {
		return output
	}
	return registry.NeverWorse(output, filtered)
}

func filterSuccess(lines []string, summary []string) string {
	if len(summary) > 0 {
		return strings.Join(summary, "\n") + "\n"
	}
	return "ok\n"
}

func filterFailure(lines []string, summary []string) string {
	var sb strings.Builder
	detailLines := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if isPassLine(trimmed) {
			continue
		}
		if isNoiseLine(trimmed) {
			continue
		}
		if isFailureLine(trimmed) || isFailureContext(trimmed) {
			if detailLines >= maxFailureDetail {
				continue
			}
			sb.WriteString(line)
			sb.WriteByte('\n')
			detailLines++
		}
	}

	for _, s := range summary {
		if sb.Len() > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(s)
		sb.WriteByte('\n')
	}
	return sb.String()
}

func extractSummary(lines []string) []string {
	var summary []string
	start := len(lines) - maxSummaryScan
	if start < 0 {
		start = 0
	}
	for i := start; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			continue
		}
		if isSummaryLine(trimmed) {
			summary = append(summary, trimmed)
		}
	}
	return summary
}

func isSummaryLine(line string) bool {
	lower := strings.ToLower(line)
	switch {
	case strings.HasPrefix(lower, "test suites:"):
		return true
	case strings.HasPrefix(lower, "tests:"):
		return true
	case strings.HasPrefix(lower, "test files"):
		return true
	case strings.HasPrefix(lower, "tests "):
		return true
	case strings.Contains(lower, " passed") && strings.Contains(lower, " total"):
		return true
	case strings.HasPrefix(lower, "time:"):
		return true
	case strings.HasPrefix(lower, "duration"):
		return true
	case strings.HasPrefix(lower, "ran "):
		return true
	default:
		return false
	}
}

func isPassLine(line string) bool {
	if strings.HasPrefix(line, "PASS ") || strings.HasPrefix(line, "✓") || strings.HasPrefix(line, "√") {
		return true
	}
	if strings.Contains(line, " passed (") && strings.Contains(line, "Tests") {
		return true
	}
	return false
}

func isFailureLine(line string) bool {
	lower := strings.ToLower(line)
	if strings.HasPrefix(line, "FAIL ") || strings.HasPrefix(line, "●") || strings.HasPrefix(line, "✕") {
		return true
	}
	if strings.Contains(lower, "assertionerror") || strings.Contains(lower, "expect(") {
		return true
	}
	if strings.HasPrefix(lower, "error:") || strings.HasPrefix(lower, "failed") {
		return true
	}
	return false
}

func isFailureContext(line string) bool {
	if strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t") {
		return true
	}
	if strings.HasPrefix(line, "  at ") || strings.HasPrefix(line, ">") {
		return true
	}
	if strings.Contains(line, ".test.") || strings.Contains(line, ".spec.") {
		return true
	}
	return false
}

func isNoiseLine(line string) bool {
	lower := strings.ToLower(line)
	if strings.HasPrefix(lower, "> ") || strings.HasPrefix(lower, "npm warn") {
		return true
	}
	if strings.HasPrefix(lower, "watching") || strings.HasPrefix(lower, "collecting") {
		return true
	}
	return false
}

// IsPackageTest reports whether npm/pnpm args invoke a test script.
func IsPackageTest(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "test":
		return true
	case "run":
		if len(args) < 2 {
			return false
		}
		switch args[1] {
		case "test", "vitest", "jest", "unit", "e2e":
			return true
		}
	}
	return false
}

// IsNpxTest reports whether npx args invoke vitest or jest.
func IsNpxTest(args []string) bool {
	if len(args) == 0 {
		return false
	}
	for _, a := range args {
		base := strings.ToLower(a)
		if base == "vitest" || base == "jest" {
			return true
		}
		if strings.Contains(base, "vitest") || strings.Contains(base, "jest") {
			return true
		}
	}
	return false
}
