#!/bin/bash
set -euo pipefail

REPO="Lalcs/a2ahoy"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
BINARY_NAME="a2ahoy"
tmp_file=""

# Degrade to plain text when stdout is not a terminal (e.g. log redirect).
if [ -t 1 ]; then
  C_RESET=$'\033[0m'
  C_BLUE=$'\033[1;34m'
  C_GREEN=$'\033[1;32m'
  C_YELLOW=$'\033[1;33m'
  C_RED=$'\033[1;31m'
else
  C_RESET=""
  C_BLUE=""
  C_GREEN=""
  C_YELLOW=""
  C_RED=""
fi

info()    { printf '%s[INFO]%s  %s\n' "$C_BLUE"   "$C_RESET" "$*"; }
success() { printf '%s[OK]%s    %s\n' "$C_GREEN"  "$C_RESET" "$*"; }
warn()    { printf '%s[WARN]%s  %s\n' "$C_YELLOW" "$C_RESET" "$*" >&2; }
error()   { printf '%s[ERROR]%s %s\n' "$C_RED"    "$C_RESET" "$*" >&2; exit 1; }

detect_platform() {
  local os arch

  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  case "$os" in
    linux|darwin) ;;
    *)            error "Unsupported OS: $os" ;;
  esac

  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64)   arch="amd64" ;;
    aarch64|arm64)   arch="arm64" ;;
    *)               error "Unsupported architecture: $arch" ;;
  esac

  echo "${os}-${arch}"
}

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

# Custom INSTALL_DIR values like /opt/bin need sudo; the default $HOME/.local/bin does not.
ensure_install_dir() {
  if [ -d "$INSTALL_DIR" ]; then
    return 0
  fi

  info "Creating install directory: $INSTALL_DIR"
  if mkdir -p "$INSTALL_DIR" 2>/dev/null; then
    return 0
  fi

  if command -v sudo >/dev/null 2>&1; then
    sudo mkdir -p "$INSTALL_DIR" || error "Failed to create $INSTALL_DIR"
  else
    error "Failed to create $INSTALL_DIR (no sudo available)"
  fi
}

print_path_guidance() {
  local shell_name rc_file cmd
  shell_name="${SHELL##*/}"

  warn "$INSTALL_DIR is not in your PATH."
  echo

  case "$shell_name" in
    bash) rc_file="$HOME/.bashrc" ;;
    zsh)  rc_file="$HOME/.zshrc" ;;
    fish)
      echo "  Run this command to add it to your PATH:"
      echo
      printf '    %sfish_add_path $HOME/.local/bin%s\n' "$C_GREEN" "$C_RESET"
      echo
      return
      ;;
    *)
      echo "  Add the following to your shell startup file:"
      echo
      printf '    %sexport PATH="$HOME/.local/bin:$PATH"%s\n' "$C_GREEN" "$C_RESET"
      echo
      return
      ;;
  esac

  # bash/zsh: rc_file is the single source of truth for the target file.
  cmd="echo 'export PATH=\"\$HOME/.local/bin:\$PATH\"' >> $rc_file"
  echo "  Run this command to add it to your shell config:"
  echo
  printf '    %s%s%s\n' "$C_GREEN" "$cmd" "$C_RESET"
  echo
  echo "  Then restart your shell or run:"
  printf '    %ssource %s%s\n' "$C_GREEN" "$rc_file" "$C_RESET"
  echo
}

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

  ensure_install_dir

  info "Installing to ${INSTALL_DIR}/${BINARY_NAME}..."
  if [ -w "$INSTALL_DIR" ]; then
    mv "$tmp_file" "${INSTALL_DIR}/${BINARY_NAME}"
  else
    sudo mv "$tmp_file" "${INSTALL_DIR}/${BINARY_NAME}"
  fi

  success "Installed ${BINARY_NAME} ${version} to ${INSTALL_DIR}/${BINARY_NAME}"

  case ":$PATH:" in
    *":$INSTALL_DIR:"*)
      success "${INSTALL_DIR} is already in your PATH. Run '${BINARY_NAME} --help' to get started."
      ;;
    *)
      print_path_guidance
      ;;
  esac
}

main
