## prunesh

Shell commands registered with prunesh are rewritten to `prunesh <cmd>` before they run. Output may be compacted by truncation, grouping, or comment stripping. Do not re-run a command only because the output looks condensed: the original command still ran.

Load and follow the `prunesh` skill when it is available for the detailed protocol. These rules remain active when the skill is not loaded.

### Active when

prunesh is enabled for a project when `.prunesh` exists at the repository root. If it is missing, filtering may still occur but the project has not opted in.

### Behavior

- Compact or truncated output is intentional — do not re-run commands to bypass it.
- The proxy passes through any command it has no rule for, unchanged.
- If `prunesh` is not in PATH, the hook exits silently and commands run unmodified.
