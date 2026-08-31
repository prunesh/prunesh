// Package read filters Claude Code Read tool output to reduce token usage.
// Strips comment-only lines, collapses blank runs, and truncates large files.
// Language is detected from the file extension. No external dependencies.
package read

import (
	"fmt"
	"path/filepath"
	"strings"
)

const (
	maxLines  = 300
	headLines = 200
)

// FilterContent filters file content to reduce token usage.
// Returns (filtered, changed). Safe to call on any content — falls back to
// original if nothing changes or language is unknown.
func FilterContent(path, content string) (string, bool) {
	if content == "" {
		return content, false
	}

	lines := strings.Split(content, "\n")
	originalCount := len(lines)

	lang := detectLang(path)
	hasPrefix := detectLinePrefix(lines)

	if lang == langSlash {
		lines = stripBlockComments(lines, hasPrefix)
	}
	lines = stripCommentLines(lines, lang, hasPrefix)
	lines = collapseBlankLines(lines, hasPrefix)

	truncated := false
	if len(lines) > maxLines {
		remaining := len(lines) - headLines
		lines = append(lines[:headLines],
			fmt.Sprintf("... [prunesh: %d lines not shown]", remaining))
		truncated = true
	}

	if !truncated && len(lines) == originalCount {
		return content, false
	}

	return strings.Join(lines, "\n"), true
}

// ── Language detection ────────────────────────────────────────────────────────

type language int

const (
	langUnknown language = iota
	langSlash            // Go, Rust, JS, TS, C, C++, Java — // line comments
	langHash             // Python, Shell, Ruby — # line comments
	langData             // JSON, YAML, TOML, Markdown — no comment stripping
)

func detectLang(path string) language {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go", ".rs", ".js", ".mjs", ".cjs", ".ts", ".tsx", ".c", ".h", ".cpp", ".cc", ".java", ".css", ".scss", ".vue", ".svelte":
		return langSlash
	case ".py", ".pyw", ".sh", ".bash", ".zsh", ".rb":
		return langHash
	case ".json", ".jsonc", ".yaml", ".yml", ".toml", ".md", ".markdown",
		".txt", ".html", ".sql", ".lock", ".env":
		return langData
	default:
		return langUnknown
	}
}

// ── Line-number prefix detection ──────────────────────────────────────────────
//
// The Claude Code Read tool may return content with line-number prefixes
// in the format "     1→content" (spaces + digits + U+2192 arrow).
// We detect this and account for it when checking blank/comment lines.

func detectLinePrefix(lines []string) bool {
	for _, line := range lines {
		if line == "" {
			continue
		}
		return looksLikeNumberedLine(line)
	}
	return false
}

func looksLikeNumberedLine(line string) bool {
	i := 0
	for i < len(line) && line[i] == ' ' {
		i++
	}
	digits := 0
	for i < len(line) && line[i] >= '0' && line[i] <= '9' {
		digits++
		i++
	}
	if digits == 0 {
		return false
	}
	if i < len(line) {
		if line[i] == '\t' {
			return true
		}
		// U+2192 → encodes as 0xE2 0x86 0x92 in UTF-8
		if i+2 < len(line) && line[i] == 0xE2 && line[i+1] == 0x86 && line[i+2] == 0x92 {
			return true
		}
	}
	return false
}

// contentOf returns the part of a line after the line-number prefix, if any.
func contentOf(line string) string {
	i := 0
	for i < len(line) && line[i] == ' ' {
		i++
	}
	for i < len(line) && line[i] >= '0' && line[i] <= '9' {
		i++
	}
	if i < len(line) {
		if line[i] == '\t' {
			return line[i+1:]
		}
		if i+2 < len(line) && line[i] == 0xE2 && line[i+1] == 0x86 && line[i+2] == 0x92 {
			return line[i+3:]
		}
	}
	return line
}

// ── Filtering ────────────────────────────────────────────────────────────────

func isBlank(line string, hasPrefix bool) bool {
	content := line
	if hasPrefix {
		content = contentOf(line)
	}
	return strings.TrimSpace(content) == ""
}

func isCommentOnly(line string, lang language, hasPrefix bool) bool {
	if lang == langData || lang == langUnknown {
		return false
	}
	content := line
	if hasPrefix {
		content = contentOf(line)
	}
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return false
	}
	switch lang {
	case langSlash:
		return strings.HasPrefix(trimmed, "//")
	case langHash:
		return strings.HasPrefix(trimmed, "#")
	}
	return false
}

func stripCommentLines(lines []string, lang language, hasPrefix bool) []string {
	if lang == langData || lang == langUnknown {
		return lines
	}
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		if !isCommentOnly(line, lang, hasPrefix) {
			result = append(result, line)
		}
	}
	return result
}

func stripBlockComments(lines []string, hasPrefix bool) []string {
	inBlock := false
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		content := line
		prefix := ""
		if hasPrefix {
			content = contentOf(line)
			prefix = line[:len(line)-len(content)]
		}
		var kept strings.Builder
		i := 0
		for i < len(content) {
			if inBlock {
				end := strings.Index(content[i:], "*/")
				if end < 0 {
					i = len(content)
					break
				}
				i += end + 2
				inBlock = false
				continue
			}
			start := strings.Index(content[i:], "/*")
			if start < 0 {
				kept.WriteString(content[i:])
				break
			}
			kept.WriteString(content[i : i+start])
			i += start + 2
			inBlock = true
		}
		if kept.Len() > 0 {
			out = append(out, prefix+kept.String())
		} else if !inBlock && strings.TrimSpace(content) != "" {
			out = append(out, line)
		}
	}
	return out
}

func collapseBlankLines(lines []string, hasPrefix bool) []string {
	result := make([]string, 0, len(lines))
	prevBlank := false
	for _, line := range lines {
		blank := isBlank(line, hasPrefix)
		if blank && prevBlank {
			continue
		}
		result = append(result, line)
		prevBlank = blank
	}
	return result
}
