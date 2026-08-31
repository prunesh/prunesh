// Module ls: filters `ls` output for Claude Code.
package ls

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/prunesh/prunesh/internal/registry"
)

const (
	maxSample    = 8
	maxExtGroups = 6
	minCompact   = 8
)

var months = []string{
	"Jan", "Feb", "Mar", "Apr", "May", "Jun",
	"Jul", "Aug", "Sep", "Oct", "Nov", "Dec",
}

var noiseDirs = map[string]struct{}{
	"node_modules": {},
	".git":         {},
	"target":       {},
	"dist":         {},
	"vendor":       {},
	"__pycache__":  {},
	".next":        {},
	"build":        {},
	"coverage":     {},
	".cache":       {},
}

func init() {
	registry.Register(&Module{})
}

type Module struct{}

func (m *Module) Name() string { return "ls" }

func (m *Module) ExtraEnv(_ []string) []string {
	return []string{"LC_ALL=C"}
}

func (m *Module) Rewrite(args []string) ([]string, bool) {
	for _, a := range args {
		if a == "--help" {
			return nil, false
		}
	}
	out := buildArgs(args)
	if sameArgs(out, args) {
		return nil, false
	}
	return out, true
}

func (m *Module) FilterOutput(args []string, output string, _ int) string {
	showAll := wantsAll(args)
	if isLongListing(output) {
		return compactLong(output, showAll)
	}
	return compactNames(output, showAll)
}

func (m *Module) TokensBefore(output string) int {
	return registry.EstimateTokens(output)
}

func (m *Module) TokensAfter(filtered string) int {
	return registry.EstimateTokens(filtered)
}

func buildArgs(args []string) []string {
	var flags, paths []string
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			flags = append(flags, a)
		} else {
			paths = append(paths, a)
		}
	}

	lead := "-l"
	if wantsAll(args) {
		lead = "-la"
	}
	out := []string{lead}
	for _, f := range flags {
		if f == "--all" {
			continue
		}
		if strings.HasPrefix(f, "--") {
			out = append(out, f)
			continue
		}
		stripped := strings.TrimLeft(f, "-")
		var extra strings.Builder
		for _, c := range stripped {
			if c != 'l' && c != 'a' && c != 'h' {
				extra.WriteRune(c)
			}
		}
		if extra.Len() > 0 {
			out = append(out, "-"+extra.String())
		}
	}
	return append(out, paths...)
}

func sameArgs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func wantsAll(args []string) bool {
	for _, a := range args {
		if a == "--all" {
			return true
		}
		if strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") && strings.ContainsRune(a, 'a') {
			return true
		}
	}
	return false
}

func isLongListing(output string) bool {
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "total ") {
			return true
		}
		if dateStart(line) >= 0 {
			return true
		}
	}
	return false
}

type entry struct {
	name string
	dir  bool
	size string
}

func compactLong(output string, showAll bool) string {
	var dirs, files []entry
	byExt := map[string]int{}
	parsed := 0
	sawContent := false

	for _, line := range strings.Split(output, "\n") {
		if line == "" || strings.HasPrefix(line, "total ") {
			continue
		}
		sawContent = true
		e, ok := parseLongLine(line)
		if !ok {
			continue
		}
		parsed++
		if !showAll && isNoise(e.name) {
			continue
		}
		if e.dir {
			dirs = append(dirs, e)
			continue
		}
		files = append(files, e)
		byExt[extOf(e.name)]++
	}

	if parsed == 0 && sawContent {
		return output
	}
	return render(dirs, files, byExt, output)
}

func compactNames(output string, showAll bool) string {
	var dirs, files []entry
	byExt := map[string]int{}

	for _, line := range strings.Split(strings.TrimRight(output, "\n"), "\n") {
		name := strings.TrimSpace(line)
		if name == "" || name == "." || name == ".." {
			continue
		}
		if !showAll && isNoise(strings.TrimSuffix(name, "/")) {
			continue
		}
		if strings.HasSuffix(name, "/") {
			dirs = append(dirs, entry{name: strings.TrimSuffix(name, "/"), dir: true})
			continue
		}
		files = append(files, entry{name: name})
		byExt[extOf(name)]++
	}

	return render(dirs, files, byExt, output)
}

