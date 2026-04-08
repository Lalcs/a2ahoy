#!/bin/bash
set -euo pipefail

INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
BINARY_NAME="a2ahoy"

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

# Falls back to `command -v` so legacy /usr/local/bin installs are still found.
# Does not follow symlinks: package-manager installs should be removed via the package manager.
resolve_binary() {
  local candidate="${INSTALL_DIR}/${BINARY_NAME}"

  if [ -e "$candidate" ] || [ -L "$candidate" ]; then
    echo "$candidate"
    return 0
  fi

  if command -v "$BINARY_NAME" &>/dev/null; then
    command -v "$BINARY_NAME"
    return 0
  fi

  return 1
}

remove_path() {
  local path="$1"
  local parent

  parent="$(dirname "$path")"

  if [ ! -e "$path" ] && [ ! -L "$path" ]; then
    return 0
  fi

  if [ -w "$parent" ]; then
    rm -f "$path"
  else
    sudo rm -f "$path"
  fi
}

main() {
  local binary_path binary_dir backup_path

  info "Locating ${BINARY_NAME}..."
  if ! binary_path="$(resolve_binary)"; then
    warn "${BINARY_NAME} not found in ${INSTALL_DIR} or on PATH."
    info "Nothing to uninstall."
    exit 0
  fi

  binary_dir="$(dirname "$binary_path")"
  backup_path="${binary_dir}/${BINARY_NAME}.bak"

  info "Found ${BINARY_NAME} at ${binary_path}"

  if [ ! -w "$binary_dir" ]; then
    info "Install directory is not writable; sudo will be required."
    if ! command -v sudo &>/dev/null; then
      error "sudo is required to remove ${binary_path} but is not installed."
    fi
  fi

  info "Removing ${binary_path}..."
  remove_path "$binary_path"

  if [ -e "$backup_path" ] || [ -L "$backup_path" ]; then
    info "Removing leftover backup ${backup_path}..."
    remove_path "$backup_path"
  fi

  success "Successfully uninstalled ${BINARY_NAME} from ${binary_dir}"

  if command -v "$BINARY_NAME" &>/dev/null; then
    warn "Another ${BINARY_NAME} is still on your PATH at $(command -v "$BINARY_NAME")."
    warn "It was not removed by this script (likely installed via a package manager)."
  fi
}

main
