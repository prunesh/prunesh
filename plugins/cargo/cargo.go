// Package cargo filters cargo test/build/clippy/check output for Claude Code.
package cargo

import (
	"fmt"
	"strings"

	"github.com/prunesh/prunesh/internal/registry"
)

const (
	maxFailureLines   = 300
	compileLinePrefix = "Compiling "
	checkLinePrefix   = "Checking "
)

var handledSubs = map[string]struct{}{
	"test": {}, "build": {}, "check": {}, "clippy": {},
}

func init() {
	registry.Register(&Module{})
}

type Module struct{}

func (m *Module) Name() string { return "cargo" }

func (m *Module) Rewrite(args []string) ([]string, bool) {
	globals, sub, rest, ok := splitCargoArgs(args)
	if !ok {
		return nil, false
	}
	if _, handled := handledSubs[sub]; !handled {
		return nil, false
	}
	if hasQuiet(globals, rest) || hasVerbose(globals, rest) {
		return nil, false
	}
	out := append(append([]string{}, globals...), sub, "-q")
	out = append(out, rest...)
	return out, true
}

func (m *Module) FilterOutput(args []string, output string, exitCode int) string {
	_, sub, _, ok := splitCargoArgs(args)
	if !ok {
		return output
	}
	if _, handled := handledSubs[sub]; !handled {
		return output
	}
	if exitCode != 0 {
		return filterFailure(output)
	}
	return filterSuccess(output)
}

func (m *Module) TokensBefore(output string) int {
	return registry.EstimateTokens(output)
}

func (m *Module) TokensAfter(filtered string) int {
	return registry.EstimateTokens(filtered)
}

func filterSuccess(output string) string {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	compileCount := 0
	var kept []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if isCompileLine(trimmed) {
			compileCount++
			continue
		}
		if trimmed == "" {
			continue
		}
		if isPassingTestLine(trimmed) {
			continue
		}
		if strings.HasPrefix(trimmed, "running ") && strings.Contains(trimmed, " test") {
			continue
		}
		if shouldKeepSuccessLine(trimmed) {
			kept = append(kept, line)
		}
	}

	var sb strings.Builder
	if compileCount > 0 {
		fmt.Fprintf(&sb, "Compiling %d crates\n", compileCount)
	}
	for _, l := range kept {
		sb.WriteString(l)
		sb.WriteByte('\n')
	}
	if sb.Len() == 0 {
		return "ok\n"
	}
	result := sb.String()
	if hasTestResult(result) {
		return result
	}
	if compileCount > 0 && len(kept) <= 2 {
		return result
	}
	if strings.TrimSpace(result) == "" {
		return "ok\n"
	}
	return result
}

func filterFailure(output string) string {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	compileCount := 0
	var kept []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if isCompileLine(trimmed) {
			compileCount++
			continue
		}
		kept = append(kept, line)
	}

	var sb strings.Builder
	if compileCount > 0 {
		fmt.Fprintf(&sb, "Compiling %d crates\n", compileCount)
	}
	for _, l := range kept {
		sb.WriteString(l)
		sb.WriteByte('\n')
	}
	result := sb.String()
	if len(kept) > maxFailureLines {
		return capLines(result, maxFailureLines)
	}
	return result
}

func isCompileLine(trimmed string) bool {
	return strings.HasPrefix(trimmed, compileLinePrefix) ||
		strings.HasPrefix(trimmed, checkLinePrefix)
}

func isPassingTestLine(trimmed string) bool {
	return strings.HasPrefix(trimmed, "test ") && strings.Contains(trimmed, "... ok")
}

func shouldKeepSuccessLine(trimmed string) bool {
	switch {
	case strings.HasPrefix(trimmed, "Finished"):
		return true
	case strings.HasPrefix(trimmed, "test result:"):
		return true
	case strings.HasPrefix(trimmed, "Doc-tests"):
		return true
	case strings.HasPrefix(trimmed, "Running "):
		return true
	default:
		return false
	}
}

func hasTestResult(out string) bool {
	return strings.Contains(out, "test result:")
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

func hasQuiet(globals, rest []string) bool {
	return hasFlag(globals, "-q", "--quiet") || hasFlag(rest, "-q", "--quiet")
}

func hasVerbose(globals, rest []string) bool {
	return hasFlag(globals, "-v", "-vv", "--verbose") ||
		hasFlag(rest, "-v", "-vv", "--verbose")
}

func hasFlag(args []string, flags ...string) bool {
	for _, a := range args {
		for _, f := range flags {
			if a == f {
				return true
			}
			if strings.HasPrefix(a, f+"=") {
				return true
			}
		}
	}
	return false
}

func splitCargoArgs(args []string) (globals []string, sub string, rest []string, ok bool) {
	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case a == "--manifest-path" || a == "--config" || a == "--color" ||
			a == "--target" || a == "--target-dir" || a == "--timings" ||
			a == "-p" || a == "--package" || a == "--exclude" ||
			a == "--features" || a == "-j" || a == "--jobs":
			if i+1 >= len(args) {
				return nil, "", nil, false
			}
			globals = append(globals, a, args[i+1])
			i += 2
		case strings.HasPrefix(a, "--manifest-path=") || strings.HasPrefix(a, "--config=") ||
			strings.HasPrefix(a, "--color=") || strings.HasPrefix(a, "--target=") ||
			strings.HasPrefix(a, "--target-dir=") || strings.HasPrefix(a, "--features=") ||
			strings.HasPrefix(a, "--exclude=") || strings.HasPrefix(a, "--package="):
			globals = append(globals, a)
			i++
		case a == "-q" || a == "--quiet" || a == "-v" || a == "-vv" || a == "--verbose" ||
			a == "--release" || a == "--frozen" || a == "--locked" || a == "--offline" ||
			a == "--workspace" || a == "--all-features" || a == "--no-default-features" ||
			a == "--keep-going" || a == "--all-targets" || a == "--lib" || a == "--bins" ||
			a == "--tests" || a == "--benches" || a == "--examples":
			globals = append(globals, a)
			i++
		default:
			if strings.HasPrefix(a, "-") {
				return nil, "", nil, false
			}
			return globals, a, args[i+1:], true
		}
	}
	return globals, "", nil, false
}
