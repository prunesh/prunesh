---
name: prunesh
description: Active when .prunesh exists at the repo root — transparent command proxy and output filter for git, grep, rg, find, ls, docker, cargo, go, python, pytest, npm, tree, and MCP Read output. Load to understand filtering behavior and avoid misinterpreting compact output.
---

# prunesh

prunesh intercepts shell commands before they run (PreToolUse rewrite) and compacts their output after (PostToolUse filter). The proxy is transparent: it exits silently when the binary is absent, and passes through any command it has no rule for.

## 1. Verify the proxy is active for this project

Before applying prunesh context:

1. Resolve the Git repository root, or use the current workspace if it is not a Git repository.
2. Read `<root>/.prunesh`.
3. Continue only when the file is present.
4. Confirm the binary is available: `command -v prunesh`.

If `.prunesh` is missing, prunesh is not initialized for this project. Create it to enable filtering:

```bash
echo '{}' > .prunesh
```

If `prunesh` is not in PATH, the binary is not installed. No filtering will occur regardless of the marker.

## 2. Commands intercepted

These commands are automatically rewritten to `prunesh <cmd> <args>` before running:

| Command | Effect |
|---|---|
| `git` | `--no-pager`, stats on diff, log truncation |
| `grep` / `rg` | matches grouped by file, line limits |
| `find` | truncates deep listings, strips empty results |
| `ls` | groups by extension, line limit |
| `docker` | strips build and log noise |
| `cargo` / `go` | compact build and test output |
| `python` / `pytest` | compact failure output |
| `npm test` / `npx` | compact vitest/jest output |
| `tree` | depth and line limits |
| `read` (MCP Read tool) | file size truncation |

MCP tool output is also filtered after execution when a rule matches.

## 3. Interpreting filtered output

Compact or truncated output is intentional. Do not:

- Re-run the command with broader flags to retrieve the full output
- Interpret a shorter-than-expected result as a failure
- Add flags like `--no-filter` that do not exist in the real tool

If the full output is genuinely needed for the task, bypass filtering for that call:

```bash
prunesh proxy <cmd> <args>
```

## 4. Token analytics

```bash
prunesh gain
```

Shows cumulative token savings across sessions, broken down by command.
