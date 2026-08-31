// prunesh — PreToolUse proxy and PostToolUse filter for coding agents.
// Binary: prunesh
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/prunesh/prunesh/internal/pluginregistry"
	"github.com/prunesh/prunesh/internal/hook"
	"github.com/prunesh/prunesh/internal/jsonmerge"
	"github.com/prunesh/prunesh/internal/proxy"
	"github.com/prunesh/prunesh/internal/registry"
	"github.com/prunesh/prunesh/plugins/gain"
	"github.com/prunesh/prunesh/plugins/mcpscan"

	_ "github.com/prunesh/prunesh/plugins/cargo"
	_ "github.com/prunesh/prunesh/plugins/docker"
	_ "github.com/prunesh/prunesh/plugins/find"
	_ "github.com/prunesh/prunesh/plugins/git"
	_ "github.com/prunesh/prunesh/plugins/go"
	_ "github.com/prunesh/prunesh/plugins/grep"
	_ "github.com/prunesh/prunesh/plugins/ls"
	_ "github.com/prunesh/prunesh/plugins/npmtest"
	_ "github.com/prunesh/prunesh/plugins/pytest"
	_ "github.com/prunesh/prunesh/plugins/python"
	_ "github.com/prunesh/prunesh/plugins/readcmd"
	_ "github.com/prunesh/prunesh/plugins/rg"
	_ "github.com/prunesh/prunesh/plugins/tree"
)

const version = "0.12.0"

func usage() {
	fmt.Fprintf(os.Stderr, `prunesh %s

Usage:
  prunesh init                       Activate prunesh in the current project (writes .prunesh marker)
  prunesh hook-pre --agent=<agent>   PreToolUse hook — rewrites shell commands to prunesh
  prunesh hook-post --agent=<agent>  PostToolUse hook — reads stdin, writes filtered output
  prunesh json-merge <file>          Deep-merge JSON from stdin into <file>
  prunesh <module> [args...]         Run a registered command through the proxy
  prunesh mcp-scan                   List tools from all MCP servers, suggest passthrough prefixes
  prunesh gain                       Show token savings analytics
  prunesh plugin install <mod@ver> [--replace]  Install an external plugin (go dependency)
  prunesh plugin uninstall <id>      Remove an installed plugin by full id
  prunesh plugin list                List installed plugins (active marked)
  prunesh version                    Print version

Agents:
  claudecode, cursor, codex, opencode

Environment:
  PRUNESH_MCP_PASSTHROUGH_PATTERNS  Comma-separated MCP tool patterns to skip filtering
                                 Example: hc_*,my_tool
`, version)
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "version", "--version", "-v":
		fmt.Printf("prunesh %s\n", version)

	case "init":
		runInit()

	case "hook-pre":
		agent, err := parseAgentFlag(os.Args[2:])
		if err != nil {
			fmt.Fprintf(os.Stderr, "prunesh hook-pre: %v\n", err)
			os.Exit(1)
		}
		bin, err := os.Executable()
		if err != nil {
			fmt.Fprintf(os.Stderr, "prunesh hook-pre: %v\n", err)
			os.Exit(1)
		}
		_, err = hook.RunPre(os.Stdin, os.Stdout, bin, agent)
		if err != nil {
			fmt.Fprintf(os.Stderr, "prunesh hook-pre: %v\n", err)
			os.Exit(1)
		}

	case "hook-post":
		agent, err := parseAgentFlag(os.Args[2:])
		if err != nil {
			fmt.Fprintf(os.Stderr, "prunesh hook-post: %v\n", err)
			os.Exit(1)
		}
		_, err = hook.Run(os.Stdin, os.Stdout, agent)
		if err != nil {
			fmt.Fprintf(os.Stderr, "prunesh hook-post: %v\n", err)
			os.Exit(1)
		}

	case "json-merge":
		if len(os.Args) != 3 || os.Args[2] == "" {
			fmt.Fprintln(os.Stderr, "usage: prunesh json-merge <file>")
			os.Exit(1)
		}
		changed, err := jsonmerge.MergeFile(os.Args[2])
		if err != nil {
			fmt.Fprintf(os.Stderr, "prunesh json-merge: %v\n", err)
			os.Exit(1)
		}
		if changed {
			fmt.Println("updated")
		} else {
			fmt.Println("unchanged")
		}

	case "mcp-scan":
		if err := mcpscan.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "prunesh mcp-scan: %v\n", err)
			os.Exit(1)
		}

	case "gain":
		t, err := gain.Open()
		if err != nil {
			fmt.Fprintf(os.Stderr, "prunesh: cannot open gain db: %v\n", err)
			os.Exit(1)
		}
		defer t.Close()
		if err := gain.PrintSummary(t); err != nil {
			fmt.Fprintf(os.Stderr, "prunesh: %v\n", err)
			os.Exit(1)
		}

	case "plugin":
		runPlugin(os.Args[2:])

	default:
		if registry.Get(os.Args[1]) == nil && !pluginregistry.HasActive(os.Args[1]) {
			fmt.Fprintf(os.Stderr, "prunesh: unknown command %q\n\n", os.Args[1])
			usage()
			os.Exit(1)
		}
		os.Exit(proxy.Run(os.Args[1], os.Args[2:]))
	}
}

func parseAgentFlag(args []string) (hook.Agent, error) {
	var agent hook.Agent
	seen := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--agent":
			if i+1 >= len(args) {
				return "", fmt.Errorf("--agent requires a value")
			}
			parsed, err := hook.ParseAgent(args[i+1])
			if err != nil {
				return "", err
			}
			agent = parsed
			seen = true
			i++
		case strings.HasPrefix(a, "--agent="):
			parsed, err := hook.ParseAgent(strings.TrimPrefix(a, "--agent="))
			if err != nil {
				return "", err
			}
			agent = parsed
			seen = true
		default:
			return "", fmt.Errorf("unknown argument %q", a)
		}
	}
	if !seen {
		return "", fmt.Errorf("--agent is required")
	}
	return agent, nil
}
