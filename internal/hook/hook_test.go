package hook_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/jmeiracorbal/gtk-ai/internal/hook"
	"github.com/jmeiracorbal/gtk-ai/internal/registry"

	_ "github.com/jmeiracorbal/gtk-ai/modules/find"
	_ "github.com/jmeiracorbal/gtk-ai/modules/git"
	_ "github.com/jmeiracorbal/gtk-ai/modules/grep"
	_ "github.com/jmeiracorbal/gtk-ai/modules/ls"
)

// ── Helpers ───────────────────────────────────────────────────────────────────

func bashPayload(command, output string) string {
	toolInput, _ := json.Marshal(map[string]string{"command": command})
	toolResp, _ := json.Marshal(map[string]interface{}{"stdout": output, "stderr": "", "interrupted": false})
	p, _ := json.Marshal(map[string]json.RawMessage{
		"tool_name":     json.RawMessage(`"Bash"`),
		"tool_input":    toolInput,
		"tool_response": toolResp,
	})
	return string(p)
}

func readPayload(filePath, content string) string {
	toolInput, _ := json.Marshal(map[string]string{"file_path": filePath})
	toolResp, _ := json.Marshal([]map[string]string{{"type": "text", "text": content}})
	p, _ := json.Marshal(map[string]json.RawMessage{
		"tool_name":     json.RawMessage(`"Read"`),
		"tool_input":    toolInput,
		"tool_response": toolResp,
	})
	return string(p)
}

func mcpPayload(toolName, text string) string {
	toolResp, _ := json.Marshal([]map[string]string{{"type": "text", "text": text}})
	name, _ := json.Marshal(toolName)
	p, _ := json.Marshal(map[string]json.RawMessage{
		"tool_name":     name,
		"tool_input":    json.RawMessage(`{}`),
		"tool_response": toolResp,
	})
	return string(p)
}

func runHook(t *testing.T, payload string) (modified bool, output string) {
	t.Helper()
	var out bytes.Buffer
	ok, err := hook.Run(strings.NewReader(payload), &out)
	if err != nil {
		t.Fatalf("hook.Run error: %v", err)
	}
	return ok, out.String()
}

