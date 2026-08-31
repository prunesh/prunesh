package hook

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/prunesh/prunesh/internal/shell"
)

const stdinCap = 1 << 20

type preOutput struct {
	HookSpecificOutput preSpecific `json:"hookSpecificOutput"`
}

type preSpecific struct {
	HookEventName      string         `json:"hookEventName"`
	PermissionDecision string         `json:"permissionDecision,omitempty"`
	UpdatedInput       map[string]any `json:"updatedInput,omitempty"`
}

type cursorPreOutput struct {
	Permission   string         `json:"permission"`
	UpdatedInput map[string]any `json:"updated_input"`
}

type openCodePreOutput struct {
	Command string `json:"command"`
}

// RunPre reads a PreToolUse event, rewrites shell commands to prunesh when a module matches.
// pruneshBin is the binary path inserted into the rewritten command.
// agent selects the stdout JSON contract of the target coding agent.
func RunPre(r io.Reader, w io.Writer, pruneshBin string, agent Agent) (bool, error) {
	if pruneshBin == "" {
		return false, fmt.Errorf("prunesh binary path is empty")
	}
	if agent == "" {
		return false, fmt.Errorf("agent is empty")
	}

	data, err := io.ReadAll(io.LimitReader(r, stdinCap+1))
	if err != nil {
		return false, fmt.Errorf("read stdin: %w", err)
	}
	if len(data) > stdinCap {
		return false, nil
	}
	if len(data) == 0 {
		return false, nil
	}

	var input hookInput
	if err := json.Unmarshal(data, &input); err != nil {
		return false, nil
	}
	if !isShellTool(input.ToolName) {
		return false, nil
	}

	var toolInput map[string]any
	if err := json.Unmarshal(input.ToolInput, &toolInput); err != nil {
		return false, nil
	}
	cmd, ok := toolInput["command"].(string)
	if !ok || cmd == "" {
		return false, nil
	}

	rewritten, changed := shell.Rewrite(cmd, pruneshBin)
	if !changed {
		return false, nil
	}

	toolInput["command"] = rewritten
	return writePre(w, agent, toolInput, rewritten)
}

func writePre(w io.Writer, agent Agent, toolInput map[string]any, rewritten string) (bool, error) {
	var out []byte
	var err error
	switch agent {
	case AgentClaudeCode:
		out, err = json.Marshal(preOutput{
			HookSpecificOutput: preSpecific{
				HookEventName: "PreToolUse",
				UpdatedInput:  toolInput,
			},
		})
	case AgentCodex:
		out, err = json.Marshal(preOutput{
			HookSpecificOutput: preSpecific{
				HookEventName:      "PreToolUse",
				PermissionDecision: "allow",
				UpdatedInput:       toolInput,
			},
		})
	case AgentCursor:
		out, err = json.Marshal(cursorPreOutput{
			Permission:   "allow",
			UpdatedInput: toolInput,
		})
	case AgentOpenCode:
		out, err = json.Marshal(openCodePreOutput{Command: rewritten})
	default:
		return false, fmt.Errorf("unknown agent %q", agent)
	}
	if err != nil {
		return false, fmt.Errorf("marshal: %w", err)
	}
	_, err = fmt.Fprintln(w, string(out))
	return true, err
}
