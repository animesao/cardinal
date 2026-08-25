#!/bin/sh
set -e

BOLD=$(tput bold 2>/dev/null || echo "")
RED=$(tput setaf 1 2>/dev/null || echo "")
GREEN=$(tput setaf 2 2>/dev/null || echo "")
RESET=$(tput sgr0 2>/dev/null || echo "")

info()  { echo "${BOLD}${GREEN}[cardinal]${RESET} $*"; }
warn()  { echo "${BOLD}${RED}[cardinal]${RESET} $*" >&2; }

FORCE=false
for arg do
    case "$arg" in
        -f|--force|-y|--yes) FORCE=true ;;
    esac
done

info "Uninstalling cardinal..."

unmount_overlay() {
    OVERLAY_DIR="${CARDINAL_DIR:-$HOME/.cardinal}/overlay"
    if [ -d "$OVERLAY_DIR" ]; then
        for d in "$OVERLAY_DIR"/*/merged; do
            if [ -d "$d" ] && mountpoint -q "$d" 2>/dev/null; then
                umount "$d" 2>/dev/null || true
                info "  Unmounted $d"
            fi
        done
    fi
}

PREFIX="${PREFIX:-/usr/local}"
BIN="$PREFIX/bin/cardinal"

if [ -f "$BIN" ]; then
    rm -f "$BIN"
    info "Removed $BIN"
else
    warn "cardinal binary not found at $BIN"
fi

CARDINAL_DIR="${CARDINAL_DIR:-$HOME/.cardinal}"
if [ -d "$CARDINAL_DIR" ]; then
    echo ""
    if [ "$FORCE" = "true" ] || [ ! -t 0 ]; then
        unmount_overlay
        rm -rf "$CARDINAL_DIR"
        info "Removed $CARDINAL_DIR"
    else
        warn "WARNING: This will DELETE all images, containers, and data"
        printf "Remove %s? [y/N] " "$CARDINAL_DIR"
        read -r confirm
        if [ "$confirm" = "y" ] || [ "$confirm" = "Y" ]; then
            unmount_overlay
            rm -rf "$CARDINAL_DIR"
            info "Removed $CARDINAL_DIR"
        else
            info "Skipped $CARDINAL_DIR (remove manually: rm -rf $CARDINAL_DIR)"
        fi
    fi
fi

info "cardinal uninstalled."
