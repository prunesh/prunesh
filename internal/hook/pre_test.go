package hook_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/prunesh/prunesh/internal/hook"
)

func TestPreRewritesGitStatus(t *testing.T) {
	payload := `{"tool_name":"Bash","tool_input":{"command":"git status","description":"show status"}}`
	var out bytes.Buffer
	ok, err := hook.RunPre(strings.NewReader(payload), &out, "prunesh", hook.AgentClaudeCode)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected rewrite")
	}
	var result struct {
		HookSpecificOutput struct {
			HookEventName string `json:"hookEventName"`
			UpdatedInput  struct {
				Command     string `json:"command"`
				Description string `json:"description"`
			} `json:"updatedInput"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.HookSpecificOutput.HookEventName != "PreToolUse" {
		t.Fatalf("event %q", result.HookSpecificOutput.HookEventName)
	}
	if result.HookSpecificOutput.UpdatedInput.Command != "prunesh git status" {
		t.Fatalf("command %q", result.HookSpecificOutput.UpdatedInput.Command)
	}
	if result.HookSpecificOutput.UpdatedInput.Description != "show status" {
		t.Fatalf("description dropped: %q", result.HookSpecificOutput.UpdatedInput.Description)
	}
}

func TestPreLeavesEcho(t *testing.T) {
	payload := `{"tool_name":"Bash","tool_input":{"command":"echo hi"}}`
	var out bytes.Buffer
	ok, err := hook.RunPre(strings.NewReader(payload), &out, "prunesh", hook.AgentClaudeCode)
	if err != nil {
		t.Fatal(err)
	}
	if ok || out.Len() != 0 {
		t.Fatalf("echo must pass through, ok=%v out=%q", ok, out.String())
	}
}

func TestPreLeavesGtkai(t *testing.T) {
	payload := `{"tool_name":"Bash","tool_input":{"command":"prunesh git status"}}`
	var out bytes.Buffer
	ok, err := hook.RunPre(strings.NewReader(payload), &out, "prunesh", hook.AgentClaudeCode)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("already proxied command must not rewrite")
	}
}

func TestPreEmptyBin(t *testing.T) {
	_, err := hook.RunPre(strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"git status"}}`), &bytes.Buffer{}, "", hook.AgentClaudeCode)
	if err == nil {
		t.Fatal("empty prunesh path must fail")
	}
}

func TestPreIgnoresRead(t *testing.T) {
	payload := `{"tool_name":"Read","tool_input":{"file_path":"x.go"}}`
	var out bytes.Buffer
	ok, err := hook.RunPre(strings.NewReader(payload), &out, "prunesh", hook.AgentClaudeCode)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("Read must not be rewritten")
	}
}

func TestPreRewritesRegisteredBash(t *testing.T) {
	cases := []string{"ls -la", "find . -name '*.go'", "grep -n foo .", "rg foo"}
	for _, cmd := range cases {
		payload := fmt.Sprintf(`{"tool_name":"Bash","tool_input":{"command":%q}}`, cmd)
		var out bytes.Buffer
		ok, err := hook.RunPre(strings.NewReader(payload), &out, "prunesh", hook.AgentClaudeCode)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Fatalf("%q: expected rewrite", cmd)
		}
		if !strings.Contains(out.String(), "prunesh "+strings.Fields(cmd)[0]) {
			t.Fatalf("%q: rewritten command missing prunesh prefix: %s", cmd, out.String())
		}
	}
}

func TestPostSkipsGtkaiCommand(t *testing.T) {
	modified, _ := runHook(t, bashPayload("prunesh git status", strings.Repeat("M  file.go\n", 40)))
	if modified {
		t.Fatal("post-hook must not filter prunesh proxy output")
	}
}

func TestPreEmptyAgent(t *testing.T) {
	_, err := hook.RunPre(strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"git status"}}`), &bytes.Buffer{}, "prunesh", "")
	if err == nil {
		t.Fatal("empty agent must fail")
	}
}

func TestPreRewritesCursorShell(t *testing.T) {
	payload := `{"tool_name":"Shell","tool_input":{"command":"git status","working_directory":"/proj"}}`
	var out bytes.Buffer
	ok, err := hook.RunPre(strings.NewReader(payload), &out, "prunesh", hook.AgentCursor)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected rewrite")
	}
	var result struct {
		Permission   string `json:"permission"`
		UpdatedInput struct {
			Command          string `json:"command"`
			WorkingDirectory string `json:"working_directory"`
		} `json:"updated_input"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Permission != "allow" {
		t.Fatalf("permission %q", result.Permission)
	}
	if result.UpdatedInput.Command != "prunesh git status" {
		t.Fatalf("command %q", result.UpdatedInput.Command)
	}
	if result.UpdatedInput.WorkingDirectory != "/proj" {
		t.Fatalf("working_directory dropped: %q", result.UpdatedInput.WorkingDirectory)
	}
}

