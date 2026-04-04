#!/bin/sh
# gtk-ai installer
# Usage: curl -sSL https://raw.githubusercontent.com/jmeiracorbal/gtk-ai/main/install.sh | sh
#
# To configure only the Claude Code side (skip binary install):
#   GTKAI_CLAUDE_ONLY=1 sh install.sh

set -e

REPO="jmeiracorbal/gtk-ai"
BINARY="gtkai"
INSTALL_DIR="${GTKAI_INSTALL_DIR:-$HOME/.local/bin}"
CLAUDE_ONLY="${GTKAI_CLAUDE_ONLY:-}"
TMP_DIR=$(mktemp -d)

# ── Colours ───────────────────────────────────────────────────────────────────

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

# ── Banner ────────────────────────────────────────────────────────────────────

printf "${BOLD}"
cat <<'EOF'
   gtk-ai — rule-based output filtering for Claude Code
EOF
printf "${RESET}\n"

# ── Detect OS and architecture ────────────────────────────────────────────────

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

# ── Detect tools ──────────────────────────────────────────────────────────────

header "Checking dependencies"

HAS_GO=false
HAS_CURL=false
HAS_WGET=false
HAS_JQ=false

command -v go    >/dev/null 2>&1 && HAS_GO=true   && success "Go found: $(go version | awk '{print $3}')"
command -v curl  >/dev/null 2>&1 && HAS_CURL=true && success "curl found"
command -v wget  >/dev/null 2>&1 && HAS_WGET=true
command -v git   >/dev/null 2>&1 || error "git is required. Install it and retry."
command -v jq    >/dev/null 2>&1 && HAS_JQ=true   || warn "jq not found — marketplace JSON update will be skipped"

# ── Download helpers ──────────────────────────────────────────────────────────

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

# ── jq helpers ────────────────────────────────────────────────────────────────

# Run jq and write output atomically; fails hard if jq errors.
# Usage: jq_update [jq-options...] filter file
# All arguments are passed to jq; the last argument is the file updated in place.
jq_update() {
  eval "file=\${$#}"
  tmp=$(mktemp)
  if ! jq "$@" > "$tmp"; then
    rm -f "$tmp"
    error "jq failed updating $file"
  fi
  mv "$tmp" "$file"
}


# ── Install binary ────────────────────────────────────────────────────────────

