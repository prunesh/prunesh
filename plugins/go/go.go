// Package gocmd filters go test/build/vet output for Claude Code.
package gocmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/prunesh/prunesh/internal/registry"
)

const (
	maxBuildVetLines = 200
)

func init() {
	registry.Register(&Module{})
}

type Module struct{}

func (m *Module) Name() string { return "go" }

func (m *Module) Rewrite(args []string) ([]string, bool) {
	globals, sub, rest, ok := splitGoArgs(args)
	if !ok || sub != "test" {
		return nil, false
	}
	if hasFlag(rest, "-json") || hasFlag(rest, "-bench") {
		return nil, false
	}
	out := append(append([]string{}, globals...), "test", "-json")
	out = append(out, rest...)
	return out, true
}

func (m *Module) FilterOutput(args []string, output string, exitCode int) string {
	_, sub, _, ok := splitGoArgs(args)
	if !ok {
		return output
	}
	switch sub {
	case "test":
		return filterTest(output)
	case "build", "vet":
		return filterBuildVet(output, exitCode)
	default:
		return output
	}
}

func (m *Module) TokensBefore(output string) int {
	return registry.EstimateTokens(output)
}

func (m *Module) TokensAfter(filtered string) int {
	return registry.EstimateTokens(filtered)
}

type testEvent struct {
	Action  string  `json:"Action"`
	Package string  `json:"Package"`
	Test    string  `json:"Test"`
	Output  string  `json:"Output"`
	Elapsed float64 `json:"Elapsed"`
}

type pkgState struct {
	status  string
	elapsed float64
	outputs []string
}

func filterTest(output string) string {
	if filtered, ok := filterTestJSON(output); ok {
		return filtered
	}
	return filterTestClassic(output)
}

func filterTestJSON(output string) (string, bool) {
	lines := strings.Split(output, "\n")
	pkgs := make(map[string]*pkgState)
	var pkgOrder []string

	getPkg := func(name string) *pkgState {
		if pkgs[name] == nil {
			pkgs[name] = &pkgState{}
			pkgOrder = append(pkgOrder, name)
		}
		return pkgs[name]
	}

	jsonLines := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var ev testEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue
		}
		jsonLines++
		if ev.Package == "" {
			continue
		}
		p := getPkg(ev.Package)
		switch ev.Action {
		case "output":
			if ev.Output != "" {
				p.outputs = append(p.outputs, ev.Output)
			}
		case "pass", "fail", "skip":
			if ev.Test == "" {
				p.status = ev.Action
				if ev.Elapsed > 0 {
					p.elapsed = ev.Elapsed
				}
			} else if ev.Action == "fail" {
				p.status = "fail"
			}
		}
	}
	if jsonLines == 0 {
		return "", false
	}

	var okCount int
	var totalElapsed float64
	var failSB strings.Builder

	for _, name := range pkgOrder {
		p := pkgs[name]
		switch p.status {
		case "pass", "skip":
			okCount++
			totalElapsed += p.elapsed
		case "fail":
			failSB.WriteString("FAIL\t")
			failSB.WriteString(name)
			if p.elapsed > 0 {
				fmt.Fprintf(&failSB, "\t%.3fs\n", p.elapsed)
			} else {
				failSB.WriteByte('\n')
			}
			for _, o := range p.outputs {
				failSB.WriteString(o)
			}
		default:
			if len(p.outputs) > 0 {
				p.status = "fail"
				failSB.WriteString("FAIL\t")
				failSB.WriteString(name)
				failSB.WriteByte('\n')
				for _, o := range p.outputs {
					failSB.WriteString(o)
				}
			}
		}
	}

	var sb strings.Builder
	if okCount > 0 {
		if totalElapsed > 0 {
			fmt.Fprintf(&sb, "%d packages ok (%.1fs total)\n", okCount, totalElapsed)
		} else {
			fmt.Fprintf(&sb, "%d packages ok\n", okCount)
		}
	}
	if failSB.Len() > 0 {
		if sb.Len() > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(failSB.String())
	}
	if sb.Len() == 0 {
		return output, true
	}
	if !strings.HasSuffix(sb.String(), "\n") {
		sb.WriteByte('\n')
	}
	return sb.String(), true
}

func filterTestClassic(output string) string {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	okCount := 0
	failIdx := -1
	for i, line := range lines {
		if strings.HasPrefix(line, "ok  \t") || strings.HasPrefix(line, "ok\t") {
			okCount++
			continue
		}
		if strings.HasPrefix(line, "FAIL\t") && failIdx < 0 {
			failIdx = i
		}
	}
	if failIdx < 0 {
		if okCount > 0 && okCount == countClassicResults(lines) {
			return fmt.Sprintf("%d packages ok\n", okCount)
		}
		return output
	}
	var sb strings.Builder
	if okCount > 0 {
		fmt.Fprintf(&sb, "%d packages ok\n\n", okCount)
	}
	sb.WriteString(strings.Join(lines[failIdx:], "\n"))
	sb.WriteByte('\n')
	return sb.String()
}

func countClassicResults(lines []string) int {
	n := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "ok  \t") || strings.HasPrefix(line, "ok\t") ||
			strings.HasPrefix(line, "FAIL\t") {
			n++
		}
	}
	return n
}

func filterBuildVet(output string, exitCode int) string {
	if exitCode != 0 {
		return capLines(output, maxBuildVetLines)
	}
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return "ok\n"
	}
	if len(strings.Split(trimmed, "\n")) <= 5 {
		return output
	}
	return "ok\n"
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

func hasFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
		if strings.HasPrefix(a, flag+"=") {
			return true
		}
	}
	return false
}

func splitGoArgs(args []string) (globals []string, sub string, rest []string, ok bool) {
	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case a == "-C" || a == "-mod" || a == "-modfile" || a == "-overlay" ||
			a == "-p" || a == "-tags" || a == "-toolexec" || a == "-gcflags" ||
			a == "-ldflags" || a == "-asmflags" || a == "-compiler" ||
			a == "-installsuffix" || a == "-pkgdir" || a == "-buildmode":
			if i+1 >= len(args) {
				return nil, "", nil, false
			}
			globals = append(globals, a, args[i+1])
			i += 2
		case strings.HasPrefix(a, "-mod=") || strings.HasPrefix(a, "-modfile=") ||
			strings.HasPrefix(a, "-overlay=") || strings.HasPrefix(a, "-tags="):
			globals = append(globals, a)
			i++
		case a == "-modcacherw" || a == "-work" || a == "-a" || a == "-n" || a == "-x" ||
			a == "-race" || a == "-msan" || a == "-asan":
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
