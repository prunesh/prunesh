#!/bin/sh
# gtkai PostToolUse hook for Claude Code.
command -v gtkai >/dev/null 2>&1 || exit 0
exec gtkai hook-post
