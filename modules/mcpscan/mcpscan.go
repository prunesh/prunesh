// Package mcpscan queries registered MCP servers and lists their tools,
// helping users decide which prefixes to add to GTK_MCP_PASSTHROUGH_PATTERNS.
package mcpscan

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ── settings.json structures ──────────────────────────────────────────────────

type claudeSettings struct {
	MCPServers map[string]mcpServerCfg `json:"mcpServers"`
}

type mcpServerCfg struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
	Type    string            `json:"type"` // "stdio" (default) | "sse" | "http"
}

// ── JSON-RPC 2.0 structures ───────────────────────────────────────────────────

type rpcRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      *int        `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int            `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type toolsListResult struct {
	Tools []struct {
		Name string `json:"name"`
	} `json:"tools"`
}

// ── Public API ────────────────────────────────────────────────────────────────

// ServerResult holds the scan result for one MCP server.
type ServerResult struct {
	Name  string
	Tools []string
	Err   error
}

// Run queries all stdio MCP servers registered in ~/.claude/settings.json
// and prints tool names grouped by prefix, compared against
// GTK_MCP_PASSTHROUGH_PATTERNS.
func Run() error {
	settingsPath := filepath.Join(os.Getenv("HOME"), ".claude", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return fmt.Errorf("cannot read %s: %w", settingsPath, err)
	}

	var cfg claudeSettings
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("cannot parse settings.json: %w", err)
	}

	if len(cfg.MCPServers) == 0 {
		fmt.Println("No MCP servers found in ~/.claude/settings.json")
		return nil
	}

	fmt.Printf("MCP servers found: %d\n\n", len(cfg.MCPServers))

	// Stable ordering
	names := sortedKeys(cfg.MCPServers)

	// Query each server
	results := make([]ServerResult, 0, len(names))
	for _, name := range names {
		srv := cfg.MCPServers[name]
		tools, err := queryServer(srv)
		results = append(results, ServerResult{Name: name, Tools: tools, Err: err})
	}

	// Print per-server tools
	allTools := map[string]bool{}
	for _, r := range results {
		if r.Err != nil {
			fmt.Printf("  %-22s  [error: %v]\n", r.Name, r.Err)
			continue
		}
		if len(r.Tools) == 0 {
			fmt.Printf("  %-22s  (no tools)\n", r.Name)
			continue
		}
		fmt.Printf("  %-22s  →  %s\n", r.Name, strings.Join(r.Tools, ", "))
		for _, t := range r.Tools {
			allTools[t] = true
		}
	}

	// Group tools by prefix (prefix = everything up to and including the first _)
	prefixes := groupByPrefix(allTools)
	if len(prefixes) == 0 {
		return nil
	}

	// Compare against configured patterns
	configured := passthroughSet()
	prefixList := sortedKeys(prefixes)

	fmt.Printf("\nPrefix suggestions:\n")
	missing := []string{}
	for _, p := range prefixList {
		if configured[p] {
			fmt.Printf("  %-14s  ✓ in GTK_MCP_PASSTHROUGH_PATTERNS\n", p)
		} else {
			fmt.Printf("  %-14s  ✗ not configured\n", p)
			missing = append(missing, p)
		}
	}

	if len(missing) > 0 {
		existing := sortedKeys(configured)
		all := append(existing, missing...)
		fmt.Printf("\n  Add to your shell config:\n")
		fmt.Printf("  export GTK_MCP_PASSTHROUGH_PATTERNS=%q\n", strings.Join(all, ","))
	}

	return nil
}

// ── Prefix grouping ───────────────────────────────────────────────────────────

// groupByPrefix groups tool names by their underscore prefix.
// "hc_file_context" → prefix "hc_", "mem_save" → prefix "mem_".
// Tools without an underscore are ignored (they don't benefit from prefix passthrough).
func groupByPrefix(tools map[string]bool) map[string]bool {
	out := map[string]bool{}
	for tool := range tools {
		if idx := strings.Index(tool, "_"); idx > 0 && idx < len(tool)-1 {
			out[tool[:idx+1]] = true
		}
	}
	return out
}

// passthroughSet returns the current GTK_MCP_PASSTHROUGH_PATTERNS as a set.
func passthroughSet() map[string]bool {
	out := map[string]bool{}
	raw := os.Getenv("GTK_MCP_PASSTHROUGH_PATTERNS")
	for _, p := range strings.Split(raw, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out[p] = true
		}
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// ── MCP stdio query ───────────────────────────────────────────────────────────

func queryServer(srv mcpServerCfg) ([]string, error) {
	if srv.Type != "" && srv.Type != "stdio" {
		return nil, fmt.Errorf("unsupported transport %q (only stdio is supported)", srv.Type)
	}
	if srv.Command == "" {
		return nil, fmt.Errorf("no command configured")
	}

	cmd := exec.Command(srv.Command, srv.Args...)

	// Merge env
	cmd.Env = os.Environ()
	for k, v := range srv.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start: %w", err)
	}
	defer func() {
		stdin.Close()
		cmd.Process.Kill() //nolint:errcheck
		cmd.Wait()         //nolint:errcheck
	}()

	type result struct {
		tools []string
		err   error
	}
	ch := make(chan result, 1)
	go func() {
		tools, err := mcpHandshake(stdin, stdout)
		ch <- result{tools, err}
	}()

	select {
	case r := <-ch:
		return r.tools, r.err
	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("timeout (5s)")
	}
}

// mcpHandshake performs the minimal MCP handshake to retrieve tool names.
// Protocol: initialize → (wait for response) → notifications/initialized → tools/list
func mcpHandshake(w io.Writer, r io.Reader) ([]string, error) {
	scanner := bufio.NewScanner(r)

	id1 := 1
	if err := writeJSON(w, rpcRequest{
		JSONRPC: "2.0",
		ID:      &id1,
		Method:  "initialize",
		Params: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo":      map[string]interface{}{"name": "gtkai", "version": "0.3.3"},
		},
	}); err != nil {
		return nil, err
	}

	// Read initialize response
	if !scanner.Scan() {
		return nil, fmt.Errorf("no response to initialize")
	}
	var initResp rpcResponse
	if err := json.Unmarshal(scanner.Bytes(), &initResp); err != nil {
		return nil, fmt.Errorf("parse initialize response: %w", err)
	}
	if initResp.Error != nil {
		return nil, fmt.Errorf("initialize: %s", initResp.Error.Message)
	}

	// Send initialized notification (no ID — it's a notification)
	if err := writeJSON(w, rpcRequest{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
		Params:  map[string]interface{}{},
	}); err != nil {
		return nil, err
	}

	// Request tools/list
	id2 := 2
	if err := writeJSON(w, rpcRequest{
		JSONRPC: "2.0",
		ID:      &id2,
		Method:  "tools/list",
		Params:  map[string]interface{}{},
	}); err != nil {
		return nil, err
	}

	// Read lines until we get the tools/list response (skip intervening notifications)
	for scanner.Scan() {
		var resp rpcResponse
		if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
			continue
		}
		if resp.ID == nil || *resp.ID != id2 {
			continue // notification or unrelated response
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("tools/list: %s", resp.Error.Message)
		}
		var result toolsListResult
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			return nil, fmt.Errorf("parse tools/list: %w", err)
		}
		tools := make([]string, len(result.Tools))
		for i, t := range result.Tools {
			tools[i] = t.Name
		}
		sort.Strings(tools)
		return tools, nil
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("connection closed before tools/list response")
}

func writeJSON(w io.Writer, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "%s\n", data)
	return err
}
