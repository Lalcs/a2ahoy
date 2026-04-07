#!/bin/bash
set -euo pipefail

INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
BINARY_NAME="a2ahoy"

# --- Helper functions ---

info()  { echo "[INFO]  $*"; }
warn()  { echo "[WARN]  $*" >&2; }
error() { echo "[ERROR] $*" >&2; exit 1; }

# --- Resolve binary location ---
#
# Resolution order:
#   1. ${INSTALL_DIR}/${BINARY_NAME} (matches install.sh default)
#   2. `command -v` lookup as a fallback for binaries installed elsewhere
#
# We deliberately do NOT follow symlinks: package-manager installs
# (Homebrew, apt) should be removed via the package manager.

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

# --- Remove a single path with sudo fallback ---

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

# --- Main ---

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

  info "Successfully uninstalled ${BINARY_NAME} from ${binary_dir}"

  if command -v "$BINARY_NAME" &>/dev/null; then
    warn "Another ${BINARY_NAME} is still on your PATH at $(command -v ${BINARY_NAME})."
    warn "It was not removed by this script (likely installed via a package manager)."
  fi
}

main