func render(dirs, files []entry, byExt map[string]int, original string) string {
	total := len(dirs) + len(files)
	if total == 0 {
		return original
	}
	if total < minCompact {
		return original
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%d dirs, %d files\n", len(dirs), len(files)))

	if len(dirs) > 0 {
		sb.WriteString("dirs: ")
		n := len(dirs)
		if n > maxSample {
			n = maxSample
		}
		for i := 0; i < n; i++ {
			if i > 0 {
				sb.WriteByte(' ')
			}
			sb.WriteString(dirs[i].name)
			sb.WriteByte('/')
		}
		if len(dirs) > maxSample {
			sb.WriteString(fmt.Sprintf(" ... +%d more", len(dirs)-maxSample))
		}
		sb.WriteByte('\n')
	}

	if len(files) > 0 {
		type extGroup struct {
			ext   string
			count int
		}
		groups := make([]extGroup, 0, len(byExt))
		for ext, n := range byExt {
			groups = append(groups, extGroup{ext, n})
		}
		for i := 0; i < len(groups); i++ {
			for j := i + 1; j < len(groups); j++ {
				if groups[j].count > groups[i].count || (groups[j].count == groups[i].count && groups[j].ext < groups[i].ext) {
					groups[i], groups[j] = groups[j], groups[i]
				}
			}
		}

		shown := 0
		for _, g := range groups {
			if shown >= maxExtGroups {
				sb.WriteString(fmt.Sprintf("  +%d more ext\n", len(groups)-shown))
				break
			}
			label := g.ext
			if label == "" {
				label = "no-ext"
			} else {
				label = "." + label
			}
			sb.WriteString(fmt.Sprintf("  %s(%d):", label, g.count))
			written := 0
			for _, f := range files {
				if extOf(f.name) != g.ext {
					continue
				}
				if written >= maxSample {
					break
				}
				sb.WriteByte(' ')
				sb.WriteString(f.name)
				if f.size != "" {
					sb.WriteByte(' ')
					sb.WriteString(f.size)
				}
				written++
			}
			if g.count > maxSample {
				sb.WriteString(fmt.Sprintf(" ... +%d more", g.count-maxSample))
			}
			sb.WriteByte('\n')
			shown++
		}
	}

	result := sb.String()
	if len(result) >= len(original) {
		return original
	}
	return result
}

func parseLongLine(line string) (entry, bool) {
	start := dateStart(line)
	if start < 0 {
		return entry{}, false
	}
	nameStart := dateEnd(line, start)
	if nameStart < 0 || nameStart >= len(line) {
		return entry{}, false
	}
	name := line[nameStart:]
	if name == "." || name == ".." {
		return entry{}, false
	}

	before := strings.Fields(line[:start])
	if len(before) < 1 {
		return entry{}, false
	}
	perms := before[0]
	if len(perms) < 10 {
		return entry{}, false
	}

	var size uint64
	for i := len(before) - 1; i >= 1; i-- {
		n, err := strconv.ParseUint(before[i], 10, 64)
		if err == nil {
			size = n
			break
		}
	}

	return entry{
		name: name,
		dir:  perms[0] == 'd',
		size: humanSize(size),
	}, true
}

func dateStart(line string) int {
	for _, m := range months {
		needle := " " + m + " "
		if i := strings.Index(line, needle); i >= 0 {
			return i
		}
	}
	return -1
}

func dateEnd(line string, monthIdx int) int {
	rest := line[monthIdx+1:]
	fields := strings.Fields(rest)
	if len(fields) < 3 {
		return -1
	}
	consumed := 0
	for i := 0; i < 3; i++ {
		idx := strings.Index(rest[consumed:], fields[i])
		if idx < 0 {
			return -1
		}
		consumed += idx + len(fields[i])
	}
	if monthIdx+1+consumed < len(line) && line[monthIdx+1+consumed] == ' ' {
		consumed++
	}
	return monthIdx + 1 + consumed
}

func humanSize(n uint64) string {
	switch {
	case n >= 1048576:
		return fmt.Sprintf("%.1fM", float64(n)/1048576)
	case n >= 1024:
		return fmt.Sprintf("%.1fK", float64(n)/1024)
	default:
		return fmt.Sprintf("%dB", n)
	}
}

func isNoise(name string) bool {
	base := name
	if i := strings.LastIndex(name, "/"); i >= 0 {
		base = name[i+1:]
	}
	_, ok := noiseDirs[base]
	return ok
}

func extOf(name string) string {
	base := name
	if i := strings.LastIndex(name, "/"); i >= 0 {
		base = name[i+1:]
	}
	dot := strings.LastIndex(base, ".")
	if dot <= 0 || dot == len(base)-1 {
		return ""
	}
	return base[dot+1:]
}