func extractBashOutput(t *testing.T, raw string) string {
	t.Helper()
	var result struct {
		HookSpecificOutput struct {
			UpdatedOutput *string `json:"updatedOutput"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if result.HookSpecificOutput.UpdatedOutput == nil {
		t.Fatal("updatedOutput is nil")
	}
	return *result.HookSpecificOutput.UpdatedOutput
}

func extractMCPOutput(t *testing.T, raw string) string {
	t.Helper()
	var result struct {
		HookSpecificOutput struct {
			UpdatedMCPOutput []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"updatedMCPToolOutput"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if len(result.HookSpecificOutput.UpdatedMCPOutput) == 0 {
		t.Fatal("updatedMCPToolOutput is empty")
	}
	return result.HookSpecificOutput.UpdatedMCPOutput[0].Text
}

func reportGain(t *testing.T, label, before, after string) {
	t.Helper()
	tokBefore := registry.EstimateTokens(before)
	tokAfter := registry.EstimateTokens(after)
	saved := tokBefore - tokAfter
	pct := 0.0
	if tokBefore > 0 {
		pct = float64(saved) / float64(tokBefore) * 100
	}
	t.Logf("%s: %d tokens → %d tokens  (saved %d, %.0f%%)", label, tokBefore, tokAfter, saved, pct)
}

// ── find ─────────────────────────────────────────────────────────────────────

func TestFindCompression(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 150; i++ {
		fmt.Fprintf(&sb, "./src/module_%03d/handler.go\n", i)
	}
	raw := sb.String()

	modified, out := runHook(t, bashPayload("find . -name '*.go'", raw))
	if !modified {
		t.Fatal("expected hook to modify output")
	}

	compressed := extractBashOutput(t, out)
	reportGain(t, "find (150 paths)", raw, compressed)

	if len(compressed) >= len(raw) {
		t.Errorf("expected compressed output to be shorter than raw (%d >= %d)", len(compressed), len(raw))
	}
	if !strings.Contains(compressed, "150") {
		t.Error("expected result count in compressed output")
	}
	if !strings.Contains(compressed, ".go") {
		t.Error("expected extension summary in compressed output")
	}
}

func TestFindEmptyOutput(t *testing.T) {
	modified, out := runHook(t, bashPayload("find . -name '*.rs'", ""))
	// empty output: hook returns false (nothing to compress)
	if modified {
		t.Errorf("empty find output should not be modified, got: %s", out)
	}
}

func TestFindSmallOutput(t *testing.T) {
	// find always reformats output, even for small result sets
	raw := "./main.go\n./go.mod\n"
	modified, out := runHook(t, bashPayload("find . -name '*.go'", raw))
	if !modified {
		t.Fatal("find should always reformat output")
	}
	compressed := extractBashOutput(t, out)
	if !strings.Contains(compressed, "2") {
		t.Errorf("expected result count in output, got: %s", compressed)
	}
	if !strings.Contains(compressed, ".go") {
		t.Errorf("expected extension in output, got: %s", compressed)
	}
}

// ── ls ───────────────────────────────────────────────────────────────────────

func TestLsSmallOutput(t *testing.T) {
	// small output: reformatted result is larger than raw, so hook returns original
	lines := []string{
		"cmd/", "internal/", "modules/",
		"main.go", "go.mod", "go.sum", "README.md",
		"Makefile", "Dockerfile", "LICENSE",
	}
	raw := strings.Join(lines, "\n") + "\n"

	modified, _ := runHook(t, bashPayload("ls", raw))
	if modified {
		t.Error("small ls output should not be modified (reformatted result would be larger)")
	}
}

func TestLsLargeOutputNotModified(t *testing.T) {
	// ls always lists all filenames — reformatted output is always longer than raw.
	// The len(result) >= len(output) guard ensures the hook never expands output.
	var sb strings.Builder
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&sb, "handler_%02d.go\n", i)
	}
	for i := 0; i < 20; i++ {
		fmt.Fprintf(&sb, "handler_%02d_test.go\n", i)
	}
	for i := 0; i < 10; i++ {
		fmt.Fprintf(&sb, "module_%02d/\n", i)
	}
	raw := sb.String()

	modified, _ := runHook(t, bashPayload("ls", raw))
	if modified {
		t.Error("ls output should not be modified: reformatted result is always longer than raw")
	}
}

// ── grep ─────────────────────────────────────────────────────────────────────

func TestGrepCompression(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 250; i++ {
		fmt.Fprintf(&sb, "src/file_%03d.go:%d:    return fmt.Errorf(\"token error: %d\")\n", i%20, i+1, i)
	}
	raw := sb.String()

	modified, out := runHook(t, bashPayload("grep -rn 'Errorf' src/", raw))
	if !modified {
		t.Fatal("expected hook to modify output")
	}

	compressed := extractBashOutput(t, out)
	reportGain(t, "grep (250 matches)", raw, compressed)

	if len(compressed) >= len(raw) {
		t.Errorf("expected compressed output to be shorter (%d >= %d)", len(compressed), len(raw))
	}
	if !strings.Contains(compressed, "250") {
		t.Error("expected match count in compressed output")
	}
}

// ── git ──────────────────────────────────────────────────────────────────────

func TestGitDiffCompression(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("diff --git a/main.go b/main.go\n")
	sb.WriteString("index abc..def 100644\n")
	for i := 0; i < 400; i++ {
		fmt.Fprintf(&sb, "+\tfmt.Println(\"line %d added\")\n", i)
	}
	raw := sb.String()

	modified, out := runHook(t, bashPayload("git diff", raw))
	if !modified {
		t.Fatal("expected hook to modify output")
	}

	compressed := extractBashOutput(t, out)
	reportGain(t, "git diff (400 lines)", raw, compressed)

	if len(compressed) >= len(raw) {
		t.Errorf("expected compressed output to be shorter (%d >= %d)", len(compressed), len(raw))
	}
	if !strings.Contains(compressed, "truncated") {
		t.Error("expected truncation notice in output")
	}
}

func TestGitStatusSmallOutput(t *testing.T) {
	// small status: reformatted result would be larger, hook should return original
	raw := " M internal/hook/post_tool_use.go\n M README.md\n?? plugin/\n?? docs/\n"

	modified, _ := runHook(t, bashPayload("git status", raw))
	if modified {
		t.Error("small git status should not be modified (reformatted result would be larger)")
	}
}

func TestGitStatusCompression(t *testing.T) {
	// large status: many files should compress into category summary
	var sb strings.Builder
	for i := 0; i < 15; i++ {
		fmt.Fprintf(&sb, " M internal/module_%02d/handler.go\n", i)
	}
	for i := 0; i < 8; i++ {
		fmt.Fprintf(&sb, "A  internal/module_%02d/new_file.go\n", i)
	}
	for i := 0; i < 20; i++ {
		fmt.Fprintf(&sb, "?? tmp/cache_%02d.json\n", i)
	}
	raw := sb.String()

	modified, out := runHook(t, bashPayload("git status", raw))
	if !modified {
		t.Fatal("expected hook to compress large git status output")
	}

	compressed := extractBashOutput(t, out)
	reportGain(t, "git status (43 files)", raw, compressed)

	if len(compressed) >= len(raw) {
		t.Errorf("expected compressed output to be shorter (%d >= %d)", len(compressed), len(raw))
	}
	if !strings.Contains(compressed, "Modified") {
		t.Error("expected Modified section")
	}
	if !strings.Contains(compressed, "Staged") {
		t.Error("expected Staged section")
	}
	if !strings.Contains(compressed, "Untracked") {
		t.Error("expected Untracked section")
	}
}


func TestGitLogCompression(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 80; i++ {
		fmt.Fprintf(&sb, "commit abc%04d\nAuthor: Dev <dev@example.com>\nDate: Mon Jan 1 00:00:00 2024\n\n    fix: change %d\n\n", i, i)
	}
	raw := sb.String()

	modified, out := runHook(t, bashPayload("git log", raw))
	if !modified {
		t.Fatal("expected hook to modify git log output")
	}

	compressed := extractBashOutput(t, out)
	reportGain(t, "git log (80 commits)", raw, compressed)

	if len(compressed) >= len(raw) {
		t.Errorf("expected compressed output to be shorter (%d >= %d)", len(compressed), len(raw))
	}
}

// ── read ─────────────────────────────────────────────────────────────────────

func TestReadCompressionGoFile(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("package main\n\n")
	for i := 0; i < 100; i++ {
		fmt.Fprintf(&sb, "// This is a comment explaining line %d\n", i)
		fmt.Fprintf(&sb, "var v%d = %d\n\n", i, i)
	}
	raw := sb.String()

	modified, out := runHook(t, readPayload("main.go", raw))
	if !modified {
		t.Fatal("expected hook to modify Read output")
	}

	var result struct {
		HookSpecificOutput struct {
			UpdatedMCPOutput []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"updatedMCPToolOutput"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	compressed := result.HookSpecificOutput.UpdatedMCPOutput[0].Text
	reportGain(t, "read .go (100 commented vars)", raw, compressed)

	if strings.Contains(compressed, "// This is a comment") {
		t.Error("comment lines should have been stripped")
	}
	if !strings.Contains(compressed, "var v0") {
		t.Error("code lines should be preserved")
	}
}

func TestReadTruncation(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 400; i++ {
		fmt.Fprintf(&sb, "line %d: some content here\n", i)
	}
	raw := sb.String()

	modified, out := runHook(t, readPayload("data.txt", raw))
	if !modified {
		t.Fatal("400-line file should always be truncated regardless of language detection")
	}

	var result struct {
		HookSpecificOutput struct {
			UpdatedMCPOutput []struct {
				Text string `json:"text"`
			} `json:"updatedMCPToolOutput"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(result.HookSpecificOutput.UpdatedMCPOutput) == 0 {
		t.Fatal("expected updatedMCPToolOutput in hook response for truncated read")
	}
	compressed := result.HookSpecificOutput.UpdatedMCPOutput[0].Text
	reportGain(t, "read plain text (400 lines)", raw, compressed)

	if !strings.Contains(compressed, "not shown") {
		t.Error("expected truncation notice in output")
	}
}

// ── MCP ──────────────────────────────────────────────────────────────────────

func TestMCPCompression(t *testing.T) {
	raw := strings.Repeat("This is a long MCP tool response with lots of text. ", 100)

	modified, out := runHook(t, mcpPayload("mcp__myserver__query_data", raw))
	if !modified {
		t.Fatal("expected hook to modify large MCP output")
	}

	compressed := extractMCPOutput(t, out)
	reportGain(t, "mcp (5200 chars)", raw, compressed)

	if len(compressed) > 3100 {
		t.Errorf("expected MCP output truncated to ~3000 chars, got %d", len(compressed))
	}
	if !strings.Contains(compressed, "truncated") {
		t.Error("expected truncation notice in MCP output")
	}
}

func TestMCPSmallOutput(t *testing.T) {
	raw := `{"status": "ok"}`
	modified, _ := runHook(t, mcpPayload("mcp__myserver__ping", raw))
	if modified {
		t.Error("small MCP output should not be modified")
	}
}

// ── passthrough ───────────────────────────────────────────────────────────────

func TestMCPPassthrough(t *testing.T) {
	t.Setenv("GTK_MCP_PASSTHROUGH_PATTERNS", "query_data*")
	raw := strings.Repeat("x", 5000)

	modified, _ := runHook(t, mcpPayload("mcp__myserver__query_data_large", raw))
	if modified {
		t.Error("tool matching passthrough pattern should not be compressed")
	}
}
