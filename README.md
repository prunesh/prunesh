# prunesh

![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go&logoColor=white)
![Version](https://img.shields.io/badge/version-0.12.0-blue?style=flat)
![License](https://img.shields.io/badge/license-Apache%202.0-blue?style=flat)
![Claude Code](https://img.shields.io/badge/Claude%20Code-plugin-blueviolet?style=flat)
![Cursor](https://img.shields.io/badge/Cursor-hooks-000000?style=flat)
![Codex](https://img.shields.io/badge/Codex-hooks-412991?style=flat)
![OpenCode](https://img.shields.io/badge/OpenCode-plugin-6D28D9?style=flat)

> *Podar es eliminar lo que sobra para que lo esencial crezca más fuerte.*

`prunesh` reduces token usage from coding agents by intercepting shell commands and filtering their output before it reaches the model.

Two parts:

- the `prunesh` Go binary: rewrites commands before they run and filters stdout
- per-agent integrations: register `PreToolUse` / `PostToolUse` hooks and invoke `prunesh`

```text
Agent → Shell("git status")
              ↓ PreToolUse → prunesh hook-pre --agent=<agent>
         command becomes: prunesh git status
              ↓ prunesh runs: git status --porcelain -b
         compact grouped output
              ↓
         Agent receives filtered output
```

## Benchmark

Numbers from `go test ./internal/hook/... -v`. Token estimate: ~4 chars/token.

| Input | Tokens before | Tokens after | Savings |
|---|---:|---:|---:|
| `find`: 150 paths | 1,050 | 374 | **64%** |
| `ls`: 70 entries | 262 | 65 | **75%** |
| `grep`: 250 matches across 20 files | 3,820 | 3,360 | **12%** |
| `git diff`: 400 lines | 3,185 | 813 | **74%** |
| `git log`: 80 commits | 1,917 | 244 | **87%** |
| `Read`: Go file, 100 commented vars | 1,346 | 348 | **74%** |
| `Read`: plain text, 400 lines | 2,772 | 1,380 | **50%** |
| MCP tool response — 5,200 chars | 1,300 | 758 | **42%** |

Savings grow with output size. Small outputs may not be reduced.

## Installation

Installs the `prunesh` binary and configures hooks for every compatible agent found on the machine. Run once per machine.

### Option A: install script

```bash
curl -sSL https://raw.githubusercontent.com/prunesh/prunesh/main/install.sh | sh
```

Explicit targets:

```bash
curl -sSL https://raw.githubusercontent.com/prunesh/prunesh/main/install.sh | sh -s -- --agent=cursor
curl -sSL https://raw.githubusercontent.com/prunesh/prunesh/main/install.sh | sh -s -- --agent=codex
curl -sSL https://raw.githubusercontent.com/prunesh/prunesh/main/install.sh | sh -s -- --agent=opencode
curl -sSL https://raw.githubusercontent.com/prunesh/prunesh/main/install.sh | sh -s -- --agent=all
```

Claude Code still needs the plugin installed:

```bash
claude plugin install -s user prunesh@prunesh
```

Then restart the agent.

### Option B: build from source

Requires Go 1.22+.

```bash
git clone https://github.com/prunesh/prunesh
cd prunesh
go build -o ~/.local/bin/prunesh ./cmd/prunesh/
```

Then configure agents without reinstalling the binary:

```bash
GTKAI_SKIP_BINARY=1 sh install.sh -- --agent=all
```

### Agent surfaces

| | Claude Code | Cursor | Codex | OpenCode |
|---|---|---|---|---|
| **Hooks** | plugin `PreToolUse` / `PostToolUse` | `~/.cursor/hooks/` | `~/.codex/hooks/` | `~/.config/opencode/plugins/prunesh.ts` |
| **Shell rewrite** | matcher `Bash` | matcher `Shell` | matcher `Bash` and shell aliases | `tool.execute.before` |
| **Read / MCP post** | yes | MCP only | shell rewrite only | `read` and MCP tools |

## Project activation

Once installed, prunesh must be activated per project. Run once at the repo root:

```bash
prunesh init
```

This writes an empty `.prunesh` marker at the git root. Hooks silently skip any project without the marker — prunesh never activates where you did not opt in.

## Built-in modules

Each module handles one command. All built-in modules ship with the binary.

| Module | Command | What it does |
|---|---|---|
| `find` | `find` | Groups paths by directory, caps shown files, extension summary |
| `ls` | `ls` | Injects `-l` + `LC_ALL=C`; compact to counts and samples |
| `git` | `git` | status/log/diff/branch/show; write subcommands compact on exit 0; push/pull/fetch inject `-q` |
| `grep` / `rg` | `grep`, `rg` | Shared grouping by file with per-file and total caps; `grep` injects `-nH` |
| `cat` / `head` / `tail` | same | Reuses `read.FilterContent` on single-file output |
| `tree` | `tree` | Entry count + capped listing |
| `gain` | — | SQLite analytics: recorded on each proxy run |

## Marketplace plugins

External plugins ship as standalone repos and are installed with `prunesh plugin`:

```bash
prunesh plugin install github.com/prunesh/date@v0.13.0
prunesh plugin install github.com/prunesh/date@v0.13.0 --replace
prunesh plugin list
prunesh plugin uninstall prunesh/date
```

| Command | Description |
|---|---|
| `plugin install <module@version>` | Download, validate `prunesh.json` contract, build, register in `~/.prunesh/plugins.db` |
| `plugin install … --replace` | Required when another plugin is already active for the same command |
| `plugin list` | List installed plugins; marks the active one per command |
| `plugin uninstall <id>` | Remove by full id (e.g. `prunesh/date`); deletes `~/.prunesh/plugins/<id>/` |

**Conflict policy:** if plugin `acme/date` is active for `date`, installing `prunesh/date` aborts unless you pass `--replace`. With `--replace`, the new plugin becomes active; the previous one stays installed but inactive. To remove it: `plugin uninstall acme/date`. To switch back: reinstall with `--replace`.

Uninstalling the active plugin promotes the most recently installed survivor for that command, or falls back to the built-in module when one exists.

### stdin/v1 protocol

External plugins use contract `stdin/v1`: prunesh runs the plugin binary and exchanges JSON on stdin/stdout (`rewrite` + `filter_output`). Any language works as long as the binary implements the protocol. See [ARCHITECTURE.md](ARCHITECTURE.md) and the reference plugin [prunesh/date](https://github.com/prunesh/date).

## Adding a built-in module

1. Create `plugins/mycommand/mycommand.go`
2. Implement the `Module` interface
3. Register at `init()` time
4. Import in `cmd/prunesh/main.go`

```go
package mycommand

import "github.com/prunesh/prunesh/internal/registry"

func init() { registry.Register(&Module{}) }

type Module struct{}

func (m *Module) Name() string { return "mycommand" }

func (m *Module) Rewrite(args []string) ([]string, bool) { return nil, false }

func (m *Module) FilterOutput(args []string, output string, exitCode int) string { return output }

func (m *Module) TokensBefore(output string) int { return registry.EstimateTokens(output) }

func (m *Module) TokensAfter(filtered string) int { return registry.EstimateTokens(filtered) }
```

```go
_ "github.com/prunesh/prunesh/plugins/mycommand"
```

## MCP passthrough

By default, prunesh truncates all `mcp__*` tool responses above 3,000 chars. To exempt specific tools:

```sh
export GTK_MCP_PASSTHROUGH_PATTERNS="my_tool_*,other_tool"
```

Pattern syntax: exact name or glob prefix (`prefix_*`).

## Commands

```text
prunesh hook-pre --agent=<agent>   PreToolUse handler — rewrites shell commands to prunesh
prunesh hook-post --agent=<agent>  PostToolUse handler — filters Read and MCP
prunesh json-merge <file>          Deep-merge JSON from stdin into an agent config file
prunesh <module> [args...]         Proxy: run a registered command through prunesh
prunesh mcp-scan                   List MCP server tools, suggest passthrough prefixes
prunesh gain                       Token savings analytics
prunesh plugin install <mod@ver> [--replace]  Install an external plugin
prunesh plugin uninstall <id>      Remove an installed plugin by full id
prunesh plugin list                List installed plugins (active marked)
prunesh version                    Print version
```

## Architecture

```text
prunesh/
├── cmd/prunesh/              # binary entry point
├── internal/
│   ├── registry/           # Module interface + Register() + EstimateTokens()
│   ├── matchgroup/         # shared grep/rg grouping
│   ├── shell/              # shell command rewrite
│   ├── proxy/              # execute + filter + gain
│   ├── text/               # ANSI strip
│   ├── jsonmerge/          # installer config merge
│   ├── hook/               # PreToolUse and PostToolUse handlers
│   ├── pluginregistry/     # SQLite DB for installed plugins
│   ├── pluginsubprocess/   # stdin/v1 protocol adapter
│   ├── plugininstall/      # download, validate, install plugin binaries
│   └── pluginmanifest/     # prunesh.json manifest parsing and validation
├── plugins/                # built-in modules (compiled into the binary)
└── integrations/
    ├── claude/             # Claude Code plugin (hooks + scripts)
    ├── cursor/             # Cursor hook scripts
    ├── codex/              # Codex hook scripts
    └── opencode/           # OpenCode plugin
```

The `registry` package is the only shared dependency between modules. Modules never import each other.

## Known issues

### Codex: PreToolUse hook shows "New hook — review required"

When the `PreToolUse` hook is added to `~/.codex/hooks.json` for the first time, Codex CLI requires an explicit trust review before running it. This is expected — it is a security feature of Codex, not a bug in prunesh.

**Symptom**

```
PreToolUse hooks
1 hook needs review before it can run.

[!] Hook 1 · new

Event     PreToolUse
Matcher   Bash|shell|local_shell|container_exec|exec_command|shell_command
Source    User config - ~/.codex/hooks.json
Command   ~/.codex/hooks/prunesh-pre-tool-use.sh
Trust     New hook - review required
```

**Why it happens**

Codex stores a SHA-256 hash of each approved hook command in `~/.codex/config.toml` under `[hooks.state]`. A hook without a recorded hash is treated as untrusted and blocked until the user approves it.

**Fix**

Start a Codex session normally. When the review prompt appears, approve the hook (press `a` or follow the on-screen prompt). Codex writes the hash to `config.toml` and the hook runs on all subsequent sessions without interruption.

After approval, `config.toml` will contain an entry like:

```toml
[hooks.state."/Users/<you>/.codex/hooks.json:pre_tool_use:0:0"]
trusted_hash = "sha256:<hash>"
```

The `SessionStart` and `Stop` hooks follow the same flow and must be approved once each if they were not already trusted.

## License

Apache 2.0, see [LICENSE](LICENSE). Attribution required on redistribution.
