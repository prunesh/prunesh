// Module find: filters `find` output for Claude Code.
package find

import (
	"fmt"
	"sort"
	"strings"

	"github.com/prunesh/prunesh/internal/registry"
)

const (
	maxShown   = 50
	minCompact = 8
	maxExts    = 5
)

func init() {
	registry.Register(&Module{})
}

type Module struct{}

func (m *Module) Name() string { return "find" }

func (m *Module) Rewrite(args []string) ([]string, bool) {
	return nil, false
}

func (m *Module) FilterOutput(_ []string, output string, _ int) string {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	var paths []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			paths = append(paths, l)
		}
	}

	total := len(paths)
	if total == 0 || total < minCompact {
		return output
	}

	byDir := map[string][]string{}
	var dirOrder []string
	for _, p := range paths {
		dir, name := splitPath(p)
		if _, seen := byDir[dir]; !seen {
			dirOrder = append(dirOrder, dir)
		}
		byDir[dir] = append(byDir[dir], name)
	}
	sort.Strings(dirOrder)

	byExt := map[string]int{}
	for _, p := range paths {
		byExt[extOf(p)]++
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%d files, %d dirs\n\n", total, len(dirOrder)))

	shown := 0
	for _, dir := range dirOrder {
		if shown >= maxShown {
			break
		}
		names := byDir[dir]
		remaining := maxShown - shown
		display := dir
		if len(display) > 50 {
			display = "..." + display[len(display)-47:]
		}
		if !strings.HasSuffix(display, "/") {
			display += "/"
		}
		take := names
		if len(names) > remaining {
			take = names[:remaining]
		}
		sb.WriteString(display)
		sb.WriteByte(' ')
		sb.WriteString(strings.Join(take, " "))
		sb.WriteByte('\n')
		shown += len(take)
	}
	if shown < total {
		sb.WriteString(fmt.Sprintf("+%d more\n", total-shown))
	}

	if len(byExt) > 0 {
		type kv struct {
			ext string
			n   int
		}
		exts := make([]kv, 0, len(byExt))
		for ext, n := range byExt {
			exts = append(exts, kv{ext, n})
		}
		sort.Slice(exts, func(i, j int) bool {
			if exts[i].n != exts[j].n {
				return exts[i].n > exts[j].n
			}
			return exts[i].ext < exts[j].ext
		})
		sb.WriteByte('\n')
		sb.WriteString("ext: ")
		n := len(exts)
		if n > maxExts {
			n = maxExts
		}
		for i := 0; i < n; i++ {
			if i > 0 {
				sb.WriteByte(' ')
			}
			label := exts[i].ext
			if label == "" {
				label = "no-ext"
			}
			sb.WriteString(fmt.Sprintf(".%s(%d)", label, exts[i].n))
		}
		sb.WriteByte('\n')
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

func splitPath(p string) (dir, name string) {
	slash := strings.LastIndex(p, "/")
	if slash < 0 {
		return ".", p
	}
	dir = p[:slash]
	if dir == "" {
		dir = "/"
	}
	return dir, p[slash+1:]
}

func extOf(path string) string {
	slash := strings.LastIndex(path, "/")
	name := path
	if slash >= 0 {
		name = path[slash+1:]
	}
	dot := strings.LastIndex(name, ".")
	if dot <= 0 || dot == len(name)-1 {
		return ""
	}
	return name[dot+1:]
}
