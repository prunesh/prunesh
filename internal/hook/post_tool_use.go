// Package hook implements the PostToolUse handler for Claude Code.
// A single hook process handles Bash output filtering, Read filtering, and MCP passthrough.
package hook

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jmeiracorbal/gtk-ai/internal/registry"
	gitmod "github.com/jmeiracorbal/gtk-ai/modules/git"
	readmod "github.com/jmeiracorbal/gtk-ai/modules/read"
)

// ── Input structures ──────────────────────────────────────────────────────────

type bashResponse struct {
	Output      string `json:"output"`
	Interrupted bool   `json:"interrupted"`
}

type textContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type hookInput struct {
	ToolName  string          `json:"tool_name"`
	ToolInput json.RawMessage `json:"tool_input"`
	ToolResp  json.RawMessage `json:"tool_response"`
}

type bashInput struct {
	Command string `json:"command"`
}

type readInput struct {
	FilePath string `json:"file_path"`
}

// ── Output structures ─────────────────────────────────────────────────────────

type hookOutput struct {
	HookSpecificOutput hookSpecific `json:"hookSpecificOutput"`
}

type hookSpecific struct {
	HookEventName      string        `json:"hookEventName"`
	UpdatedOutput      *string       `json:"updatedOutput,omitempty"`
	UpdatedMCPOutput   []textContent `json:"updatedMCPToolOutput,omitempty"`
}

// ── MCP passthrough patterns ──────────────────────────────────────────────────

const mcpMaxChars = 3000

func passthroughPatterns() []string {
	raw := os.Getenv("GTK_MCP_PASSTHROUGH_PATTERNS")
	if raw == "" {
		return []string{}
	}
	var out []string
	for _, p := range strings.Split(raw, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func matchesPassthrough(toolName string, patterns []string) bool {
	for _, p := range patterns {
		if strings.HasSuffix(p, "*") {
			if strings.HasPrefix(toolName, strings.TrimSuffix(p, "*")) {
				return true
			}
		} else if p == toolName {
			return true
		}
	}
	return false
}

// ── Run ───────────────────────────────────────────────────────────────────────

// Run reads a PostToolUse event from r, applies filtering if needed, writes result to w.
// Returns true if output was modified.
func Run(r io.Reader, w io.Writer) (bool, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return false, fmt.Errorf("read stdin: %w", err)
	}

	var input hookInput
	if err := json.Unmarshal(data, &input); err != nil {
		return false, nil // not valid JSON, pass through
	}

	switch {
	case input.ToolName == "Bash":
		return handleBash(input, w)
	case input.ToolName == "Read":
		return handleRead(input, w)
	case strings.HasPrefix(input.ToolName, "mcp__"):
		return handleMCP(input, w)
	}
	return false, nil
}

// ── Bash handler ──────────────────────────────────────────────────────────────

func handleBash(input hookInput, w io.Writer) (bool, error) {
	// Extract the command that was run
	var bi bashInput
	if err := json.Unmarshal(input.ToolInput, &bi); err != nil {
		return false, nil
	}

	// Extract output
	var resp bashResponse
	if err := json.Unmarshal(input.ToolResp, &resp); err != nil {
		return false, nil
	}
	if resp.Output == "" {
		return false, nil
	}

	// Detect which module applies
	filtered, changed := filterBashOutput(bi.Command, resp.Output)
	if !changed {
		return false, nil
	}

	return writeOutput(w, &filtered, nil)
}

func filterBashOutput(command, output string) (string, bool) {
	// Extract the base command (first word, strip path)
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return output, false
	}

	base := fields[0]
	// Strip path prefix if any (e.g. /usr/bin/find → find)
	if idx := strings.LastIndex(base, "/"); idx >= 0 {
		base = base[idx+1:]
	}

	// Special case: git needs the subcommand
	if base == "git" {
		subcommand := ""
		for _, f := range fields[1:] {
			if !strings.HasPrefix(f, "-") {
				subcommand = f
				break
			}
		}
		filtered := gitmod.FilterOutputWithArgs(subcommand, output)
		return filtered, filtered != output
	}

	mod := registry.Get(base)
	if mod == nil {
		return output, false
	}

	filtered := mod.FilterOutput(output)
	return filtered, filtered != output
}

// ── MCP handler ───────────────────────────────────────────────────────────────

func handleMCP(input hookInput, w io.Writer) (bool, error) {
	// Extract tool name from mcp__server__tool_name
	parts := strings.SplitN(input.ToolName, "__", 3)
	toolName := ""
	if len(parts) == 3 {
		toolName = parts[2]
	}

	// Passthrough: don't filter these tools
	if matchesPassthrough(toolName, passthroughPatterns()) {
		return false, nil
	}

	var contents []textContent
	if err := json.Unmarshal(input.ToolResp, &contents); err != nil {
		return false, nil
	}

	modified := false
	for i, c := range contents {
		if c.Type == "text" && len(c.Text) > mcpMaxChars {
			contents[i].Text = c.Text[:mcpMaxChars] +
				fmt.Sprintf("\n... [gtkai: truncated %d chars]", len(c.Text)-mcpMaxChars)
			modified = true
		}
	}

	if !modified {
		return false, nil
	}

	return writeOutput(w, nil, contents)
}

// ── Read handler ──────────────────────────────────────────────────────────────

func handleRead(input hookInput, w io.Writer) (bool, error) {
	var ri readInput
	if err := json.Unmarshal(input.ToolInput, &ri); err != nil {
		return false, nil
	}

	var contents []textContent
	if err := json.Unmarshal(input.ToolResp, &contents); err != nil {
		return false, nil
	}

	modified := false
	for i, c := range contents {
		if c.Type != "text" || c.Text == "" {
			continue
		}
		filtered, changed := readmod.FilterContent(ri.FilePath, c.Text)
		if changed {
			contents[i].Text = filtered
			modified = true
		}
	}

	if !modified {
		return false, nil
	}

	return writeOutput(w, nil, contents)
}

// ── Output writer ─────────────────────────────────────────────────────────────

func writeOutput(w io.Writer, bashOut *string, mcpOut []textContent) (bool, error) {
	spec := hookSpecific{HookEventName: "PostToolUse"}
	if bashOut != nil {
		spec.UpdatedOutput = bashOut
	} else {
		spec.UpdatedMCPOutput = mcpOut
	}

	out, err := json.Marshal(hookOutput{HookSpecificOutput: spec})
	if err != nil {
		return false, fmt.Errorf("marshal: %w", err)
	}
	_, err = fmt.Fprintln(w, string(out))
	return true, err
}