if [ -n "$CLAUDE_ONLY" ]; then
  header "Skipping binary install (GTKAI_CLAUDE_ONLY=1)"

  if ! command -v "$BINARY" >/dev/null 2>&1 && ! [ -x "$INSTALL_DIR/$BINARY" ]; then
    error "$BINARY not found. Install it first or unset GTKAI_CLAUDE_ONLY."
  fi

  if [ -x "$INSTALL_DIR/$BINARY" ]; then
    INSTALLED_VERSION=$("$INSTALL_DIR/$BINARY" version | awk '{print $2}')
  else
    INSTALLED_VERSION=$(gtkai version | awk '{print $2}')
  fi
  success "$BINARY $INSTALLED_VERSION found"
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
        if [ "${GTKAI_SKIP_CHECKSUM:-}" = "1" ]; then
          warn "Could not fetch checksum — proceeding because GTKAI_SKIP_CHECKSUM=1"
        else
          error "Could not fetch checksum. Set GTKAI_SKIP_CHECKSUM=1 to bypass."
        fi
      else
        if command -v shasum >/dev/null 2>&1; then
          ACTUAL=$(shasum -a 256 "$TMP_DIR/$BINARY" | awk '{print $1}')
        elif command -v sha256sum >/dev/null 2>&1; then
          ACTUAL=$(sha256sum "$TMP_DIR/$BINARY" | awk '{print $1}')
        else
          error "No SHA256 tool found (shasum/sha256sum). Set GTKAI_SKIP_CHECKSUM=1 to bypass."
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
      git clone --depth 1 "https://github.com/$REPO.git" "$TMP_DIR/gtk-ai" >/dev/null 2>&1

      info "Building $BINARY..."
      cd "$TMP_DIR/gtk-ai"
      go build -o "$INSTALL_DIR/$BINARY" ./cmd/gtkai/
      cd - >/dev/null
      success "Built from source"
    fi
  fi

  if ! "$INSTALL_DIR/$BINARY" version >/dev/null 2>&1; then
    error "Binary installed but failed to run. Check $INSTALL_DIR/$BINARY"
  fi

  INSTALLED_VERSION=$("$INSTALL_DIR/$BINARY" version | awk '{print $2}')
  success "$BINARY $INSTALLED_VERSION installed to $INSTALL_DIR/$BINARY"

  # ── Add to PATH ──────────────────────────────────────────────────────────────

  header "Configuring PATH"

  add_to_path() {
    shell_rc="$1"
    if [ -f "$shell_rc" ]; then
      if ! grep -q "$INSTALL_DIR" "$shell_rc" 2>/dev/null; then
        printf '\n# gtk-ai\nexport PATH="%s:$PATH"\n' "$INSTALL_DIR" >> "$shell_rc"
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

# ── Claude Code setup ─────────────────────────────────────────────────────────

header "Configuring Claude Code"

CLAUDE_DIR="$HOME/.claude"
SETTINGS_FILE="$CLAUDE_DIR/settings.json"
KNOWN_MARKETPLACES="$CLAUDE_DIR/plugins/known_marketplaces.json"
MARKETPLACE_DIR="$CLAUDE_DIR/plugins/marketplaces/gtk-ai"
PROTOCOL_DOC="$CLAUDE_DIR/gtk-ai.md"
CLAUDE_MD="$CLAUDE_DIR/CLAUDE.md"

mkdir -p "$CLAUDE_DIR/plugins/marketplaces"

# Register marketplace in settings.json
if [ -f "$SETTINGS_FILE" ] && $HAS_JQ; then
  if jq -e '.extraKnownMarketplaces["gtk-ai"]' "$SETTINGS_FILE" >/dev/null 2>&1; then
    info "$HOME/.claude/settings.json — marketplace gtk-ai already registered"
  else
    jq_update '.extraKnownMarketplaces["gtk-ai"] = {"source": {"source": "github", "repo": "jmeiracorbal/gtk-ai"}}' "$SETTINGS_FILE"
    success "$HOME/.claude/settings.json — marketplace gtk-ai registered"
  fi
elif [ ! -f "$SETTINGS_FILE" ]; then
  jq_create '{
  "extraKnownMarketplaces": {
    "gtk-ai": {
      "source": {
        "source": "github",
        "repo": "jmeiracorbal/gtk-ai"
      }
    }
  }
}' "$SETTINGS_FILE"
  success "$HOME/.claude/settings.json — created with marketplace gtk-ai"
else
  warn "jq not found — skipping settings.json update. Add the marketplace manually."
fi

# Clone marketplace repo to local cache, pinned to installed version
if [ -d "$MARKETPLACE_DIR/.git" ]; then
  info "Marketplace cache exists — updating..."
  git -C "$MARKETPLACE_DIR" pull --ff-only -q 2>/dev/null || warn "Could not update marketplace cache"
  if git -C "$MARKETPLACE_DIR" checkout "v$INSTALLED_VERSION" -q 2>/dev/null; then
    success "$HOME/.claude/plugins/marketplaces/gtk-ai pinned to v$INSTALLED_VERSION"
  else
    warn "Tag v$INSTALLED_VERSION not found — using default branch"
  fi
else
  info "Cloning marketplace cache..."
  if git clone --depth 1 --branch "v$INSTALLED_VERSION" "https://github.com/$REPO.git" "$MARKETPLACE_DIR" >/dev/null 2>&1; then
    success "$HOME/.claude/plugins/marketplaces/gtk-ai cloned at v$INSTALLED_VERSION"
  elif git clone --depth 1 "https://github.com/$REPO.git" "$MARKETPLACE_DIR" >/dev/null 2>&1; then
    warn "Tag v$INSTALLED_VERSION not found — using default branch"
  else
    error "Failed to clone marketplace repository"
  fi
