// Module git: filters `git` subcommand output for Claude Code.
// Handles: diff, log, status, branch.
package git

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/prunesh/prunesh/internal/registry"
)

const (
	maxDiffLines    = 300
	maxHunkLines    = 100
	maxLogDefault   = 10
	maxLogUserFmt   = 50
	maxLogLineWidth = 80
	maxStatusLines  = 100
	maxBranchList   = 15
	maxStashEntries = 20
	prettyLogFmt    = "%h %s (%ar) <%an>"
)

func init() {
	registry.Register(&Module{})
}

type Module struct{}

func (m *Module) Name() string { return "git" }

func (m *Module) Rewrite(args []string) ([]string, bool) {
	globals, sub, rest, ok := splitGitArgs(args)
	if !ok {
		return nil, false
	}
	switch sub {
	case "status":
		if !usesCompactStatusPath(rest) {
			return nil, false
		}
		out := append(append([]string{}, globals...), "status", "--porcelain", "-b")
		return out, true
	case "log":
		return rewriteLog(globals, rest)
	case "push", "pull", "fetch":
		return rewriteQuiet(globals, sub, rest)
	default:
		return nil, false
	}
}

func rewriteQuiet(globals []string, sub string, rest []string) ([]string, bool) {
	if hasVerboseFlag(rest) {
		return nil, false
	}
	out := append(append([]string{}, globals...), sub, "-q")
	out = append(out, rest...)
	return out, true
}

func hasVerboseFlag(args []string) bool {
	for _, a := range args {
		switch a {
		case "-v", "-vv", "--verbose", "--progress":
			return true
		}
		if strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") {
			for _, c := range a[1:] {
				if c == 'v' {
					return true
				}
			}
		}
	}
	return false
}

func rewriteLog(globals, rest []string) ([]string, bool) {
	hasFmt := hasLogFormat(rest)
	_, hasLimit := parseLogLimit(rest)
	if hasFmt && hasLimit {
		return nil, false
	}
	out := append(append([]string{}, globals...), "log")
	if !hasFmt {
		out = append(out, "--pretty=format:"+prettyLogFmt)
	}
	if !hasLimit {
		if hasFmt {
			out = append(out, "-50")
		} else {
			out = append(out, "-10")
		}
	}
	out = append(out, rest...)
	return out, true
}

func (m *Module) FilterOutput(args []string, output string, exitCode int) string {
	_, sub, rest, ok := splitGitArgs(args)
	if !ok {
		return output
	}
	switch sub {
	case "diff":
		return filterDiff(output)
	case "log":
		return filterLog(rest, output)
	case "status":
		return filterStatus(output)
	case "branch":
		return filterBranch(output)
	case "show":
		return filterShow(output)
	case "add", "commit", "push", "pull", "fetch":
		return filterWrite(sub, output, exitCode)
	case "stash":
		return filterStash(rest, output, exitCode)
	default:
		return output
	}
}

func filterDiff(output string) string {
	if !strings.Contains(output, "diff --git") && !strings.Contains(output, "@@") {
		return capLines(output, maxDiffLines)
	}

	var sb strings.Builder
	hunkShown := 0
	hunkSkipped := 0
	inHunk := false
	totalLines := 0
	truncated := false

	flushSkip := func() {
		if hunkSkipped > 0 {
			fmt.Fprintf(&sb, " ... (%d lines truncated)\n", hunkSkipped)
			truncated = true
			hunkSkipped = 0
		}
	}

	for _, line := range strings.Split(output, "\n") {
		if totalLines >= maxDiffLines {
			truncated = true
			break
		}
		switch {
		case strings.HasPrefix(line, "diff --git"):
			flushSkip()
			file := diffFile(line)
			sb.WriteByte('\n')
			sb.WriteString(file)
			sb.WriteByte('\n')
			inHunk = true
			hunkShown = 0
			totalLines++
		case strings.HasPrefix(line, "index ") || strings.HasPrefix(line, "--- ") ||
			strings.HasPrefix(line, "+++ ") || strings.HasPrefix(line, "new file") ||
			strings.HasPrefix(line, "deleted file") || strings.HasPrefix(line, "old mode") ||
			strings.HasPrefix(line, "new mode"):
			continue
		case strings.HasPrefix(line, "@@"):
			flushSkip()
			sb.WriteByte(' ')
			sb.WriteString(line)
			sb.WriteByte('\n')
			inHunk = true
			hunkShown = 0
			totalLines++
		case inHunk && (strings.HasPrefix(line, "+") || strings.HasPrefix(line, "-") || strings.HasPrefix(line, " ")):
			if strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") {
				continue
			}
			if hunkShown < maxHunkLines {
				sb.WriteByte(' ')
				sb.WriteString(line)
				sb.WriteByte('\n')
				hunkShown++
				totalLines++
			} else {
				hunkSkipped++
			}
		}
	}
	flushSkip()
	if truncated {
		sb.WriteString("... (more changes truncated)\n")
	}
	result := sb.String()
	if result == "" || len(result) >= len(output) {
		return capLines(output, maxDiffLines)
	}
	return result
}

