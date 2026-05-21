#!/usr/bin/env bash
# build.sh — Cross-compile music-tui for macOS / Linux / Windows
#
# Requirements:
#   go   — https://go.dev/dl/
#   zig  — https://ziglang.org/  (required for Linux targets)
#          macOS: brew install zig
#
# Platform notes:
#   macOS   — CGO_ENABLED=0; oto uses purego to dlopen AudioToolbox at runtime.
#             No system libraries required at build time.
#   Windows — CGO_ENABLED=0; oto uses golang.org/x/sys for WASAPI.
#             No system libraries required at build time.
#   Linux   — CGO_ENABLED=1; oto requires ALSA (libasound2-dev / alsa-lib-devel).
#             zig is used as the C cross-compiler so no host gcc is needed, but
#             the ALSA development headers must be present on the build host:
#               Ubuntu/Debian: apt install libasound2-dev
#               Fedora/RHEL:   dnf install alsa-lib-devel
#             The resulting binary links libasound dynamically; the target host
#             must have libasound2 (typically pre-installed on desktop Linux).
#
# Usage:  chmod +x build.sh && ./build.sh
#
# To skip Linux targets on macOS (where ALSA headers are unavailable):
#   SKIP_LINUX=1 ./build.sh

set -euo pipefail

APP="music-tui"
PKG="./cmd/music-tui"
BIN="bin"
LDFLAGS="-s -w"
SKIP_LINUX="${SKIP_LINUX:-0}"

mkdir -p "$BIN"

# ── Helper: create a disposable zig cc wrapper for a given target triple ──────
zig_cc() {
  local f
  f=$(mktemp /tmp/zig-cc-XXXXXX)
  printf '#!/bin/sh\nexec zig cc -target %s "$@"\n' "$1" > "$f"
  chmod +x "$f"
  echo "$f"
}

ok()   { printf "  ✓  %-12s  %s\n" "$(du -sh "$1" | cut -f1)" "$1"; }
skip() { printf "  -  %-30s  skipped (%s)\n" "$1" "$2"; }

echo "Building $APP"
echo "──────────────────────────────────────────"

# ── macOS arm64 (CGo-free) ────────────────────────────────────────────────────
printf "  %-38s" "macOS arm64  (CGo-free)"
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 \
  go build -trimpath -ldflags="$LDFLAGS" -o "$BIN/${APP}-darwin-arm64" "$PKG"
ok "$BIN/${APP}-darwin-arm64"

# ── macOS amd64 (CGo-free, cross-arch from arm64 host) ───────────────────────
printf "  %-38s" "macOS amd64  (CGo-free)"
GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 \
  go build -trimpath -ldflags="$LDFLAGS" -o "$BIN/${APP}-darwin-amd64" "$PKG"
ok "$BIN/${APP}-darwin-amd64"

# ── Windows amd64 (CGo-free) ──────────────────────────────────────────────────
printf "  %-38s" "Windows amd64 (CGo-free)"
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
  go build -trimpath -ldflags="$LDFLAGS" \
  -o "$BIN/${APP}-windows-amd64.exe" "$PKG"
ok "$BIN/${APP}-windows-amd64.exe"

# ── Linux targets (require ALSA headers on the build host) ───────────────────
if [[ "$SKIP_LINUX" == "1" ]]; then
  skip "Linux amd64" "SKIP_LINUX=1"
  skip "Linux arm64" "SKIP_LINUX=1"
elif ! pkg-config --exists alsa 2>/dev/null; then
  echo ""
  echo "  ⚠  Linux targets skipped: ALSA development headers not found."
  echo "     Install them and re-run to build Linux binaries:"
  echo "       Ubuntu/Debian: sudo apt install libasound2-dev"
  echo "       Fedora/RHEL:   sudo dnf install alsa-lib-devel"
  echo "     Or set SKIP_LINUX=1 to suppress this warning."
  echo ""
else
  # ── Linux amd64 (zig, CGo for ALSA, dynamic glibc) ─────────────────────────
  printf "  %-38s" "Linux  amd64 (CGo, dynamic ALSA)"
  W=$(zig_cc x86_64-linux-gnu)
  GOOS=linux GOARCH=amd64 CGO_ENABLED=1 CC="$W" \
    go build -trimpath -ldflags="$LDFLAGS" \
    -o "$BIN/${APP}-linux-amd64" "$PKG"
  rm -f "$W"
  ok "$BIN/${APP}-linux-amd64"

  # ── Linux arm64 (zig, CGo for ALSA, dynamic glibc) ─────────────────────────
  printf "  %-38s" "Linux  arm64 (CGo, dynamic ALSA)"
  W=$(zig_cc aarch64-linux-gnu)
  GOOS=linux GOARCH=arm64 CGO_ENABLED=1 CC="$W" \
    go build -trimpath -ldflags="$LDFLAGS" \
    -o "$BIN/${APP}-linux-arm64" "$PKG"
  rm -f "$W"
  ok "$BIN/${APP}-linux-arm64"
fi

echo "──────────────────────────────────────────"
echo "All binaries in ./$BIN/"
ls -lh "$BIN"/ | awk 'NR>1 {printf "  %6s  %s\n", $5, $9}'
