#!/usr/bin/env bash
set -euo pipefail

# dck AppImage desktop installer.
# AppRun invokes this script when the AppImage is opened without arguments
# (normally by double-clicking it) or with the explicit --install action.

INSTALL_PATH="/usr/local/bin/dck"
SOURCE_APPIMAGE="${1:-${APPIMAGE:-}}"
SOURCE_BINARY="${2:-}"

info() { printf '[dck] %s\n' "$*"; }
warn() { printf '[dck] warning: %s\n' "$*" >&2; }
fail() { printf '[dck] error: %s\n' "$*" >&2; exit 1; }

if [[ -z "$SOURCE_APPIMAGE" || ! -f "$SOURCE_APPIMAGE" ]]; then
  fail "the original AppImage path was not provided; run the AppImage from a file manager or use the CLI with an argument"
fi

if [[ "$SOURCE_APPIMAGE" != /* ]]; then
  SOURCE_APPIMAGE="$(CDPATH= cd -- "$(dirname -- "$SOURCE_APPIMAGE")" && pwd)/$(basename -- "$SOURCE_APPIMAGE")"
fi

if [[ ! -r "$SOURCE_APPIMAGE" ]]; then
  fail "cannot read AppImage: $SOURCE_APPIMAGE"
fi

if [[ -z "$SOURCE_BINARY" || ! -f "$SOURCE_BINARY" || ! -x "$SOURCE_BINARY" ]]; then
  fail "the embedded dck binary was not found; run the AppImage directly instead of a manually extracted AppRun"
fi

# FUSE AppImage mounts are readable by the desktop user but often invisible to
# root. Copy the embedded binary to a normal temporary file before invoking
# sudo; otherwise sudo install fails with "cannot stat ... Permission denied".
TMP_BINARY="$(mktemp "${TMPDIR:-/tmp}/dck-appimage.XXXXXX")"
cleanup() { rm -f "$TMP_BINARY"; }
trap cleanup EXIT

if ! cp -- "$SOURCE_BINARY" "$TMP_BINARY"; then
  fail "cannot copy the embedded dck binary from the AppImage mount"
fi
chmod 0755 "$TMP_BINARY"

if [[ "$EUID" -eq 0 ]]; then
  install -D -m 0755 "$TMP_BINARY" "$INSTALL_PATH"
else
  if ! command -v sudo >/dev/null 2>&1; then
    fail "sudo is required to install dck to $INSTALL_PATH"
  fi
  sudo install -D -m 0755 "$TMP_BINARY" "$INSTALL_PATH"
fi
info "Installed the embedded dck binary from $SOURCE_APPIMAGE to $INSTALL_PATH"

if [[ -d /run/systemd/system ]]; then
  info "Installing and starting the dck supervisor"
  if [[ "$EUID" -eq 0 ]]; then
    "$INSTALL_PATH" bootstrap --install
  else
    sudo "$INSTALL_PATH" bootstrap --install
  fi
else
  warn "systemd was not detected; the dck command was installed, but boot recovery is unavailable"
fi

if [[ "$EUID" -eq 0 ]]; then
  VERSION_OUTPUT="$($INSTALL_PATH version 2>&1 || true)"
else
  VERSION_OUTPUT="$(sudo "$INSTALL_PATH" version 2>&1 || true)"
fi

printf '\n'
info "Installation complete."
printf '%s\n' "$VERSION_OUTPUT"
info "Use 'dck --help' in a terminal to get started."
