// Package hook implements PreToolUse and PostToolUse handlers for coding agents.
package hook

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/prunesh/prunesh/internal/registry"
	"github.com/prunesh/prunesh/internal/text"
	readmod "github.com/prunesh/prunesh/plugins/read"
)

// ── Input structures ──────────────────────────────────────────────────────────

type bashResponse struct {
	Stdout      string `json:"stdout"`
	Interrupted bool   `json:"interrupted"`
}

type textContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type hookInput struct {
	ToolName   string          `json:"tool_name"`
	ToolInput  json.RawMessage `json:"tool_input"`
	ToolResp   json.RawMessage `json:"tool_response"`
	ToolOutput json.RawMessage `json:"tool_output"`
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
	HookEventName    string        `json:"hookEventName"`
	UpdatedOutput    *string       `json:"updatedOutput,omitempty"`
	UpdatedMCPOutput []textContent `json:"updatedMCPToolOutput,omitempty"`
}

type cursorPostOutput struct {
	UpdatedMCPToolOutput []textContent `json:"updated_mcp_tool_output,omitempty"`
}

type openCodePostOutput struct {
	Output string `json:"output"`
}

// ── MCP passthrough patterns ──────────────────────────────────────────────────

const mcpMaxChars = 3000

func passthroughPatterns() []string {
	raw := os.Getenv("PRUNESH_MCP_PASSTHROUGH_PATTERNS")
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
// agent selects the stdout JSON contract of the target coding agent.
func Run(r io.Reader, w io.Writer, agent Agent) (bool, error) {
	if agent == "" {
		return false, fmt.Errorf("agent is empty")
	}
	if agent == AgentCodex {
		return false, nil
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return false, fmt.Errorf("read stdin: %w", err)
	}

	var input hookInput
	if err := json.Unmarshal(data, &input); err != nil {
		return false, nil // not valid JSON, pass through
	}

	switch {
	case isShellTool(input.ToolName):
		return handleBash(input, w, agent)
	case input.ToolName == "Read" || input.ToolName == "read":
		return handleRead(input, w, agent)
	case strings.HasPrefix(input.ToolName, "mcp__") || strings.HasPrefix(input.ToolName, "MCP:"):
		return handleMCP(input, w, agent)
	}
	return false, nil
}

// ── Bash handler ──────────────────────────────────────────────────────────────

func handleBash(input hookInput, w io.Writer, agent Agent) (bool, error) {
	if agent != AgentClaudeCode {
		return false, nil
	}

	var bi bashInput
	if err := json.Unmarshal(input.ToolInput, &bi); err != nil {
		return false, nil
	}

	var resp bashResponse
	if err := json.Unmarshal(input.ToolResp, &resp); err != nil {
		return false, nil
	}
	if resp.Stdout == "" {
		return false, nil
	}

	filtered, changed := filterBashOutput(bi.Command, resp.Stdout)
	if !changed {
		return false, nil
	}

	return writeOutput(w, agent, &filtered, nil)
}

func responseBody(input hookInput, agent Agent) json.RawMessage {
	switch agent {
	case AgentCursor:
		return input.ToolOutput
	default:
		return input.ToolResp
	}
}

func filterBashOutput(command, output string) (string, bool) {
	// Extract the base command (first word, strip path)
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return output, false
	}

	base := fields[0]
	if idx := strings.LastIndex(base, "/"); idx >= 0 {
		base = base[idx+1:]
	}
	if base == "prunesh" {
		return output, false
	}

	mod := registry.Get(base)
	if mod == nil {
		return output, false
	}

	stripped := text.StripANSI(output)
	filtered := mod.FilterOutput(fields[1:], stripped, -1)
	shown := registry.NeverWorse(output, filtered)
	return shown, shown != output
}

// ── MCP handler ───────────────────────────────────────────────────────────────

