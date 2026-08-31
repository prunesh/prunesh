package hook_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/prunesh/prunesh/internal/hook"
	"github.com/prunesh/prunesh/internal/registry"
)

// cursorMCPPayload builds a Cursor PostToolUse payload for an MCP tool.
// Cursor encodes tool_output as a JSON-encoded string containing a JSON array.
func cursorMCPPayload(toolName, text string) string {
	contents, _ := json.Marshal([]map[string]string{{"type": "text", "text": text}})
	output, _ := json.Marshal(string(contents))
	p, _ := json.Marshal(map[string]json.RawMessage{
		"tool_name":   json.RawMessage(`"` + toolName + `"`),
		"tool_input":  json.RawMessage(`{}`),
		"tool_output": output,
	})
	return string(p)
}

func runHookAgent(t *testing.T, payload string, agent hook.Agent) (modified bool, output string) {
	t.Helper()
	var out bytes.Buffer
	ok, err := hook.Run(strings.NewReader(payload), &out, agent)
	if err != nil {
		t.Fatalf("hook.Run(%s): %v", agent, err)
	}
	return ok, out.String()
}

func assertSavings(t *testing.T, label, before, after string, minPct float64) {
	t.Helper()
	tokBefore := registry.EstimateTokens(before)
	tokAfter := registry.EstimateTokens(after)
	saved := tokBefore - tokAfter
	pct := 0.0
	if tokBefore > 0 {
		pct = float64(saved) / float64(tokBefore) * 100
	}
	t.Logf("%s: %d tok → %d tok (saved %d, %.0f%%)", label, tokBefore, tokAfter, saved, pct)
	if pct < minPct {
		t.Errorf("%s: savings %.0f%% below minimum %.0f%%", label, pct, minPct)
	}
}

// ── ClaudeCode ────────────────────────────────────────────────────────────────

func TestSavingsClaudeCodeGitDiff(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("diff --git a/main.go b/main.go\nindex abc..def 100644\n")
	for i := 0; i < 400; i++ {
		fmt.Fprintf(&sb, "+\tfmt.Println(\"line %d added\")\n", i)
	}
	raw := sb.String()

	modified, out := runHookAgent(t, bashPayload("git diff", raw), hook.AgentClaudeCode)
	if !modified {
		t.Fatal("claudecode: expected git diff to be filtered")
	}

	var result struct {
		HookSpecificOutput struct {
			UpdatedOutput *string `json:"updatedOutput"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatal(err)
	}
	assertSavings(t, "claudecode/git-diff", raw, *result.HookSpecificOutput.UpdatedOutput, 40)
}

func TestSavingsClaudeCodeRead(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("package main\n\n")
	for i := 0; i < 120; i++ {
		fmt.Fprintf(&sb, "// comment explaining line %d\nvar v%d = %d\n\n", i, i, i)
	}
	raw := sb.String()

	modified, out := runHookAgent(t, readPayload("main.go", raw), hook.AgentClaudeCode)
	if !modified {
		t.Fatal("claudecode: expected go file read to be filtered")
	}

	var result struct {
		HookSpecificOutput struct {
			UpdatedMCPOutput []struct {
				Text string `json:"text"`
			} `json:"updatedMCPToolOutput"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatal(err)
	}
	assertSavings(t, "claudecode/read-go", raw, result.HookSpecificOutput.UpdatedMCPOutput[0].Text, 25)
}

func TestSavingsClaudeCodeMCP(t *testing.T) {
	raw := strings.Repeat("This is a long MCP tool response with lots of text. ", 100)

	modified, out := runHookAgent(t, mcpPayload("mcp__srv__query", raw), hook.AgentClaudeCode)
	if !modified {
		t.Fatal("claudecode: expected large MCP to be filtered")
	}

	compressed := extractMCPOutput(t, out)
	assertSavings(t, "claudecode/mcp", raw, compressed, 40)
}

// ── Cursor ────────────────────────────────────────────────────────────────────

func TestSavingsCursorMCP(t *testing.T) {
	raw := strings.Repeat("This is a long MCP tool response with lots of text. ", 100)

	modified, out := runHookAgent(t, cursorMCPPayload("MCP:query_data", raw), hook.AgentCursor)
	if !modified {
		t.Fatal("cursor: expected large MCP to be filtered")
	}

	var result struct {
		UpdatedMCPToolOutput []struct {
			Text string `json:"text"`
		} `json:"updated_mcp_tool_output"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("cursor: unmarshal: %v — output: %s", err, out)
	}
	if len(result.UpdatedMCPToolOutput) == 0 {
		t.Fatal("cursor: updated_mcp_tool_output is empty")
	}
	assertSavings(t, "cursor/mcp", raw, result.UpdatedMCPToolOutput[0].Text, 40)
}

func TestSavingsCursorReadIsNoop(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 200; i++ {
		fmt.Fprintf(&sb, "// comment line %d\nvar x = 1\n", i)
	}
	modified, _ := runHookAgent(t, readPayload("main.go", sb.String()), hook.AgentCursor)
	if modified {
		t.Error("cursor: Read must be a no-op in post hook (proxy handles it)")
	}
}

func TestSavingsCursorBashIsNoop(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 80; i++ {
		fmt.Fprintf(&sb, "commit abc%04d\nAuthor: Dev <dev@example.com>\n\n    fix: change %d\n\n", i, i)
	}
	modified, _ := runHookAgent(t, bashPayload("git log", sb.String()), hook.AgentCursor)
	if modified {
		t.Error("cursor: bash must be a no-op in post hook (proxy handles it)")
	}
}

// ── OpenCode ──────────────────────────────────────────────────────────────────

func TestSavingsOpenCodeRead(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("package main\n\n")
	for i := 0; i < 120; i++ {
		fmt.Fprintf(&sb, "// comment explaining line %d\nvar v%d = %d\n\n", i, i, i)
	}
	raw := sb.String()

	modified, out := runHookAgent(t, readPayload("main.go", raw), hook.AgentOpenCode)
	if !modified {
		t.Fatal("opencode: expected go file read to be filtered")
	}

	var result struct {
		Output string `json:"output"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("opencode: unmarshal: %v — output: %s", err, out)
	}
	assertSavings(t, "opencode/read-go", raw, result.Output, 25)
}

func TestSavingsOpenCodeMCP(t *testing.T) {
	raw := strings.Repeat("This is a long MCP tool response with lots of text. ", 100)

	modified, out := runHookAgent(t, mcpPayload("mcp__srv__query", raw), hook.AgentOpenCode)
	if !modified {
		t.Fatal("opencode: expected large MCP to be filtered")
	}

	var result struct {
		Output string `json:"output"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("opencode: unmarshal: %v", err)
	}
	assertSavings(t, "opencode/mcp", raw, result.Output, 40)
}

// ── Codex ─────────────────────────────────────────────────────────────────────

func TestSavingsCodexIsAlwaysNoop(t *testing.T) {
	scenarios := []struct {
		name    string
		payload string
	}{
		{"bash/git-log", bashPayload("git log", strings.Repeat("commit abc\nAuthor: x\n\n    msg\n\n", 80))},
		{"read/go-file", readPayload("main.go", strings.Repeat("// comment\nvar x = 1\n", 100))},
		{"mcp/large", mcpPayload("mcp__srv__query", strings.Repeat("long text ", 200))},
	}
	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			modified, _ := runHookAgent(t, s.payload, hook.AgentCodex)
			if modified {
				t.Errorf("codex/%s: must be no-op (codex has no post hook)", s.name)
			}
		})
	}
}
