// Package shell rewrites agent Bash commands to prunesh proxy invocations.
package shell

import (
	"strings"
	"unicode"

	"github.com/prunesh/prunesh/internal/pluginregistry"
	"github.com/prunesh/prunesh/internal/registry"
)

type tokenKind int

const (
	tokWord tokenKind = iota
	tokAnd
	tokOr
	tokSemi
	tokPipe
	tokPipeAnd
)

type token struct {
	kind tokenKind
	val  string
}

// Rewrite inserts pruneshBin before a registered command.
// pruneshBin must be non-empty; the caller supplies it.
func Rewrite(cmd, pruneshBin string) (string, bool) {
	if pruneshBin == "" {
		return "", false
	}
	if strings.Contains(cmd, "<<") || strings.Contains(cmd, "$(") || strings.Contains(cmd, "`") {
		return "", false
	}

	toks, ok := tokenize(cmd)
	if !ok || len(toks) == 0 {
		return "", false
	}

	clauses := splitClauses(toks)
	changed := false
	var out []token
	for i, cl := range clauses {
		if i > 0 {
			out = append(out, cl.sep)
		}
		rw, did := rewriteClause(cl.toks, pruneshBin)
		if did {
			changed = true
			out = append(out, rw...)
		} else {
			out = append(out, cl.toks...)
		}
	}
	if !changed {
		return "", false
	}
	return join(out), true
}

type clause struct {
	sep  token
	toks []token
}

func splitClauses(toks []token) []clause {
	var clauses []clause
	var cur []token
	sep := token{}
	first := true
	flush := func() {
		if len(cur) == 0 {
			return
		}
		c := clause{toks: cur}
		if !first {
			c.sep = sep
		}
		clauses = append(clauses, c)
		cur = nil
		first = false
	}
	for _, t := range toks {
		switch t.kind {
		case tokAnd, tokOr, tokSemi:
			flush()
			sep = t
			first = false
			cur = nil
		default:
			cur = append(cur, t)
		}
	}
	flush()
	return clauses
}

func rewriteClause(toks []token, pruneshBin string) ([]token, bool) {
	stages, seps := splitPipeline(toks)
	if len(stages) == 0 {
		return nil, false
	}
	if len(stages) == 1 {
		return rewriteSimple(stages[0], pruneshBin)
	}

	last := stages[len(stages)-1]
	name := commandName(last)
	if name != "grep" && name != "rg" {
		return nil, false
	}
	rw, ok := rewriteSimple(last, pruneshBin)
	if !ok {
		return nil, false
	}
	var out []token
	for i, st := range stages {
		if i > 0 {
			out = append(out, seps[i-1])
		}
		if i == len(stages)-1 {
			out = append(out, rw...)
		} else {
			out = append(out, st...)
		}
	}
	return out, true
}

func splitPipeline(toks []token) (stages [][]token, seps []token) {
	var cur []token
	for _, t := range toks {
		if t.kind == tokPipe || t.kind == tokPipeAnd {
			stages = append(stages, cur)
			seps = append(seps, t)
			cur = nil
			continue
		}
		cur = append(cur, t)
	}
	stages = append(stages, cur)
	return stages, seps
}

func rewriteSimple(toks []token, pruneshBin string) ([]token, bool) {
	words := wordVals(toks)
	if len(words) == 0 {
		return nil, false
	}

	i := 0
	for i < len(words) {
		w := words[i]
		if isAssign(w) || w == "sudo" || w == "env" {
			i++
			continue
		}
		break
	}
	if i >= len(words) {
		return nil, false
	}

	base := basename(words[i])
	if base == "prunesh" {
		return nil, false
	}
	if registry.Get(base) == nil && !pluginregistry.HasActive(base) {
		return nil, false
	}

	out := make([]token, 0, len(toks)+2)
	wi := 0
	for _, t := range toks {
		if t.kind != tokWord {
			out = append(out, t)
			continue
		}
		if wi == i {
			out = append(out, token{kind: tokWord, val: pruneshBin})
			out = append(out, token{kind: tokWord, val: base})
			wi++
			continue
		}
		out = append(out, t)
		wi++
	}
	return out, true
}