func diffFile(line string) string {
	if i := strings.Index(line, " b/"); i >= 0 {
		return line[i+3:]
	}
	fields := strings.Fields(line)
	if len(fields) > 0 {
		return fields[len(fields)-1]
	}
	return line
}

func capLines(output string, max int) string {
	lines := strings.Split(output, "\n")
	if len(lines) <= max {
		return output
	}
	return strings.Join(lines[:max], "\n") + fmt.Sprintf("\n... +%d lines truncated\n", len(lines)-max)
}

func filterLog(args []string, output string) string {
	userFmt := hasLogFormat(args)
	userN, hasLimit := parseLogLimit(args)
	limit := maxLogDefault
	if hasLimit {
		limit = userN
	} else if userFmt {
		limit = maxLogUserFmt
	}

	if looksLikeVerboseLog(output) {
		entries := splitLogEntries(output)
		max := limit
		if hasLimit && userN >= len(entries) {
			max = len(entries)
		}
		if len(entries) <= max {
			return output
		}
		var sb strings.Builder
		for _, e := range entries[:max] {
			sb.WriteString(e)
		}
		sb.WriteString(fmt.Sprintf("... +%d commits truncated\n", len(entries)-max))
		return sb.String()
	}

	var lines []string
	for _, l := range strings.Split(strings.TrimRight(output, "\n"), "\n") {
		if strings.TrimSpace(l) == "" {
			continue
		}
		lines = append(lines, truncateLine(l, maxLogLineWidth))
	}
	max := limit
	if hasLimit {
		max = len(lines)
	}
	if len(lines) <= max {
		return strings.Join(lines, "\n") + "\n"
	}
	return strings.Join(lines[:max], "\n") + fmt.Sprintf("\n... +%d commits truncated\n", len(lines)-max)
}

func looksLikeVerboseLog(output string) bool {
	for _, l := range strings.Split(output, "\n") {
		if l == "" {
			continue
		}
		return strings.HasPrefix(l, "commit ")
	}
	return false
}

func truncateLine(s string, width int) string {
	if width <= 0 || len(s) <= width {
		return s
	}
	return s[:width]
}

func hasLogFormat(args []string) bool {
	for _, a := range args {
		if a == "--oneline" || strings.HasPrefix(a, "--pretty") || strings.HasPrefix(a, "--format") {
			return true
		}
	}
	return false
}

func parseLogLimit(args []string) (int, bool) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "-n" || a == "--max-count" {
			if i+1 >= len(args) {
				return 0, true
			}
			n, err := strconv.Atoi(args[i+1])
			if err != nil {
				return 0, true
			}
			return n, true
		}
		if rest, ok := strings.CutPrefix(a, "--max-count="); ok {
			n, err := strconv.Atoi(rest)
			if err != nil {
				return 0, true
			}
			return n, true
		}
		if strings.HasPrefix(a, "-n") && len(a) > 2 && isDigits(a[2:]) {
			n, _ := strconv.Atoi(a[2:])
			return n, true
		}
		if len(a) > 1 && a[0] == '-' && a[1] != '-' && isDigits(a[1:]) {
			n, _ := strconv.Atoi(a[1:])
			return n, true
		}
	}
	return 0, false
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func filterStatus(output string) string {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
		return "nothing to commit, working tree clean\n"
	}

	if !isPorcelainStatus(lines) {
		if len(lines) > maxStatusLines {
			return strings.Join(lines[:maxStatusLines], "\n") + fmt.Sprintf("\n... +%d lines\n", len(lines)-maxStatusLines)
		}
		return output
	}

	var branch string
	var modified, untracked, staged []string
	for _, l := range lines {
		if strings.HasPrefix(l, "##") {
			branch = strings.TrimSpace(strings.TrimPrefix(l, "##"))
			continue
		}
		if len(l) < 2 {
			continue
		}
		xy := l[:2]
		path := ""
		if len(l) > 2 {
			path = strings.TrimSpace(l[2:])
		}
		switch {
		case xy == "??":
			untracked = append(untracked, path)
		case xy[0] != ' ' && xy[0] != '?':
			staged = append(staged, path)
			if xy[1] != ' ' && xy[1] != '?' {
				modified = append(modified, path)
			}
		case xy[1] != ' ' && xy[1] != '?':
			modified = append(modified, path)
		}
	}

	var sb strings.Builder
	if branch != "" {
		sb.WriteString("* ")
		sb.WriteString(branch)
		sb.WriteString("\n")
	}
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
	if len(staged)+len(modified)+len(untracked) == 0 {
		if branch != "" {
			sb.WriteString("clean — nothing to commit\n")
			return sb.String()
		}
		return output
	}
	return sb.String()
}

