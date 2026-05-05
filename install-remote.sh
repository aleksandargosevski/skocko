#!/usr/bin/env bash
#
# Standalone installer for skocko.
# Usage: curl -fsSL https://raw.githubusercontent.com/aleksandargosevski/skocko/main/install-remote.sh | bash
#
set -euo pipefail

REPO="aleksandargosevski/skocko"
BINARY="skocko"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

get_latest_version() {
  curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" |
    grep '"tag_name"' | sed -E 's/.*"v([^"]+)".*/\1/'
}

detect_platform() {
  local os arch
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  arch="$(uname -m)"

  case "$arch" in
    x86_64|amd64) arch="amd64" ;;
    aarch64|arm64) arch="arm64" ;;
    *) echo "Unsupported architecture: $arch" >&2; exit 1 ;;
  esac

  case "$os" in
    linux|darwin) ;;
    *) echo "Unsupported OS: $os" >&2; exit 1 ;;
  esac

  echo "${os}_${arch}"
}

main() {
  local version platform url tmpdir

  echo "Installing ${BINARY}..."

  version="${VERSION:-$(get_latest_version)}"
  platform="$(detect_platform)"
  url="https://github.com/${REPO}/releases/download/v${version}/${BINARY}_${platform}.tar.gz"

  echo "  Version:  v${version}"
  echo "  Platform: ${platform}"
  echo "  URL:      ${url}"
  echo ""

  tmpdir="$(mktemp -d)"
  trap 'rm -rf "$tmpdir"' EXIT

  curl -fsSL "$url" -o "${tmpdir}/${BINARY}.tar.gz"
  tar -xzf "${tmpdir}/${BINARY}.tar.gz" -C "$tmpdir"

  mkdir -p "$INSTALL_DIR"
  mv "${tmpdir}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
  chmod +x "${INSTALL_DIR}/${BINARY}"

  echo "Installed ${BINARY} v${version} to ${INSTALL_DIR}/${BINARY}"

  if ! echo "$PATH" | tr ':' '\n' | grep -qx "$INSTALL_DIR"; then
    echo ""
    echo "Warning: ${INSTALL_DIR} is not in your PATH."
    echo "Add this to your shell config:"
    echo ""
    echo "  export PATH=\"${INSTALL_DIR}:\$PATH\""
  fi

  echo ""
  echo "Setup: create ~/.config/skocko/skocko.yaml with your project paths:"
  echo ""
  echo "  project_paths:"
  echo "    - ~/Sites"
  echo ""
  echo "tmux: add to tmux.conf for popup picker:"
  echo ""
  echo "  bind-key \"T\" display-popup -E -w 80% -h 70% \"skocko\""
}

main "$@"
