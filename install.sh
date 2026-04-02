#!/bin/sh
# gtk-ai installer
# Usage: curl -sSL https://raw.githubusercontent.com/jmeiracorbal/gtk-ai/main/install.sh | sh

set -e

REPO="jmeiracorbal/gtk-ai"
BINARY="gtkai"
INSTALL_DIR="${GTKAI_INSTALL_DIR:-$HOME/.local/bin}"
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

command -v go    >/dev/null 2>&1 && HAS_GO=true   && success "Go found: $(go version | awk '{print $3}')"
command -v curl  >/dev/null 2>&1 && HAS_CURL=true && success "curl found"
command -v wget  >/dev/null 2>&1 && HAS_WGET=true

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

# ── Install binary ────────────────────────────────────────────────────────────

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
      warn "Could not fetch checksum — skipping verification"
    else
      if command -v shasum >/dev/null 2>&1; then
        ACTUAL=$(shasum -a 256 "$TMP_DIR/$BINARY" | awk '{print $1}')
      elif command -v sha256sum >/dev/null 2>&1; then
        ACTUAL=$(sha256sum "$TMP_DIR/$BINARY" | awk '{print $1}')
      else
        warn "No SHA256 tool found (shasum/sha256sum) — skipping verification"
        ACTUAL="$EXPECTED"
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

    if ! command -v git >/dev/null 2>&1; then
      error "git is required to build from source. Install it and retry."
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

# Verify binary works
if ! "$INSTALL_DIR/$BINARY" version >/dev/null 2>&1; then
  error "Binary installed but failed to run. Check $INSTALL_DIR/$BINARY"
fi

INSTALLED_VERSION=$("$INSTALL_DIR/$BINARY" version | awk '{print $2}')
success "$BINARY $INSTALLED_VERSION installed to $INSTALL_DIR/$BINARY"

# ── Add to PATH ───────────────────────────────────────────────────────────────

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

# ── Claude Code setup ─────────────────────────────────────────────────────────

header "Configuring Claude Code"

"$INSTALL_DIR/$BINARY" setup

# ── RTK warning ───────────────────────────────────────────────────────────────

if command -v rtk >/dev/null 2>&1; then
  warn "RTK is installed. To avoid conflicts, remove its hooks from ~/.claude/settings.json"
  warn "Look for entries referencing rtk-rewrite.sh or rtk-post-tool-use.sh"
fi

# ── Cleanup ───────────────────────────────────────────────────────────────────

rm -rf "$TMP_DIR"

# ── Done ──────────────────────────────────────────────────────────────────────

printf "\n${BOLD}${GREEN}gtk-ai installed successfully!${RESET}\n\n"
