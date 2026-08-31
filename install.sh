#!/bin/sh
# prunesh installer
# Usage: curl -sSL https://raw.githubusercontent.com/prunesh/prunesh/main/install.sh | sh
#
# Agent selection (default: auto-detect installed compatible agents):
#   sh -s -- --agent=auto
#   sh -s -- --agent=claudecode
#   sh -s -- --agent=cursor
#   sh -s -- --agent=codex
#   sh -s -- --agent=opencode
#   sh -s -- --agent=all
#
# To skip binary install (configure agents only):
#   PRUNESH_CLAUDE_ONLY=1 sh install.sh
#   PRUNESH_SKIP_BINARY=1 sh install.sh -- --agent=cursor
#
# Environment:
#   PRUNESH_AGENT=cursor
#   PRUNESH_SCRIPTS_DIR=/path/to/scripts
#   PRUNESH_INSTALL_DIR=$HOME/.local/bin
#   PRUNESH_DRY_RUN=true

set -e

REPO="prunesh/prunesh"
BINARY="prunesh"
INSTALL_DIR="${PRUNESH_INSTALL_DIR:-$HOME/.local/bin}"
SKIP_BINARY="${PRUNESH_SKIP_BINARY:-${PRUNESH_CLAUDE_ONLY:-}}"
DRY_RUN="${PRUNESH_DRY_RUN:-false}"
AGENT="${PRUNESH_AGENT:-auto}"
if [ -n "${PRUNESH_CLAUDE_ONLY:-}" ] && [ -z "${PRUNESH_AGENT:-}" ]; then
  AGENT=claudecode
fi
TMP_DIR=$(mktemp -d)
TMP_ROOT=""
PRUNESH_BIN=""

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
BOLD='\033[1m'
RESET='\033[0m'

info()    { printf "${BLUE}  →${RESET} %s\n" "$1"; }
success() { printf "${GREEN}  ✓${RESET} %s\n" "$1"; }
warn()    { printf "${YELLOW}  ⚠${RESET} %s\n" "$1"; }
error()   { printf "${RED}  ✗${RESET} %s\n" "$1" >&2; exit 1; }
header()  { printf "\n${BOLD}%s${RESET}\n" "$1"; }

printf "${BOLD}"
cat <<'EOF'
   prunesh — rule-based output filtering for coding agents
EOF
printf "${RESET}\n"

header "Detecting system"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
  arm64)   ARCH="arm64" ;;
  *)       error "Unsupported architecture: $ARCH" ;;
esac

case "$OS" in
  linux|darwin) ;;
  *) error "Unsupported OS: $OS" ;;
esac

info "OS:   $OS"
info "Arch: $ARCH"

header "Checking dependencies"

HAS_GO=false
HAS_CURL=false
HAS_WGET=false

command -v go    >/dev/null 2>&1 && HAS_GO=true   && success "Go found: $(go version | awk '{print $3}')"
command -v curl  >/dev/null 2>&1 && HAS_CURL=true && success "curl found"
command -v wget  >/dev/null 2>&1 && HAS_WGET=true
command -v git   >/dev/null 2>&1 || error "git is required. Install it and retry."

fetch() {
  url="$1"
  dest="$2"
  if $HAS_CURL; then
    curl -sSL "$url" -o "$dest"
  elif $HAS_WGET; then
    wget -q "$url" -O "$dest"
  else
    error "Neither curl nor wget found. Install one and retry."
  fi
}

fetch_stdout() {
  url="$1"
  if $HAS_CURL; then
    curl -sSL "$url"
  elif $HAS_WGET; then
    wget -q "$url" -O -
  else
    error "Neither curl nor wget found. Install one and retry."
  fi
}

probe_url() {
  url="$1"
  if $HAS_CURL; then
    curl -sSfI "$url" >/dev/null 2>&1
  elif $HAS_WGET; then
    wget -q --spider "$url" >/dev/null 2>&1
  else
    return 1
  fi
}