func isPorcelainStatus(lines []string) bool {
	sawEntry := false
	for _, l := range lines {
		if l == "" {
			continue
		}
		if strings.HasPrefix(l, "##") {
			continue
		}
		if len(l) < 2 || !isPorcelainCode(l[0]) || !isPorcelainCode(l[1]) {
			return false
		}
		sawEntry = true
	}
	return sawEntry || hasBranchHeader(lines)
}

func hasBranchHeader(lines []string) bool {
	for _, l := range lines {
		if strings.HasPrefix(l, "##") {
			return true
		}
	}
	return false
}

func isPorcelainCode(c byte) bool {
	switch c {
	case ' ', 'M', 'A', 'D', 'R', 'C', 'U', 'T', '?', '!':
		return true
	default:
		return false
	}
}

func splitGitArgs(args []string) (globals []string, sub string, rest []string, ok bool) {
	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case a == "-C" || a == "-c" || a == "--git-dir" || a == "--work-tree":
			if i+1 >= len(args) {
				return nil, "", nil, false
			}
			globals = append(globals, a, args[i+1])
			i += 2
		case strings.HasPrefix(a, "--git-dir=") || strings.HasPrefix(a, "--work-tree="):
			globals = append(globals, a)
			i++
		case a == "--no-pager" || a == "--no-optional-locks" || a == "--bare" || a == "--literal-pathspecs":
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

func usesCompactStatusPath(args []string) bool {
	if len(args) == 0 {
		return true
	}
	sawBranch := false
	for _, a := range args {
		switch a {
		case "-b", "--branch":
			sawBranch = true
		case "-sb", "-bs":
			return true
		case "-s", "--short":
		default:
			return false
		}
	}
	return sawBranch
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
		n := len(branches)
		shown := branches
		if n > maxBranchList {
			shown = branches[:maxBranchList]
			sb.WriteString(fmt.Sprintf("local (%d): %s ... +%d more\n", n, strings.Join(shown, ", "), n-maxBranchList))
		} else {
			sb.WriteString(fmt.Sprintf("local (%d): %s\n", n, strings.Join(shown, ", ")))
		}
	}
	result := sb.String()
	if result == "" {
		return output
	}
	return result
}

func filterShow(output string) string {
	if strings.Contains(output, "diff --git") || strings.Contains(output, "@@") {
		return filterDiff(output)
	}
	return capLines(output, maxDiffLines)
}

func filterWrite(sub, output string, exitCode int) string {
	if exitCode != 0 {
		return output
	}
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return "ok\n"
	}
	switch sub {
	case "add":
		return "ok\n"
	case "commit":
		for _, line := range strings.Split(output, "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "[") {
				continue
			}
			end := strings.Index(line, "]")
			if end <= 1 {
				continue
			}
			inner := line[1:end]
			fields := strings.Fields(inner)
			if len(fields) >= 2 {
				return fmt.Sprintf("ok %s\n", fields[1])
			}
		}
		return "ok\n"
	case "push", "fetch":
		if len(trimmed) <= 80 {
			return trimmed + "\n"
		}
		return "ok\n"
	case "pull":
		for _, line := range strings.Split(output, "\n") {
			line = strings.TrimSpace(line)
			if strings.Contains(line, "file changed") || strings.Contains(line, "files changed") {
				return "ok " + line + "\n"
			}
		}
		return "ok\n"
	default:
		return output
	}
}

func filterStash(args []string, output string, exitCode int) string {
	if len(args) == 0 {
		return output
	}
	switch args[0] {
	case "list":
		return filterStashList(output)
	case "push", "pop", "apply", "drop", "clear":
		return filterWrite("stash", output, exitCode)
	default:
		return output
	}
}

func filterStashList(output string) string {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	var entries []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			entries = append(entries, l)
		}
	}
	if len(entries) <= maxStashEntries {
		return output
	}
	var sb strings.Builder
	for _, e := range entries[:maxStashEntries] {
		sb.WriteString(e)
		sb.WriteByte('\n')
	}
	sb.WriteString(fmt.Sprintf("... +%d more\n", len(entries)-maxStashEntries))
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
