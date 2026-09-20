#!/bin/sh
# prunesh PostToolUse hook for Cursor (matcher MCP:*).

PRUNESH=$(command -v prunesh 2>/dev/null)
if [ -z "$PRUNESH" ]; then
  for candidate in "$HOME/.local/bin/prunesh" "/usr/local/bin/prunesh" "/opt/homebrew/bin/prunesh"; do
    if [ -x "$candidate" ]; then
      PRUNESH="$candidate"
      break
    fi
  done
fi

[ -z "$PRUNESH" ] && exit 0

# Only activate in projects that have run `prunesh init`.
_root=$(git rev-parse --show-toplevel 2>/dev/null) || exit 0
[ -f "$_root/.prunesh" ] || exit 0

exec "$PRUNESH" hook-post --agent=cursor