func commandName(toks []token) string {
	words := wordVals(toks)
	i := 0
	for i < len(words) {
		w := words[i]
		if isAssign(w) || w == "sudo" || w == "env" {
			i++
			continue
		}
		break
	}
	if i >= len(words) {
		return ""
	}
	return basename(words[i])
}

func wordVals(toks []token) []string {
	var w []string
	for _, t := range toks {
		if t.kind == tokWord {
			w = append(w, t.val)
		}
	}
	return w
}

func isAssign(s string) bool {
	eq := strings.IndexByte(s, '=')
	if eq <= 0 {
		return false
	}
	name := s[:eq]
	if name[0] != '_' && (name[0] < 'A' || name[0] > 'Z') && (name[0] < 'a' || name[0] > 'z') {
		return false
	}
	for _, c := range name[1:] {
		if c != '_' && (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') && (c < '0' || c > '9') {
			return false
		}
	}
	return true
}

func basename(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

func join(toks []token) string {
	var b strings.Builder
	for i, t := range toks {
		if i > 0 {
			b.WriteByte(' ')
		}
		switch t.kind {
		case tokAnd:
			b.WriteString("&&")
		case tokOr:
			b.WriteString("||")
		case tokSemi:
			b.WriteString(";")
		case tokPipe:
			b.WriteString("|")
		case tokPipeAnd:
			b.WriteString("|&")
		default:
			b.WriteString(shellQuote(t.val))
		}
	}
	return b.String()
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	safe := true
	for _, r := range s {
		if r > 127 || !(unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("/._-+=:@,.", r)) {
			safe = false
			break
		}
	}
	if safe {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

func tokenize(cmd string) ([]token, bool) {
	var toks []token
	i := 0
	n := len(cmd)
	for i < n {
		for i < n && (cmd[i] == ' ' || cmd[i] == '\t' || cmd[i] == '\n') {
			i++
		}
		if i >= n {
			break
		}
		if cmd[i] == '>' || cmd[i] == '<' {
			return nil, false
		}
		if cmd[i] == '&' {
			if i+1 < n && cmd[i+1] == '&' {
				toks = append(toks, token{kind: tokAnd})
				i += 2
				continue
			}
			return nil, false
		}
		if cmd[i] == '|' {
			if i+1 < n && cmd[i+1] == '|' {
				toks = append(toks, token{kind: tokOr})
				i += 2
				continue
			}
			if i+1 < n && cmd[i+1] == '&' {
				toks = append(toks, token{kind: tokPipeAnd})
				i += 2
				continue
			}
			toks = append(toks, token{kind: tokPipe})
			i++
			continue
		}
		if cmd[i] == ';' {
			toks = append(toks, token{kind: tokSemi})
			i++
			continue
		}
		word, next, ok := readWord(cmd, i)
		if !ok {
			return nil, false
		}
		toks = append(toks, token{kind: tokWord, val: word})
		i = next
	}
	return toks, true
}

func readWord(s string, i int) (string, int, bool) {
	var b strings.Builder
	n := len(s)
	for i < n {
		c := s[i]
		if c == ' ' || c == '\t' || c == '\n' || c == ';' || c == '|' || c == '&' || c == '>' || c == '<' {
			break
		}
		if c == '\\' {
			if i+1 >= n {
				return "", 0, false
			}
			b.WriteByte(s[i+1])
			i += 2
			continue
		}
		if c == '\'' {
			i++
			for i < n && s[i] != '\'' {
				b.WriteByte(s[i])
				i++
			}
			if i >= n {
				return "", 0, false
			}
			i++
			continue
		}
		if c == '"' {
			i++
			for i < n && s[i] != '"' {
				if s[i] == '\\' && i+1 < n {
					b.WriteByte(s[i+1])
					i += 2
					continue
				}
				b.WriteByte(s[i])
				i++
			}
			if i >= n {
				return "", 0, false
			}
			i++
			continue
		}
		b.WriteByte(c)
		i++
	}
	if b.Len() == 0 {
		return "", 0, false
	}
	return b.String(), i, true
}
