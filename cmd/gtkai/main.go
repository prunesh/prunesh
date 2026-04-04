// gtk-ai — PostToolUse hook for Claude Code.
// Applies rule-based output filtering to Bash, git, grep, find, ls, Read, and MCP tools.
// Binary: gtkai
package main

import (
	"fmt"
	"os"

	"github.com/jmeiracorbal/gtk-ai/internal/hook"
	"github.com/jmeiracorbal/gtk-ai/modules/gain"
	"github.com/jmeiracorbal/gtk-ai/modules/mcpscan"

	// Register all built-in modules
	_ "github.com/jmeiracorbal/gtk-ai/modules/find"
	_ "github.com/jmeiracorbal/gtk-ai/modules/git"
	_ "github.com/jmeiracorbal/gtk-ai/modules/grep"
	_ "github.com/jmeiracorbal/gtk-ai/modules/ls"
)

const version = "0.2.1"

func usage() {
	fmt.Fprintf(os.Stderr, `gtkai %s

Usage:
  gtkai hook-post            PostToolUse hook — reads stdin, writes filtered output
  gtkai mcp-scan             List tools from all MCP servers, suggest passthrough prefixes
  gtkai gain                 Show token savings analytics
  gtkai version              Print version

Environment:
  GTK_MCP_PASSTHROUGH_PATTERNS  Comma-separated MCP tool patterns to skip filtering
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
		fmt.Printf("gtkai %s\n", version)

	case "hook-post":
		modified, err := hook.Run(os.Stdin, os.Stdout)
		if err != nil {
			fmt.Fprintf(os.Stderr, "gtkai hook-post: %v\n", err)
			os.Exit(1)
		}
		_ = modified
		os.Exit(0)

	case "mcp-scan":
		if err := mcpscan.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "gtkai mcp-scan: %v\n", err)
			os.Exit(1)
		}

	case "gain":
		t, err := gain.Open()
		if err != nil {
			fmt.Fprintf(os.Stderr, "gtkai: cannot open gain db: %v\n", err)
			os.Exit(1)
		}
		defer t.Close()
		if err := gain.PrintSummary(t); err != nil {
			fmt.Fprintf(os.Stderr, "gtkai: %v\n", err)
			os.Exit(1)
		}

	default:
		fmt.Fprintf(os.Stderr, "gtkai: unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}
