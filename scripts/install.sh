#!/usr/bin/env bash
set -euo pipefail

# install.sh — Download and install the latest ghost-sync binary.
#
# One-liner:
#   curl -fsSL https://raw.githubusercontent.com/Fokir/ghost-sync/master/scripts/install.sh | bash
#
# Options:
#   GHOST_SYNC_VERSION=v0.2.0  Install a specific version
#   INSTALL_DIR=~/.local/bin   Override install directory

REPO="Fokir/ghost-sync"
BINARY="ghost-sync"
VERSION="${GHOST_SYNC_VERSION:-}"
INSTALL_DIR="${INSTALL_DIR:-}"
tmpdir=""

die() { echo "ERROR: $*" >&2; exit 1; }

# ---------------------------------------------------------------------------
# Detect OS and arch
# ---------------------------------------------------------------------------
detect_platform() {
  OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
  ARCH="$(uname -m)"

  case "$OS" in
    linux*)  OS="linux" ;;
    darwin*) OS="darwin" ;;
    mingw*|msys*|cygwin*) OS="windows" ;;
    *)       die "unsupported OS: $OS" ;;
  esac

  case "$ARCH" in
    x86_64|amd64)  ARCH="amd64" ;;
    arm64|aarch64) ARCH="arm64" ;;
    *)             die "unsupported architecture: $ARCH" ;;
  esac
}

# ---------------------------------------------------------------------------
# Determine install directory
# ---------------------------------------------------------------------------
detect_install_dir() {
  if [ -n "$INSTALL_DIR" ]; then
    return
  fi

  if [ "$OS" = "windows" ]; then
    INSTALL_DIR="$HOME/bin"
  elif [ -d "$HOME/.local/bin" ]; then
    INSTALL_DIR="$HOME/.local/bin"
  elif [ -d "/usr/local/bin" ] && [ -w "/usr/local/bin" ]; then
    INSTALL_DIR="/usr/local/bin"
  else
    INSTALL_DIR="$HOME/.local/bin"
  fi
}

# ---------------------------------------------------------------------------
# Get latest version from GitHub
# ---------------------------------------------------------------------------
get_latest_version() {
  if [ -n "$VERSION" ]; then
    return
  fi

  echo "Fetching latest version..."
  VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep '"tag_name"' \
    | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')" \
    || die "could not fetch latest version — check https://github.com/${REPO}/releases"

  [ -n "$VERSION" ] || die "could not determine latest version"
}

# ---------------------------------------------------------------------------
# Add directory to shell profile PATH
# ---------------------------------------------------------------------------
add_to_path() {
  local dir="$1"
  local export_line="export PATH=\"${dir}:\$PATH\""
  local profile=""

  # Detect shell profile.
  local shell_name
  shell_name="$(basename "${SHELL:-/bin/sh}")"

  case "$shell_name" in
    zsh)  profile="$HOME/.zshrc" ;;
    bash)
      if [ -f "$HOME/.bashrc" ]; then
        profile="$HOME/.bashrc"
      elif [ -f "$HOME/.bash_profile" ]; then
        profile="$HOME/.bash_profile"
      else
        profile="$HOME/.profile"
      fi
      ;;
    *)    profile="$HOME/.profile" ;;
  esac

  # On Windows (Git Bash/MSYS), prefer .bashrc.
  if [ "$OS" = "windows" ] && [ -z "$profile" ]; then
    profile="$HOME/.bashrc"
  fi

  # Skip if already present in the profile.
  if [ -f "$profile" ] && grep -qF "$dir" "$profile"; then
    return
  fi

  echo "" >> "$profile"
  echo "# Added by ghost-sync installer" >> "$profile"
  echo "$export_line" >> "$profile"

  echo ""
  echo "Added ${dir} to PATH in ${profile}"
  echo "Restart your terminal or run:"
  echo "  source ${profile}"
}

# ---------------------------------------------------------------------------
# Download and install
# ---------------------------------------------------------------------------
install() {
  local ext="tar.gz"
  [ "$OS" = "windows" ] && ext="zip"

  local filename="${BINARY}_${VERSION#v}_${OS}_${ARCH}.${ext}"
  local url="https://github.com/${REPO}/releases/download/${VERSION}/${filename}"

  echo ""
  echo "  Version  : $VERSION"
  echo "  Platform : ${OS}/${ARCH}"
  echo "  Install  : ${INSTALL_DIR}/${BINARY}"
  echo ""

  tmpdir="$(mktemp -d)"
  trap 'rm -rf "$tmpdir"' EXIT

  echo "Downloading ${url}..."
  curl -fsSL -o "${tmpdir}/${filename}" "$url" \
    || die "download failed — does $VERSION exist for ${OS}/${ARCH}?"

  echo "Extracting..."
  if [ "$ext" = "zip" ]; then
    unzip -qo "${tmpdir}/${filename}" -d "$tmpdir"
  else
    tar xzf "${tmpdir}/${filename}" -C "$tmpdir"
  fi

  # Find the binary (goreleaser puts it in a subdirectory or at root).
  local bin_path
  bin_path="$(find "$tmpdir" -name "$BINARY" -o -name "${BINARY}.exe" | head -n1)"
  [ -n "$bin_path" ] || die "binary not found in archive"

  mkdir -p "$INSTALL_DIR"
  mv "$bin_path" "${INSTALL_DIR}/"
  chmod +x "${INSTALL_DIR}/${BINARY}" 2>/dev/null || true

  echo ""
  echo "Installed ${BINARY} ${VERSION} to ${INSTALL_DIR}/${BINARY}"

  # Add install dir to PATH if missing.
  if ! echo "$PATH" | tr ':' '\n' | grep -qx "$INSTALL_DIR"; then
    add_to_path "$INSTALL_DIR"
  fi

  echo "Run 'ghost-sync version' to verify."
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
detect_platform
detect_install_dir
get_latest_version
install