func handleMCP(input hookInput, w io.Writer, agent Agent) (bool, error) {
	toolName := mcpBareName(input.ToolName)
	if matchesPassthrough(toolName, passthroughPatterns()) {
		return false, nil
	}

	contents, ok := parseTextContents(responseBody(input, agent), agent)
	if !ok {
		return false, nil
	}

	modified := false
	for i, c := range contents {
		if c.Type == "text" && len(c.Text) > mcpMaxChars {
			contents[i].Text = c.Text[:mcpMaxChars] +
				fmt.Sprintf("\n... [prunesh: truncated %d chars]", len(c.Text)-mcpMaxChars)
			modified = true
		}
	}

	if !modified {
		return false, nil
	}

	return writeOutput(w, agent, nil, contents)
}

func readFilePath(toolInput json.RawMessage) (string, bool) {
	var in readInput
	if err := json.Unmarshal(toolInput, &in); err != nil || in.FilePath == "" {
		return "", false
	}
	return in.FilePath, true
}

func mcpBareName(toolName string) string {
	if strings.HasPrefix(toolName, "MCP:") {
		return strings.TrimPrefix(toolName, "MCP:")
	}
	parts := strings.SplitN(toolName, "__", 3)
	if len(parts) == 3 {
		return parts[2]
	}
	return toolName
}

func parseTextContents(body json.RawMessage, agent Agent) ([]textContent, bool) {
	if len(body) == 0 {
		return nil, false
	}
	var contents []textContent
	if err := json.Unmarshal(body, &contents); err == nil && len(contents) > 0 {
		return contents, true
	}
	if agent == AgentCursor {
		var s string
		if err := json.Unmarshal(body, &s); err == nil && s != "" {
			if err := json.Unmarshal([]byte(s), &contents); err == nil && len(contents) > 0 {
				return contents, true
			}
		}
	}
	return nil, false
}

// ── Read handler ──────────────────────────────────────────────────────────────

func handleRead(input hookInput, w io.Writer, agent Agent) (bool, error) {
	if agent == AgentCursor {
		return false, nil
	}

	filePath, ok := readFilePath(input.ToolInput)
	if !ok {
		return false, nil
	}

	contents, ok := parseTextContents(responseBody(input, agent), agent)
	if !ok {
		return false, nil
	}

	modified := false
	for i, c := range contents {
		if c.Type != "text" || c.Text == "" {
			continue
		}
		filtered, changed := readmod.FilterContent(filePath, c.Text)
		if changed {
			contents[i].Text = filtered
			modified = true
		}
	}

	if !modified {
		return false, nil
	}

	return writeOutput(w, agent, nil, contents)
}

// ── Output writer ─────────────────────────────────────────────────────────────

func writeOutput(w io.Writer, agent Agent, bashOut *string, mcpOut []textContent) (bool, error) {
	out, err := marshalPost(agent, bashOut, mcpOut)
	if err != nil {
		return false, err
	}
	_, err = fmt.Fprintln(w, string(out))
	return true, err
}

func marshalPost(agent Agent, bashOut *string, mcpOut []textContent) ([]byte, error) {
	switch agent {
	case AgentClaudeCode:
		spec := hookSpecific{HookEventName: "PostToolUse"}
		if bashOut != nil {
			spec.UpdatedOutput = bashOut
		} else {
			spec.UpdatedMCPOutput = mcpOut
		}
		return json.Marshal(hookOutput{HookSpecificOutput: spec})
	case AgentCursor:
		if len(mcpOut) == 0 {
			return nil, fmt.Errorf("cursor post output requires MCP content")
		}
		return json.Marshal(cursorPostOutput{UpdatedMCPToolOutput: mcpOut})
	case AgentOpenCode:
		text := ""
		if bashOut != nil {
			text = *bashOut
		} else if len(mcpOut) > 0 {
			text = mcpOut[0].Text
		}
		if text == "" {
			return nil, fmt.Errorf("opencode post output is empty")
		}
		return json.Marshal(openCodePostOutput{Output: text})
	default:
		return nil, fmt.Errorf("unknown agent %q", agent)
	}
}
