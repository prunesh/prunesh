package hook_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/prunesh/prunesh/internal/hook"
)

func TestParseAgent(t *testing.T) {
	got, err := hook.ParseAgent("cursor")
	if err != nil {
		t.Fatal(err)
	}
	if got != hook.AgentCursor {
		t.Fatalf("got %q", got)
	}
}

func TestParseAgentRejectsEmpty(t *testing.T) {
	if _, err := hook.ParseAgent(""); err == nil {
		t.Fatal("empty agent must fail")
	}
}

func TestParseAgentRejectsUnknown(t *testing.T) {
	if _, err := hook.ParseAgent("windsurf"); err == nil {
		t.Fatal("unknown agent must fail")
	}
}

func TestPostCodexIsNoop(t *testing.T) {
	payload := mcpPayload("mcp__srv__query", strings.Repeat("x", 5000))
	var out strings.Builder
	ok, err := hook.Run(strings.NewReader(payload), &out, hook.AgentCodex)
	if err != nil {
		t.Fatal(err)
	}
	if ok || out.Len() != 0 {
		t.Fatalf("codex post must pass through, ok=%v out=%q", ok, out.String())
	}
}

func TestPostOpenCodeRead(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("package main\n\n")
	for i := 0; i < 100; i++ {
		sb.WriteString("// comment line that should be stripped\n")
		sb.WriteString("var x = 1\n")
	}
	payload := readPayload("main.go", sb.String())
	var out strings.Builder
	ok, err := hook.Run(strings.NewReader(payload), &out, hook.AgentOpenCode)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected opencode read filter")
	}
	if !strings.Contains(out.String(), `"output"`) {
		t.Fatalf("expected opencode output field: %s", out.String())
	}
	if strings.Contains(out.String(), "comment line that should be stripped") {
		t.Fatal("comments should be stripped")
	}
}

func TestPostCursorMCP(t *testing.T) {
	raw := strings.Repeat("This is a long MCP tool response with lots of text. ", 100)
	contents, err := json.Marshal([]map[string]string{{"type": "text", "text": raw}})
	if err != nil {
		t.Fatal(err)
	}
	output, err := json.Marshal(string(contents))
	if err != nil {
		t.Fatal(err)
	}
	payload := `{"tool_name":"MCP:query_data","tool_input":{},"tool_output":` + string(output) + `}`
	var out strings.Builder
	ok, err := hook.Run(strings.NewReader(payload), &out, hook.AgentCursor)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected cursor MCP truncation")
	}
	if !strings.Contains(out.String(), "updated_mcp_tool_output") {
		t.Fatalf("expected cursor MCP field: %s", out.String())
	}
}

func TestPostCursorSkipsRead(t *testing.T) {
	payload := readPayload("main.go", strings.Repeat("// c\nvar x = 1\n", 80))
	var out strings.Builder
	ok, err := hook.Run(strings.NewReader(payload), &out, hook.AgentCursor)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("cursor must not rewrite Read")
	}
}

func TestPostEmptyAgent(t *testing.T) {
	_, err := hook.Run(strings.NewReader(`{}`), &strings.Builder{}, "")
	if err == nil {
		t.Fatal("empty agent must fail")
	}
}
