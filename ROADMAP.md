# Roadmap: prunesh vs rtk 0.42.4

Compared against [rtk-ai/rtk](https://github.com/rtk-ai/rtk) `0.42.4` (`ba7a9ce`). prunesh is at `0.11.0-beta.2`.

**Product decision (2026-08-17):** gtk follows the same path as rtk. It rewrites the command **before** execution (`PreToolUse` → `git status` becomes `prunesh git status`). prunesh runs the real binary, injects flags, and filters the output. Post-filtering remains only for Claude Code native tools that do not go through Bash (`Read`, MCP, `Grep`, `Glob`).

---

## 1. Primary — PreToolUse rewrite (proxy)

Today gtk filters *after* the fact. `Module.Rewrite` exists and is never called. That is the design gap, not a missing command catalog.

### Target flow

```text
Without gtk:  Claude --git status--> shell --> git --> raw stdout --> Claude

With gtk:     Claude --git status--> PreToolUse --> prunesh hook-pre
                                                       |
                                                       v
                                          command: prunesh git status
                                                       |
                                                       v
                                          prunesh runs git (injected flags)
                                          filters stdout/stderr
                                          records gain
                                                       |
                                                       v
                                          Claude receives compact output
```

Claude never sees the rewrite. The agent calls `git status`; the hook replaces it with `prunesh git status`.

### What this changes (and what it does not)

| Piece | Change |
|---|---|
| Plugin | Registers `PreToolUse` (matcher `Bash`) in addition to `PostToolUse`. The plugin still configures Claude Code. |
| Binary | Becomes a CLI proxy: `prunesh git status`, `prunesh ls`, `prunesh hook-pre`. It runs the real command. |
| `Rewrite()` | Stops being dead code: injects flags (`git status --porcelain -b`, `go test -json`, `git log --pretty=…`). |
| `FilterOutput` | Runs **inside** the proxy, on output prunesh just captured. |
| `PostToolUse` | Stays for `Read`, `mcp__*`, and later `Grep`/`Glob`. It is not the Bash path. |
| Repo phases | Still two pieces: the plugin registers hooks; the binary filters and now also executes. They stay separate. |

Do not copy from rtk: multi-agent `rtk init`, the TOML engine, `discover`/`learn`, telemetry, on-disk `tee`. gtk stays heuristic (truncation, grouping, stripping).

### Scope of this phase

One PR (or a few) that makes the rtk path work end-to-end for **one** command, plus the infrastructure for the rest.

1. **`prunesh hook-pre`**: reads PreToolUse JSON, rewrites `tool_input.command` when the binary is registered, writes `updatedInput`. If there is no module, pass through.
2. **Plugin**: `PreToolUse` + `prunesh-pre-tool-use.sh`. Matcher `Bash`. Short timeout.
3. **CLI proxy**: `prunesh <module> [args…]` runs the command, captures stdout+stderr, filters, prints, propagates the exit code, records `gain`.
4. **Command detection**: the first word is not enough. Cover `/usr/bin/git`, `sudo git`, `git -C dir status`, `VAR=1 git status`. Pipelines: rewrite only the last stage if it is safe (`grep`/`rg`); otherwise pass through.
5. **`never_worse` guard**: if filtering estimates more tokens than the raw output, print the raw output.
6. **Strip ANSI** before parsing.
7. **First end-to-end command: `git status`**. Rewrite to `prunesh git status`. The module injects `--porcelain -b` (unless the user already asked for another format) and groups by state.

Done when:

- `git status` in Claude Code is rewritten to `prunesh git status` (PreToolUse test payload).
- `prunesh git status` in a terminal produces compact output and git's exit code.
- `prunesh gain` records that invocation.
- An unregistered command (`echo hi`) is left untouched.

Until this is green, do not add new modules. The current Bash post-filter is removed once the proxy covers existing modules; until then it may coexist so there is no coverage hole.

---

## 2. Corrections — existing modules that do not deliver

With the proxy, these modules can inject flags. That is what they cannot do today, which is why they fail.

| Module | Current problem | Fix with rewrite |
|---|---|---|
| `ls` | Done in `0.4.0`: `-l` + `LC_ALL=C`, `-a` only if requested, counts + samples, noise dirs skipped. | — |
| `git status` | Expects porcelain. Claude runs the long format; the parser never matches. | Covered in the primary phase. |
| `git diff` | Done in `0.4.0`: strip `index`/`+++`/`---`, cap per hunk. | — |
| `git log` | Done in `0.4.0`: inject `%h %s (%ar) <%an>` and `-10`; honor `--pretty`/`--oneline`/`-n`. | — |
| `git branch` | Done in `0.5.0`: cap local branches; skip remote-tracking `->`. | — |
| `grep` | Done in `0.4.0`: same grouping as `rg`; injects `-nH`. | — |
| `rg` | Grouping shared with `grep`. Pipelines rewrite the last stage. | — |
| `find` | Done in `0.4.0`: group by directory, cap 50, small outputs unchanged. | — |
| `Read` | Done in `0.5.0`: `/* */` block comments; `.css`/`.vue`/`.svelte`. | Native `Grep`/`Glob` still PostToolUse |
| `gain` | Wired in the proxy runner (phase 1). | Per-filter `id` when §4 lands |

Add in the same block, because they are the same Bash path rtk rewrites to `read`:

- `git show`, `git add`/`commit`/`push`/`pull`/`fetch`/`stash` — **done in 0.5.0** (compact on exit 0; `-q` on push/pull/fetch).
- `cat` / `head` / `tail` → proxy modules reusing `read.FilterContent` — **done in 0.5.0**.
- `tree` — **done in 0.5.0** (entry count + capped listing).

Native tools (stay on `PostToolUse`; rtk cannot do this):

- `Grep`, `Glob`.
- MCP: keep truncation + passthrough; compact JSON when the body is text.

Done when: fixtures for long status, unified diff, verbose log, `ls -la`. Measurable savings on all of them, including `ls`. `go test ./...` green.

---

## 3. Remainder — runners and ecosystem

Only after the proxy and the corrections. Here rewrite does inject flags (`go test -json`).

### Runners (high noise, ~90% in rtk)

| Module | Commands | Rewrite | Filter |
|---|---|---|---|
| `gotest` | `go test`, `go build`, `go vet` | `go test -json` unless `-bench` or `-json` is already present | `ok` packages → count; full `FAIL` — **done in 0.6.0** |
| `cargo` | `test`, `build`, `clippy`, `check` | by subcommand | errors/failures; collapse `Compiling` — **done in 0.7.0** |
| `pytest` | `pytest`, `python -m pytest` | — | failures + short traceback — **done in 0.8.0** |
| `npmtest` | `npm test`, `pnpm test`, `npx vitest`/`jest` | — | failures; strip ANSI — **done in 0.9.0** |
| `docker` | `ps`, `images`, `logs`, `compose ps/logs` | — | essential columns; capped logs — **done in 0.9.0** |

Done when: `go test` fixture with 40 ok packages + 1 FAIL; the agent sees the FAIL and a count of the ok packages.

### Ecosystem, only if `gain` asks for it

Do not open modules “just in case”:

- linters: `ruff check`, `tsc`, `eslint`/`biome`, `golangci-lint`
- `gh pr`/`issue`/`run`
- `kubectl get`/`logs`
- `curl` JSON (passthrough if the body is not text)

Out of scope until the core and the runners cover a typical session: rtk TOML filters (`helm`, `pulumi`, `terraform`, `dotnet`, `gradle`, `phpunit`, …), multi-agent, `discover`/`learn`.

---

## 4. Filter plugins — namespaced filters (business logic)

This is **not** the Claude Code plugin system and not any agent hook/marketplace. It is the domain model inside prunesh: who implements filtering for which shell command.

Today the registry collapses **command = filter** (`ls` → one `Module`). The target model separates **identity** from **what gets intercepted**.

### Identity

Every filter has a full name:

```text
author/<cmd>
```

Examples: `prunesh/ls`, `prunesh/git`, `prunesh/date`, `jmeiracorbal/ls`.

- `author` — who owns the implementation.
- `<cmd>` — the shell argv0 this filter is for (`ls`, `git`, `date`, …).

Built-in filters ship as `prunesh/<cmd>` (compiled in). Third-party filters use the same naming rule.

The agent still runs `ls`, `git status`, … PreToolUse still rewrites to `prunesh ls`, `prunesh git status`. The namespace never appears in the Bash command Claude sees.

### Contract

Each filter declares, at minimum:

| Field | Meaning |
|---|---|
| `id` | Full name `author/<cmd>` (must match the naming rule) |
| `command` | Shell command intercepted (argv0 basename), e.g. `ls` |

Behavior matches today’s `Module`: `Rewrite`, `FilterOutput`, optional `ExtraEnv`. The core keeps ANSI strip, `never_worse`, and `gain`; filters do not bypass them.

A filter owns the full surface of that command or passes through: if it cannot handle an invocation, `Rewrite` returns no change and prunesh runs the original argv unchanged.

### Which filter is active

Many filters may target the same shell command. Only one is **active** at a time:

- **Active filter** = the **most recently installed** among all filters whose `command` field equals that argv0.

On **install**, when another filter already targets the same command:

- Abort with an error unless `--replace` is passed (same `id` upgrades always allowed).
- With `--replace`, the newly installed filter becomes active; the previous filter stays installed but inactive.

On **uninstall** (`prunesh plugin uninstall author/<cmd>` — full id only):

- Remove that filter by `id`.
- If **no filters remain** for that shell command → do not rewrite it in PreToolUse; pass through.
- If **other filters remain** → active = most recent among the survivors (same rule as install).

Listing installed filters and which one is active per command is part of this phase.

### CLI (sketch)

```text
prunesh plugin install <path-or-package> [--replace]   # abort on command conflict unless --replace
prunesh plugin uninstall prunesh/ls   # by full id only
prunesh plugin list                        # all filters; mark active per command
```

Exact transport (Go package, manifest + subprocess, …) is an implementation detail. The rules above are not.

### What this is not

- Not a Claude Code plugin per filter.
- Not rtk’s TOML rule engine.
- Not auto-discovery from PATH or GitHub without an explicit `filter install`.

Third-party filters are installed into prunesh’s filter registry; they do not register agent hooks.

Done when: native `prunesh/*` filters use the same registry; install/uninstall/list work; conflict and uninstall semantics above have tests; `hook-pre` resolves the active filter by shell command before rewrite.

**Status (0.11.x beta):** registry, `filter install|uninstall|list`, conflict/`--replace` semantics, active resolution, and subprocess transport are implemented. [prunesh/date](https://github.com/prunesh/date) is the first external-only filter and the reference template (`prunesh/<cmd>`). Remaining built-ins migrate gradually (see below and ARCHITECTURE.md § Built-in migration).

### Built-in migration roadmap

Each row is one external repository. One repo per shell argv0 (or a small group when they share the same implementation). Template: [prunesh/date](https://github.com/prunesh/date) + [HOWTO.md](https://github.com/prunesh/date/blob/main/HOWTO.md).

**Per-filter steps:**

1. Publish `github.com/prunesh/<cmd>` with `prunesh.json` (`command`, `stdin/v1`, semver constraint).
2. Users install it with `prunesh plugin install <module@version>`.
3. External plugin shadows the built-in (active = most recent install).
4. Remove the blank import from `cmd/prunesh/main.go` only when the command should require an external install (as with `date`).

Until step 4, the built-in stays compiled in as fallback.

#### Done

| Repo | `command` | Built-in removed | Notes |
|---|---|---|---|
| [prunesh/date](https://github.com/prunesh/date) | `date` | yes | Reference template; install with `prunesh plugin install github.com/prunesh/date@v0.13.0` |

#### Pending — high priority

Frequent in agent sessions; mature built-in logic; good next targets after `date`.

| Repo | `command` | Built-in module | Notes |
|---|---|---|---|
| `prunesh/ls` | `ls` | `modules/ls` | Very frequent; `-l` + grouping + noise dirs |
| `prunesh/grep` | `grep` | `modules/grep` | High volume; injects `-nH`; grouping shared with `rg` |
| `prunesh/find` | `find` | `modules/find` | High savings (~64% in bench); group by directory |
| `prunesh/git` | `git` | `modules/git` | Largest module: status, log, diff, branch, show, push/pull/fetch/stash |

**Recommended order for this tier:** `ls` → `grep` → `find` → `git` (leave `git` last within the tier — most subcommands and edge cases).

#### Pending — medium priority

Runners and tooling; logic is stable but less universal than core shell commands.

| Repo | `command` | Built-in module | Notes |
|---|---|---|---|
| `prunesh/go` | `go` | `modules/go` | `test -json`, `build`, `vet` |
| `prunesh/cargo` | `cargo` | `modules/cargo` | test/build/clippy/check; collapse `Compiling` |
| `prunesh/pytest` | `pytest` | `modules/pytest` | failures + short traceback |
| `prunesh/npm` | `npm` | `modules/npmtest` | test runners; strip ANSI |
| `prunesh/pnpm` | `pnpm` | `modules/npmtest` | same filter family as `npm` |
| `prunesh/npx` | `npx` | `modules/npmtest` | vitest/jest via npx |
| `prunesh/docker` | `docker` | `modules/docker` | read-only: `ps`, `images`, `logs`, `compose ps/logs` |

`python` / `python3` delegate to pytest when `-m pytest`; migrate with `prunesh/pytest` or a dedicated `prunesh/python` if the argv0 must be intercepted separately.

#### Pending — low priority

| Repo | `command` | Built-in module | Notes |
|---|---|---|---|
| `prunesh/rg` | `rg` | `modules/rg` | Grouping shared with `grep`; pipeline last-stage rewrite |
| `prunesh/tree` | `tree` | `modules/tree` | entry count + capped listing |
| `prunesh/cat` | `cat`, `head`, `tail` | `modules/readcmd` | one repo, three `command` values or three repos; reuses `read.FilterContent` |

#### Not built-in migration

These stay in the core or are separate tracks — do not confuse with `prunesh/*` shell filters:

| Track | Item | Where |
|---|---|---|
| PostToolUse | native `Grep`, `Glob` | `hook-post`; agents do not run them through Bash |
| PostToolUse | `Read`, MCP | already filtered |
| Core | `gain` per `filter_id` | attribution by installed filter id |
| Ecosystem (§3) | `ruff`, `tsc`, `eslint`/`biome`, `golangci-lint`, `gh`, `kubectl`, `curl` JSON | third-party filters **only if `gain` justifies it** |
| Out of scope | rtk TOML (`helm`, `pulumi`, `terraform`, `dotnet`, `gradle`, `phpunit`, …) | not planned until typical session is covered |

#### Summary

- **Migrated:** 1 (`date`)
- **Pending repos:** 12–15 (17 argv0 counting `cat`/`head`/`tail` and `npm`/`pnpm`/`npx` separately)
- **Next filter to add:** `prunesh/ls`

---

## Current vs target

| | prunesh 0.11.x beta | Remaining |
|---|---|---|
| Agents | Claude Code plugin; Cursor / Codex hooks; OpenCode plugin | — |
| Bash | `PreToolUse` rewrites registered commands to `prunesh …`; the binary runs and filters | — |
| Filter identity | Namespaced `author/<cmd>`; active = most recent install; built-in fallback | Migrate remaining built-ins (see §4 migration roadmap) |
| `Rewrite()` | Injects flags for `git status`, `git log`, `ls`, `grep`, `go test -json`, … | External `prunesh/*` repos per command |
| `Read` / MCP | `PostToolUse` | native `Grep`/`Glob` |
| `gain` | Every proxy execution | Per-filter attribution by `id` |
| Commands | Built-ins: find, ls, git, grep, rg, cat/head/tail, tree, go, cargo, pytest, npm/pnpm/npx, docker; external: date | 12–15 external repos (§4 migration roadmap); native Grep/Glob |

Filtering stays heuristic. No semantic compression.

---

## Risks

- **Hook contract.** A `PreToolUse` that writes bad JSON disables the hook (Claude Code silences it). Tests with a real payload; stdout is JSON only.
- **Pipelines and `&&`.** Rewriting `find … \| head` as `find` breaks the meaning. Last safe stage only, or pass through.
- **Write commands.** `git commit`, `git push`, `docker run` must not be left half-done. The proxy forwards stdin/TTY when the command is not read-only; if it cannot, do not rewrite.
- **Double filtering.** While Pre and Post both sit on Bash, output can be filtered twice and grow. Drop Bash Post as soon as the proxy covers the module.
- **`never_worse`.** A 3-file status must not become a longer paragraph.
- **Filter stack.** Multiple filters for one command rely on install order; uninstall must not leave a stale active pointer.
- **False positives in runners.** Keep any line that is not classified.

---

## PR order

1. PreToolUse proxy + end-to-end `git status` (section 1) — **done in 0.4.0**.
2. Corrections to current modules + `cat`/`head`/`tail`/`tree` + remaining git (section 2) — **done in 0.5.0**.
3. Runners (section 3) — **done in 0.9.0** (go, cargo, pytest, npm, docker).
4. Multi-agent hooks (Cursor, Codex, OpenCode) — **done in 0.10.0**.
5. Filter plugin registry + install/uninstall/list + migrate natives to `prunesh/*` (section 4) — **core done in 0.11.x beta**; per-command migration ongoing (see §4 **Built-in migration roadmap**; next: `prunesh/ls`).
6. Ecosystem commands as third-party filters according to `gain` (section 3 ecosystem).

The git tag must match every version-bearing file (`cmd/prunesh/main.go`, plugin json, `mcpscan`, README).

---

## Future: alternative plugin protocols

The current plugin protocol is subprocess/v1 — the core forks the plugin binary, sends a JSON request over stdin, and reads the JSON response from stdout. This is simple and works for stateless, low-frequency commands.

Two alternatives are worth revisiting if requirements change:

**WASM**: the plugin ships as a `.wasm` module instead of a native binary. The core loads it into an embedded runtime (e.g. `wazero`). No fork overhead; the module runs sandboxed without arbitrary syscall access. Makes sense if plugin startup latency becomes measurable or if untrusted third-party plugins are distributed. Requires authors to target `GOARCH=wasm GOOS=wasip1` (or equivalent for other languages) and the core to embed a WASM runtime.

**gRPC**: the plugin runs as a persistent server and exposes an RPC interface. The core connects and calls methods instead of forking per invocation. Makes sense for stateful plugins (e.g. one that maintains an in-memory cache across calls) or for very high invocation frequency. Adds significant operational complexity: the plugin must manage its own lifecycle, and the core must handle connection failures and restarts.

Neither is planned. The subprocess model covers all current use cases. Revisit only if `gain` data shows fork overhead is significant or if a stateful plugin use case appears.