json_merge() {
  patch="$1"
  file="$2"
  printf '%s' "$patch" | "$PRUNESH_BIN" json-merge "$file"
}

append_marked_block() {
  dest="$1"
  src="$2"
  start="<!-- prunesh:start -->"
  end="<!-- prunesh:end -->"
  mkdir -p "$(dirname "$dest")"
  if [ -f "$dest" ] && grep -qF "$start" "$dest"; then
    info "$dest — prunesh block already present"
    return
  fi
  {
    [ -f "$dest" ] && cat "$dest"
    printf '\n%s\n' "$start"
    cat "$src"
    printf '%s\n' "$end"
  } > "$dest.tmp"
  mv "$dest.tmp" "$dest"
  success "$dest updated"
}

resolve_binary() {
  if [ -x "$INSTALL_DIR/$BINARY" ]; then
    PRUNESH_BIN="$INSTALL_DIR/$BINARY"
    return
  fi
  if command -v "$BINARY" >/dev/null 2>&1; then
    PRUNESH_BIN=$(command -v "$BINARY")
    return
  fi
  error "$BINARY not found. Install it first or unset PRUNESH_SKIP_BINARY."
}

if [ -n "$SKIP_BINARY" ]; then
  header "Skipping binary install (PRUNESH_SKIP_BINARY/PRUNESH_CLAUDE_ONLY)"
  resolve_binary
  INSTALLED_VERSION=$("$PRUNESH_BIN" version | awk '{print $2}')
  success "$BINARY $INSTALLED_VERSION found ($PRUNESH_BIN)"
else
  header "Installing $BINARY"

  mkdir -p "$INSTALL_DIR"

  ASSET_NAME="${BINARY}-${OS}-${ARCH}"
  RELEASE_URL="https://github.com/$REPO/releases/latest/download/$ASSET_NAME"
  CHECKSUM_URL="https://github.com/$REPO/releases/latest/download/${ASSET_NAME}.sha256"

  if $HAS_CURL || $HAS_WGET; then
    info "Trying pre-built binary..."

    HTTP_CODE=0
    if $HAS_CURL; then
      HTTP_CODE=$(curl -sSL -o "$TMP_DIR/$BINARY" -w "%{http_code}" "$RELEASE_URL" 2>/dev/null || echo 0)
    elif $HAS_WGET; then
      wget -q "$RELEASE_URL" -O "$TMP_DIR/$BINARY" 2>/dev/null && HTTP_CODE=200 || HTTP_CODE=0
    fi

    if [ "$HTTP_CODE" = "200" ]; then
      info "Verifying checksum..."
      EXPECTED=$(fetch_stdout "$CHECKSUM_URL" | awk '{print $1}')
      if [ -z "$EXPECTED" ]; then
        if [ "${PRUNESH_SKIP_CHECKSUM:-}" = "1" ]; then
          warn "Could not fetch checksum — proceeding because PRUNESH_SKIP_CHECKSUM=1"
        else
          error "Could not fetch checksum. Set PRUNESH_SKIP_CHECKSUM=1 to bypass."
        fi
      else
        if command -v shasum >/dev/null 2>&1; then
          ACTUAL=$(shasum -a 256 "$TMP_DIR/$BINARY" | awk '{print $1}')
        elif command -v sha256sum >/dev/null 2>&1; then
          ACTUAL=$(sha256sum "$TMP_DIR/$BINARY" | awk '{print $1}')
        else
          error "No SHA256 tool found (shasum/sha256sum). Set PRUNESH_SKIP_CHECKSUM=1 to bypass."
        fi

        if [ "$ACTUAL" != "$EXPECTED" ]; then
          error "Checksum mismatch. Expected: $EXPECTED  Got: $ACTUAL"
        fi
        success "Checksum verified"
      fi

      mv "$TMP_DIR/$BINARY" "$INSTALL_DIR/$BINARY"
      chmod +x "$INSTALL_DIR/$BINARY"
      success "Binary downloaded from GitHub releases"
    else
      info "No pre-built binary found — building from source"

      if ! $HAS_GO; then
        error "Go is required to build from source. Install it from https://go.dev/dl/ and retry."
      fi

      info "Cloning repository..."
      git clone --depth 1 "https://github.com/$REPO.git" "$TMP_DIR/prunesh" >/dev/null 2>&1

      info "Building $BINARY..."
      cd "$TMP_DIR/prunesh"
      go build -o "$INSTALL_DIR/$BINARY" ./cmd/prunesh/
      cd - >/dev/null
      success "Built from source"
      if [ -d "$TMP_DIR/prunesh/integrations/cursor/hooks" ]; then
        TMP_ROOT="$TMP_DIR/prunesh"
      fi
    fi
  fi

  PRUNESH_BIN="$INSTALL_DIR/$BINARY"
  if ! "$PRUNESH_BIN" version >/dev/null 2>&1; then
    error "Binary installed but failed to run. Check $PRUNESH_BIN"
  fi

  INSTALLED_VERSION=$("$PRUNESH_BIN" version | awk '{print $2}')
  success "$BINARY $INSTALLED_VERSION installed to $PRUNESH_BIN"

  header "Configuring PATH"

  add_to_path() {
    shell_rc="$1"
    if [ -f "$shell_rc" ]; then
      if ! grep -q "$INSTALL_DIR" "$shell_rc" 2>/dev/null; then
        printf '\n# prunesh\nexport PATH="%s:$PATH"\n' "$INSTALL_DIR" >> "$shell_rc"
        success "Added $INSTALL_DIR to PATH in $shell_rc"
      else
        info "$INSTALL_DIR already in $shell_rc"
      fi
    fi
  }

  case "$SHELL" in
    */zsh)  add_to_path "$HOME/.zshrc"  ;;
    */bash) add_to_path "$HOME/.bashrc" ;;
    *)      add_to_path "$HOME/.profile" ;;
  esac

  export PATH="$INSTALL_DIR:$PATH"