func TestPreRewritesCodex(t *testing.T) {
	payload := `{"tool_name":"local_shell","tool_input":{"command":"git status"}}`
	var out bytes.Buffer
	ok, err := hook.RunPre(strings.NewReader(payload), &out, "prunesh", hook.AgentCodex)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected rewrite")
	}
	var result struct {
		HookSpecificOutput struct {
			HookEventName      string `json:"hookEventName"`
			PermissionDecision string `json:"permissionDecision"`
			UpdatedInput       struct {
				Command string `json:"command"`
			} `json:"updatedInput"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.HookSpecificOutput.PermissionDecision != "allow" {
		t.Fatalf("permissionDecision %q", result.HookSpecificOutput.PermissionDecision)
	}
	if result.HookSpecificOutput.UpdatedInput.Command != "prunesh git status" {
		t.Fatalf("command %q", result.HookSpecificOutput.UpdatedInput.Command)
	}
}

func TestPreRewritesCodexHookEventName(t *testing.T) {
	payload := `{"tool_name":"local_shell","tool_input":{"command":"git status"}}`
	var out bytes.Buffer
	ok, err := hook.RunPre(strings.NewReader(payload), &out, "prunesh", hook.AgentCodex)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected rewrite")
	}
	var result struct {
		HookSpecificOutput struct {
			HookEventName string `json:"hookEventName"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.HookSpecificOutput.HookEventName != "PreToolUse" {
		t.Fatalf("hookEventName %q", result.HookSpecificOutput.HookEventName)
	}
}

func TestPreRewritesCodexBash(t *testing.T) {
	payload := `{"tool_name":"Bash","tool_input":{"command":"git status"}}`
	var out bytes.Buffer
	ok, err := hook.RunPre(strings.NewReader(payload), &out, "prunesh", hook.AgentCodex)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected rewrite for Bash tool")
	}
	var result struct {
		HookSpecificOutput struct {
			PermissionDecision string `json:"permissionDecision"`
			UpdatedInput       struct {
				Command string `json:"command"`
			} `json:"updatedInput"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.HookSpecificOutput.PermissionDecision != "allow" {
		t.Fatalf("permissionDecision %q", result.HookSpecificOutput.PermissionDecision)
	}
	if result.HookSpecificOutput.UpdatedInput.Command != "prunesh git status" {
		t.Fatalf("command %q", result.HookSpecificOutput.UpdatedInput.Command)
	}
}

func TestPreLeavesEchoForCodex(t *testing.T) {
	payload := `{"tool_name":"local_shell","tool_input":{"command":"echo hi"}}`
	var out bytes.Buffer
	ok, err := hook.RunPre(strings.NewReader(payload), &out, "prunesh", hook.AgentCodex)
	if err != nil {
		t.Fatal(err)
	}
	if ok || out.Len() != 0 {
		t.Fatalf("echo must pass through for codex, ok=%v out=%q", ok, out.String())
	}
}

func TestPreLeavesGtkaiForCodex(t *testing.T) {
	payload := `{"tool_name":"local_shell","tool_input":{"command":"prunesh git status"}}`
	var out bytes.Buffer
	ok, err := hook.RunPre(strings.NewReader(payload), &out, "prunesh", hook.AgentCodex)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("already proxied command must not rewrite for codex")
	}
}

func TestPreRewritesOpenCode(t *testing.T) {
	payload := `{"tool_name":"bash","tool_input":{"command":"ls -la"}}`
	var out bytes.Buffer
	ok, err := hook.RunPre(strings.NewReader(payload), &out, "prunesh", hook.AgentOpenCode)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected rewrite")
	}
	var result struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(result.Command, "prunesh ls") {
		t.Fatalf("command %q", result.Command)
	}
}
