## gtk-ai — rule-based output filtering

gtk-ai is active as a PostToolUse hook. It intercepts Bash, grep, find, ls, git, and MCP tool output before it enters the context. Depending on the command, it applies truncation, extension grouping, condensed formatting, or comment line removal.

Compression is transparent: no changes to your workflow are needed.