fi

resolve_scripts() {
  if [ -n "$TMP_ROOT" ]; then
    return
  fi
  if [ -n "${PRUNESH_SCRIPTS_DIR:-}" ]; then
    TMP_ROOT="$PRUNESH_SCRIPTS_DIR"
    return
  fi
  if [ -f "$0" ] && [ -d "$(dirname "$0")/integrations/cursor/hooks" ]; then
    TMP_ROOT="$(cd "$(dirname "$0")" && pwd)"
    return
  fi

  header "Fetching agent scripts"
  archive_url="https://github.com/$REPO/releases/latest/download/prunesh-scripts.tar.gz"
  checksum_url="https://github.com/$REPO/releases/latest/download/prunesh-scripts.tar.gz.sha256"
  if probe_url "$archive_url"; then
    tmp_archive=$(mktemp)
    fetch "$archive_url" "$tmp_archive" || error "Scripts archive download failed"
    expected=$(fetch_stdout "$checksum_url" | awk '{print $1}')
    if [ -z "$expected" ]; then
      error "Could not fetch scripts checksum"
    fi
    if command -v shasum >/dev/null 2>&1; then
      actual=$(shasum -a 256 "$tmp_archive" | awk '{print $1}')
    else
      actual=$(sha256sum "$tmp_archive" | awk '{print $1}')
    fi
    if [ "$actual" != "$expected" ]; then
      error "Scripts checksum mismatch. Expected: $expected  Got: $actual"
    fi
    TMP_ROOT=$(mktemp -d)
    tar -xzf "$tmp_archive" -C "$TMP_ROOT"
    rm -f "$tmp_archive"
    success "Scripts ready"
    return
  fi

  info "No scripts archive in the latest release — cloning repository"
  git clone --depth 1 "https://github.com/$REPO.git" "$TMP_DIR/prunesh-scripts" >/dev/null 2>&1
  TMP_ROOT="$TMP_DIR/prunesh-scripts"
}