fi

# Register in known_marketplaces.json
NOW=$(date -u +"%Y-%m-%dT%H:%M:%S.000Z")
if [ -f "$KNOWN_MARKETPLACES" ] && $HAS_JQ; then
  if jq -e '.["gtk-ai"]' "$KNOWN_MARKETPLACES" >/dev/null 2>&1; then
    info "$HOME/.claude/plugins/known_marketplaces.json — gtk-ai already indexed"
  else
    jq_update \
      --arg loc "$MARKETPLACE_DIR" --arg now "$NOW" \
      '.["gtk-ai"] = {"source": {"source": "github", "repo": "jmeiracorbal/gtk-ai"}, "installLocation": $loc, "lastUpdated": $now}' \
      "$KNOWN_MARKETPLACES"
    success "$HOME/.claude/plugins/known_marketplaces.json — gtk-ai indexed"
  fi
elif ! [ -f "$KNOWN_MARKETPLACES" ]; then
  jq -n --arg loc "$MARKETPLACE_DIR" --arg now "$NOW" \
    '{"gtk-ai": {"source": {"source": "github", "repo": "jmeiracorbal/gtk-ai"}, "installLocation": $loc, "lastUpdated": $now}}' \
    > "$KNOWN_MARKETPLACES"
  success "$HOME/.claude/plugins/known_marketplaces.json — created with gtk-ai"
else
  warn "jq not found — skipping known_marketplaces.json update."
fi

# Write context doc (note: hook becomes active only after plugin install)
cat > "$PROTOCOL_DOC" <<'PROTOCOL'
## gtk-ai — rule-based output filtering

gtk-ai filters Bash, grep, find, ls, git, and MCP tool output before it enters
the context. Depending on the command, it applies truncation, extension grouping,
condensed formatting, or comment line removal.

The hook is active only when the Claude plugin is installed and enabled.
Run `claude plugin install -s user gtk-ai@gtk-ai` if you have not done so.
PROTOCOL
success "$HOME/.claude/gtk-ai.md written"

# Inject @gtk-ai.md into CLAUDE.md
if [ -f "$CLAUDE_MD" ]; then
  if grep -q "@gtk-ai.md" "$CLAUDE_MD" 2>/dev/null; then
    info "$HOME/.claude/CLAUDE.md — already up to date"
  else
    printf '\n@gtk-ai.md\n' >> "$CLAUDE_MD"
    success "$HOME/.claude/CLAUDE.md updated"
  fi
else
  printf '@gtk-ai.md\n' > "$CLAUDE_MD"
  success "$HOME/.claude/CLAUDE.md created"
fi

# ── RTK warning ───────────────────────────────────────────────────────────────

if command -v rtk >/dev/null 2>&1; then
  warn "RTK is installed. To avoid conflicts, remove its hooks from ~/.claude/settings.json"
  warn "Look for entries referencing rtk-rewrite.sh or rtk-post-tool-use.sh"
fi

# ── Cleanup ───────────────────────────────────────────────────────────────────

rm -rf "$TMP_DIR"

# ── Done ──────────────────────────────────────────────────────────────────────

if [ -n "$CLAUDE_ONLY" ]; then
  printf "\n${BOLD}${GREEN}%s${RESET}\n\n" "Claude configuration completed."
else
  printf "\n${BOLD}${GREEN}%s${RESET}\n\n" "gtk-ai installed."
fi
printf "To activate the Claude plugin, run:\n\n"
printf "  ${BOLD}%s${RESET}\n\n" "claude plugin install -s user gtk-ai@gtk-ai"
printf "Then restart Claude Code.\n\n"
