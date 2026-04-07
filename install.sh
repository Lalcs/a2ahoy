#!/bin/bash
set -euo pipefail

REPO="Lalcs/a2ahoy"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
BINARY_NAME="a2ahoy"
tmp_file=""

# --- Helper functions ---

info()  { echo "[INFO]  $*"; }
error() { echo "[ERROR] $*" >&2; exit 1; }

# --- Detect OS and Architecture ---

detect_platform() {
  local os arch

  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  case "$os" in
    linux)  os="linux" ;;
    darwin) os="darwin" ;;
    *)      error "Unsupported OS: $os" ;;
  esac

  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64)   arch="amd64" ;;
    aarch64|arm64)   arch="arm64" ;;
    *)               error "Unsupported architecture: $arch" ;;
  esac

  echo "${os}-${arch}"
}

# --- Fetch latest version from GitHub API ---

fetch_latest_version() {
  local url="https://api.github.com/repos/${REPO}/releases/latest"
  local version

  if command -v curl &>/dev/null; then
    version="$(curl -fsSL "$url" | grep '"tag_name"' | cut -d '"' -f 4)"
  elif command -v wget &>/dev/null; then
    version="$(wget -qO- "$url" | grep '"tag_name"' | cut -d '"' -f 4)"
  else
    error "curl or wget is required"
  fi

  if [ -z "$version" ]; then
    error "Failed to fetch latest version from GitHub"
  fi

  echo "$version"
}

# --- Main ---

main() {
  local platform version download_url

  info "Detecting platform..."
  platform="$(detect_platform)"
  info "Platform: $platform"

  info "Fetching latest version..."
  version="$(fetch_latest_version)"
  info "Latest version: $version"

  download_url="https://github.com/${REPO}/releases/download/${version}/${BINARY_NAME}-${platform}"
  info "Downloading ${BINARY_NAME} ${version}..."

  tmp_file="$(mktemp)"
  trap 'rm -f "$tmp_file"' EXIT

  if command -v curl &>/dev/null; then
    curl -fsSL "$download_url" -o "$tmp_file"
  else
    wget -qO "$tmp_file" "$download_url"
  fi

  chmod +x "$tmp_file"

  info "Installing to ${INSTALL_DIR}/${BINARY_NAME}..."
  if [ -w "$INSTALL_DIR" ]; then
    mv "$tmp_file" "${INSTALL_DIR}/${BINARY_NAME}"
  else
    sudo mv "$tmp_file" "${INSTALL_DIR}/${BINARY_NAME}"
  fi

  info "Successfully installed ${BINARY_NAME} ${version} to ${INSTALL_DIR}/${BINARY_NAME}"
  "${INSTALL_DIR}/${BINARY_NAME}" --help 2>/dev/null | head -1 || true
}

main