install_skill_global() {
  AGENTS_SKILL_DIR="$HOME/.agents/skills/prunesh"
  mkdir -p "$AGENTS_SKILL_DIR"
  cp "$TMP_ROOT/skills/prunesh/SKILL.md" "$AGENTS_SKILL_DIR/SKILL.md"
  success "$HOME/.agents/skills/prunesh/SKILL.md written"
}

install_skill_symlink() {
  link_dir="$1"
  target="$HOME/.agents/skills/prunesh"
  link="$link_dir/prunesh"
  mkdir -p "$link_dir"
  if [ -L "$link" ] || [ -e "$link" ]; then
    rm -f "$link"
  fi
  ln -s "$target" "$link"
  success "${link} -> ${target}"
}

setup_claudecode() {
  header "Configuring Claude Code"

  install_skill_global
  install_skill_symlink "$HOME/.claude/skills"

  printf "To activate the Claude plugin, run:\n\n"
  printf "  ${BOLD}%s${RESET}\n\n" "claude plugin install -s user prunesh@prunesh"
  printf "Then restart Claude Code.\n"
}

setup_cursor() {
  header "Configuring Cursor"

  hooks_dir="$HOME/.cursor/hooks"
  hooks_json="$HOME/.cursor/hooks.json"
  rules_dir="$HOME/.cursor/rules"

  mkdir -p "$hooks_dir" "$rules_dir"
  cp "$TMP_ROOT/integrations/cursor/hooks/prunesh-pre-tool-use.sh" "$hooks_dir/"
  cp "$TMP_ROOT/integrations/cursor/hooks/prunesh-post-tool-use.sh" "$hooks_dir/"
  chmod +x "$hooks_dir/prunesh-pre-tool-use.sh" "$hooks_dir/prunesh-post-tool-use.sh"
  success "Hook scripts installed to ${hooks_dir}"

  patch=$(printf '{"version":1,"hooks":{"preToolUse":[{"command":"%s/prunesh-pre-tool-use.sh","matcher":"Shell"}],"postToolUse":[{"command":"%s/prunesh-post-tool-use.sh","matcher":"MCP:.*"}]}}' "$hooks_dir" "$hooks_dir")
  result=$(json_merge "$patch" "$hooks_json")
  success "$HOME/.cursor/hooks.json — $result"

  cp "$TMP_ROOT/integrations/cursor/rules/prunesh.mdc" "$rules_dir/prunesh.mdc"
  success "$HOME/.cursor/rules/prunesh.mdc written"

  install_skill_global
  install_skill_symlink "$HOME/.cursor/skills"
}

