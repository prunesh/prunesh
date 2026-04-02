# gtk-ai

![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go&logoColor=white)
![Version](https://img.shields.io/badge/version-0.2.1-blue?style=flat)
![License](https://img.shields.io/badge/license-Apache%202.0-blue?style=flat)
![Claude Code](https://img.shields.io/badge/Claude%20Code-plugin%20compatible-blueviolet?style=flat)
![Build](https://img.shields.io/badge/build-passing-brightgreen?style=flat)

`gtk-ai` is a two-part integration for Claude Code:

- the `gtkai` Go binary, which applies rule-based filtering to tool output
- the Claude plugin, which registers the `PostToolUse` hook and invokes `gtkai`

It filters Bash, git, grep, find, ls, Read, and MCP tool output before it reaches the agent: truncation, grouping by extension, condensed formatting, and comment stripping depending on the command.

```text
Claude → Bash("find . -name '*.rs'")
              ↓ executes
         84 results, full paths (raw output)
              ↓ PostToolUse hook → plugin script → gtkai hook-post
         84 results / ext: .rs(84) / ...10 paths shown
              ↓
         Claude receives filtered output (shorter, some detail dropped)
```

## Benchmark

Numbers from `go test ./internal/hook/... -v`. Token estimate: ~4 chars/token.

| Input | Tokens before | Tokens after | Savings |
|---|---:|---:|---:|
| `find`: 150 paths | 1,050 | 714 | **32%** |
| `grep`: 250 matches across 20 files | 3,820 | 3,059 | **20%** |
| `git diff`: 400 lines | 3,185 | 2,386 | **25%** |
| `git log`: 80 commits | 1,917 | 1,204 | **37%** |
| `Read`: Go file, 100 commented vars | 1,346 | 348 | **74%** |
| `Read`: plain text, 400 lines | 2,772 | 1,380 | **50%** |
| MCP tool response — 5,200 chars | 1,300 | 758 | **42%** |

Savings grow with output size. Small outputs may not be reduced.

## Installation

gtk-ai requires both parts:

1. the `gtkai` Go binary
2. the Claude plugin that registers the hook and calls the binary

### Option A: manual binary + setup

Download the binary for your platform from [GitHub Releases](https://github.com/jmeiracorbal/gtk-ai/releases), place it in `~/.local/bin/`, then run:

```bash
gtkai version
# gtkai 0.2.1
```

Then run:

```bash
gtkai setup
```

`gtkai setup` configures the Claude side of the integration:

- registers the marketplace
- installs the Claude plugin
- injects the context doc into the global `~/.claude/CLAUDE.md`

The integration is not complete with only the binary. It requires both the binary and the plugin.

### Option B: build from source

Requires Go 1.22+.

```bash
git clone https://github.com/jmeiracorbal/gtk-ai
cd gtk-ai
go build -o ~/.local/bin/gtkai ./cmd/gtkai/
```

Verify the binary works:

```bash
gtkai version
# gtkai 0.2.1
```

Then install the Claude side:

```bash
gtkai setup
```

Setup installs the plugin and injects the context doc into your global `~/.claude/CLAUDE.md`. Restart Claude Code when done.

## Modules

Each module handles one command. All built-in modules ship with the binary.

| Module | Command | What it does |
|---|---|---|
| `find` | `find` | Truncates large result sets, groups by extension, shows summary |
| `ls` | `ls` | Groups files by extension, separates dirs |
| `git` | `git diff/log/status/branch` | Limits diff lines, truncates log, formats status |
| `grep` | `grep` | Caps results, shows match count per file |
| `gain` | — | SQLite analytics: tracks tokens in/out per command (not yet integrated in the hook) |

## Adding a module

1. Create `modules/mycommand/mycommand.go`
2. Implement the `Module` interface
3. Register at `init()` time
4. Import in `cmd/gtkai/main.go`

```go
package mycommand

import "github.com/jmeiracorbal/gtk-ai/internal/registry"

func init() { registry.Register(&Module{}) }

type Module struct{}

func (m *Module) Name() string { return "mycommand" }

func (m *Module) Rewrite(args []string) ([]string, bool) { return nil, false }

func (m *Module) FilterOutput(output string) string { return output }

func (m *Module) TokensBefore(output string) int { return registry.EstimateTokens(output) }

func (m *Module) TokensAfter(filtered string) int { return registry.EstimateTokens(filtered) }
```

```go
_ "github.com/jmeiracorbal/gtk-ai/modules/mycommand"
```

No other changes needed.

## MCP passthrough

By default, gtkai truncates all `mcp__*` tool responses above 3,000 chars. To exempt specific tools, set `GTK_MCP_PASSTHROUGH_PATTERNS` in your shell config:

```sh
export GTK_MCP_PASSTHROUGH_PATTERNS="my_tool_*,other_tool"
```

Pattern syntax: exact name or glob prefix (`prefix_*`).

To identify which tools to exempt, check the tool names returned by your MCP servers. Any tool whose output should reach the agent unmodified, such as structured symbol data or memory results, should be listed here.

## Commands

```text
gtkai hook-post     PostToolUse handler — reads stdin JSON, writes filtered output
gtkai gain          Token savings analytics
gtkai version       Print version
gtkai setup         Install and configure the Claude plugin integration
```

## Architecture

```text
gtk-ai/
├── cmd/gtkai/              # binary entry point
├── internal/
│   ├── registry/           # Module interface + Register() + EstimateTokens()
│   └── hook/               # unified PostToolUse handler
├── modules/
│   ├── find/               # find output filter
│   ├── ls/                 # ls output filter
│   ├── git/                # git subcommand filters
│   ├── grep/               # grep output filter
│   └── gain/               # SQLite token savings analytics
└── plugin/
    ├── hooks/              # Claude plugin hook registration
    └── scripts/            # script that invokes gtkai from Claude's hook system
```

The `registry` package is the only shared dependency between modules. Modules never import each other.

## Works well with

**[hybrid-coco](https://github.com/jmeiracorbal/hybrid-coco)**: local code intelligence for Claude Code. While gtk-ai filters tool output, hybrid-coco replaces file reads and grep with index queries for code navigation. Both operate independently through separate hooks and complement each other.

## License

Apache 2.0, see [LICENSE](LICENSE). Attribution required on redistribution.
