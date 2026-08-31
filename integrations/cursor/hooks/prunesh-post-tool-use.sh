#!/bin/sh
# prunesh PostToolUse hook for Cursor (matcher MCP:*).

GTKAI=$(command -v prunesh 2>/dev/null)
if [ -z "$GTKAI" ]; then
  for candidate in "$HOME/.local/bin/prunesh" "/usr/local/bin/prunesh" "/opt/homebrew/bin/prunesh"; do
    if [ -x "$candidate" ]; then
      GTKAI="$candidate"
      break
    fi
  done
fi

[ -z "$GTKAI" ] && exit 0

# Only activate in projects that have run `prunesh init`.
_root=$(git rev-parse --show-toplevel 2>/dev/null) || exit 0
[ -f "$_root/.prunesh" ] || exit 0

exec "$GTKAI" hook-post --agent=cursor