setup_codex() {
  header "Configuring Codex"

  hooks_dir="$HOME/.codex/hooks"
  hooks_json="$HOME/.codex/hooks.json"
  codex_config="$HOME/.codex/config.toml"
  agents_md="$HOME/.codex/AGENTS.md"

  mkdir -p "$hooks_dir"
  cp "$TMP_ROOT/integrations/codex/hooks/prunesh-pre-tool-use.sh" "$hooks_dir/"
  chmod +x "$hooks_dir/prunesh-pre-tool-use.sh"
  success "Hook scripts installed to ${hooks_dir}"

  patch=$(printf '{"hooks":{"PreToolUse":[{"matcher":"Bash|shell|local_shell|container_exec|exec_command|shell_command","hooks":[{"type":"command","command":"%s/prunesh-pre-tool-use.sh","statusMessage":"prunesh rewrite","timeout":10}]}]}}' "$hooks_dir")
  result=$(json_merge "$patch" "$hooks_json")
  success "$HOME/.codex/hooks.json — $result"

  mkdir -p "$HOME/.codex"
  touch "$codex_config"
  # Codex 0.149.1+: hooks activate via hooks.json alone; [features] codex_hooks is legacy.
  # Remove the entry if a previous install wrote it, and drop the [features] header
  # if it becomes empty.
  if grep -q 'codex_hooks' "$codex_config" 2>/dev/null; then
    tmp_cfg=$(mktemp)
    grep -v '^codex_hooks[[:space:]]*=' "$codex_config" | \
      awk '
        /^\[features\]/ { saved=$0; next }
        saved != "" {
          if (/^[[:space:]]*$/) { next }
          if (/^\[/) { saved=""; print; next }
          print saved; saved=""
        }
        { print }
      ' > "$tmp_cfg"
    mv "$tmp_cfg" "$codex_config"
    success "$HOME/.codex/config.toml — removed legacy codex_hooks (Codex 0.149.1+)"
  fi

  append_marked_block "$agents_md" "$TMP_ROOT/integrations/codex/AGENTS.md"

  install_skill_global
  install_skill_symlink "$HOME/.codex/skills"
}

setup_opencode() {
  header "Configuring OpenCode"

  plugins_dir="$HOME/.config/opencode/plugins"
  agents_md="$HOME/.config/opencode/AGENTS.md"

  mkdir -p "$plugins_dir"
  cp "$TMP_ROOT/integrations/opencode/plugins/prunesh.ts" "$plugins_dir/"
  success "Plugin installed to ${plugins_dir}"

  append_marked_block "$agents_md" "$TMP_ROOT/integrations/opencode/AGENTS.md"
}

agent_detected() {
  case "$1" in
    claudecode)
      command -v claude >/dev/null 2>&1 || [ -d "$HOME/.claude" ] || [ -f "$HOME/.claude.json" ]
      ;;
    cursor)
      command -v cursor >/dev/null 2>&1 || [ -d "$HOME/.cursor" ]
      ;;
    codex)
      command -v codex >/dev/null 2>&1 || [ -d "$HOME/.codex" ]
      ;;
    opencode)
      command -v opencode >/dev/null 2>&1 || [ -d "$HOME/.config/opencode" ]
      ;;
    *) return 1 ;;
  esac
}

detect_agents() {
  found=""
  for agent in claudecode cursor codex opencode; do
    if agent_detected "$agent"; then
      found="$found $agent"
    fi
  done
  printf '%s' "$found"
}

setup_agent() {
  agent="$1"
  case "$agent" in
    claudecode) setup_claudecode ;;
    cursor) setup_cursor ;;
    codex) setup_codex ;;
    opencode) setup_opencode ;;
    *) error "Unknown agent: ${agent}. Valid options: auto | claudecode | cursor | codex | opencode | all" ;;
  esac
}

warn_rtk() {
  if command -v rtk >/dev/null 2>&1; then
    warn "RTK is installed. To avoid conflicts, remove its hooks from agent settings"
    warn "Look for entries referencing rtk-rewrite.sh or rtk-post-tool-use.sh"
  fi
}

for arg in "$@"; do
  case "$arg" in
    --agent=*) AGENT="${arg#--agent=}" ;;
  esac
done

if [ "$DRY_RUN" = "true" ]; then
  info "Dry-run mode — no agent files will be written"
  info "Would configure agent=${AGENT}"
  success "Done (dry-run)."
  rm -rf "$TMP_DIR"
  exit 0
fi

resolve_scripts

case "$AGENT" in
  auto)
    detected=$(detect_agents)
    if [ -z "$detected" ]; then
      error "No compatible agent detected. Pass --agent=claudecode|cursor|codex|opencode|all"
    fi
    info "Auto-detected agents:${detected}"
    for selected in $detected; do
      setup_agent "$selected"
    done
    ;;
  all)
    for selected in claudecode cursor codex opencode; do
      setup_agent "$selected"
    done
    ;;
  claudecode|cursor|codex|opencode)
    setup_agent "$AGENT"
    ;;
  *)
    error "Unknown agent: ${AGENT}. Valid options: auto | claudecode | cursor | codex | opencode | all"
    ;;
esac

warn_rtk
rm -rf "$TMP_DIR"

header "Done"
success "prunesh $INSTALLED_VERSION configured for agent=${AGENT}"
printf "Restart the agent after installation.\n\n"
