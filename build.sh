#!/usr/bin/env bash
# build.sh — Cross-compile music-tui for macOS / Linux / Windows
#
# Requirements:
#   go   — https://go.dev/dl/
#   zig  — https://ziglang.org/  (used as CGO cross-compiler)
#          macOS: brew install zig
#
# All targets require CGO (go-sqlite3).
#
# Platform notes:
#   macOS   — uses the host clang toolchain; no extra tools needed beyond
#             Xcode Command Line Tools (xcode-select --install).
#   Linux   — uses zig cc targeting glibc (dynamic) because oto/v3 links
#             ALSA via pkg-config, and libasound.a (musl static) is not
#             available in the cross-compile environment.  The resulting
#             binaries require libasound2 and glibc on the target host
#             (both are pre-installed on all mainstream Linux desktops).
#   Windows — uses zig cc targeting x86_64-windows-gnu (MinGW-w64 ABI).
#             oto/v3 uses WASAPI on Windows (no ALSA needed).
#
# Usage:  chmod +x build.sh && ./build.sh
#
# Environment variables:
#   SKIP_LINUX=1   — skip Linux targets (useful when cross-env is unavailable)

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
skip() { printf "  -  %-36s  skipped (%s)\n" "$1" "$2"; }

echo "Building $APP"
echo "──────────────────────────────────────────"

# ── macOS arm64 (native clang) ────────────────────────────────────────────────
printf "  %-36s" "macOS arm64  (native)"
GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 \
  go build -trimpath -ldflags="$LDFLAGS" \
  -o "$BIN/${APP}-darwin-arm64" "$PKG"
ok "$BIN/${APP}-darwin-arm64"

# ── macOS amd64 (native clang, cross-arch — works on arm64 host) ─────────────
printf "  %-36s" "macOS amd64  (native)"
GOOS=darwin GOARCH=amd64 CGO_ENABLED=1 \
  go build -trimpath -ldflags="$LDFLAGS" \
  -o "$BIN/${APP}-darwin-amd64" "$PKG"
ok "$BIN/${APP}-darwin-amd64"

# ── Linux targets ─────────────────────────────────────────────────────────────
if [[ "$SKIP_LINUX" == "1" ]]; then
  skip "Linux amd64" "SKIP_LINUX=1"
  skip "Linux arm64" "SKIP_LINUX=1"
else
  # Linux uses zig + glibc (dynamic) instead of musl static because oto/v3
  # requires libasound.a for static linking, which is not available in the
  # cross-compile environment.  The dynamic binaries depend only on:
  #   libasound.so.2  (ALSA — pre-installed on all desktop distros)
  #   glibc           (pre-installed everywhere)

  # ── Linux amd64 ────────────────────────────────────────────────────────────
  printf "  %-36s" "Linux  amd64 (zig, dynamic glibc+ALSA)"
  W=$(zig_cc x86_64-linux-gnu)
  GOOS=linux GOARCH=amd64 CGO_ENABLED=1 CC="$W" \
    go build -trimpath -ldflags="$LDFLAGS" \
    -o "$BIN/${APP}-linux-amd64" "$PKG"
  rm -f "$W"
  ok "$BIN/${APP}-linux-amd64"

  # ── Linux arm64 ────────────────────────────────────────────────────────────
  printf "  %-36s" "Linux  arm64 (zig, dynamic glibc+ALSA)"
  W=$(zig_cc aarch64-linux-gnu)
  GOOS=linux GOARCH=arm64 CGO_ENABLED=1 CC="$W" \
    go build -trimpath -ldflags="$LDFLAGS" \
    -o "$BIN/${APP}-linux-arm64" "$PKG"
  rm -f "$W"
  ok "$BIN/${APP}-linux-arm64"
fi

# ── Windows amd64 (zig, MinGW-w64 — WASAPI, no ALSA needed) ──────────────────
printf "  %-36s" "Windows amd64 (zig, MinGW-w64)"
W=$(zig_cc x86_64-windows-gnu)
GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC="$W" \
  go build -trimpath -ldflags="$LDFLAGS" \
  -o "$BIN/${APP}-windows-amd64.exe" "$PKG"
rm -f "$W"
ok "$BIN/${APP}-windows-amd64.exe"

echo "──────────────────────────────────────────"
echo "All binaries in ./$BIN/"
ls -lh "$BIN"/ | awk 'NR>1 {printf "  %6s  %s\n", $5, $9}'
