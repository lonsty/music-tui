#!/usr/bin/env bash
# build.sh — Cross-compile music-tui for macOS / Linux / Windows
#
# Requirements:
#   go   — https://go.dev/dl/
#   zig  — https://ziglang.org/  (used as CGO cross-compiler for Windows and
#          cross-arch Linux builds)
#          macOS: brew install zig
#
# All targets require CGO (go-sqlite3).
#
# Platform notes:
#   macOS   — uses the host clang toolchain.
#             Requires Xcode Command Line Tools: xcode-select --install
#   Linux   — MUST be built ON a Linux host.
#             oto/v3 uses `pkg-config alsa` which requires ALSA development
#             headers that are not available when cross-compiling from macOS.
#             Install headers first:
#               Ubuntu/Debian: sudo apt install libasound2-dev
#               Fedora/RHEL:   sudo dnf install alsa-lib-devel
#             The resulting binaries link libasound dynamically; libasound.so.2
#             is pre-installed on all mainstream Linux desktop distributions.
#   Windows — uses zig cc with x86_64-windows-gnu (MinGW-w64 ABI).
#             oto/v3 uses WASAPI on Windows — no ALSA dependency.
#
# Usage:  chmod +x build.sh && ./build.sh
#
# Environment variables:
#   SKIP_LINUX=1    — skip Linux targets (auto-set when not on Linux)
#   SKIP_WINDOWS=1  — skip Windows target

set -euo pipefail

APP="music-tui"
PKG="./cmd/music-tui"
BIN="bin"
LDFLAGS="-s -w"

# Auto-skip Linux when not on Linux: ALSA pkg-config headers are unavailable.
if [[ "$(uname -s)" != "Linux" ]]; then
  SKIP_LINUX="${SKIP_LINUX:-1}"
else
  SKIP_LINUX="${SKIP_LINUX:-0}"
fi
SKIP_WINDOWS="${SKIP_WINDOWS:-0}"

mkdir -p "$BIN"

# ── Helper: disposable zig cc wrapper for a given target triple ───────────────
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

# ── macOS (Darwin host only) ──────────────────────────────────────────────────
if [[ "$(uname -s)" == "Darwin" ]]; then
  printf "  %-36s" "macOS arm64  (native clang)"
  GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 \
    go build -trimpath -ldflags="$LDFLAGS" \
    -o "$BIN/${APP}-darwin-arm64" "$PKG"
  ok "$BIN/${APP}-darwin-arm64"

  printf "  %-36s" "macOS amd64  (native clang)"
  GOOS=darwin GOARCH=amd64 CGO_ENABLED=1 \
    go build -trimpath -ldflags="$LDFLAGS" \
    -o "$BIN/${APP}-darwin-amd64" "$PKG"
  ok "$BIN/${APP}-darwin-amd64"
else
  skip "macOS arm64" "not on Darwin"
  skip "macOS amd64" "not on Darwin"
fi

# ── Linux (Linux host only — ALSA headers required) ───────────────────────────
if [[ "$SKIP_LINUX" == "1" ]]; then
  skip "Linux amd64" "requires Linux host with ALSA headers"
  skip "Linux arm64" "requires Linux host with ALSA headers"
elif ! pkg-config --exists alsa 2>/dev/null; then
  echo ""
  echo "  ⚠  Linux targets skipped: ALSA development headers not found."
  echo "     Install with:  sudo apt install libasound2-dev"
  echo "     or:            sudo dnf install alsa-lib-devel"
  echo "     Then re-run build.sh"
  echo ""
  skip "Linux amd64" "ALSA headers missing"
  skip "Linux arm64" "ALSA headers missing"
else
  # amd64 — native on x86_64 host, cross via zig on arm64 host
  printf "  %-36s" "Linux  amd64 (CGO, dynamic ALSA)"
  if [[ "$(uname -m)" == "x86_64" ]]; then
    GOOS=linux GOARCH=amd64 CGO_ENABLED=1 \
      go build -trimpath -ldflags="$LDFLAGS" \
      -o "$BIN/${APP}-linux-amd64" "$PKG"
  else
    W=$(zig_cc x86_64-linux-gnu)
    GOOS=linux GOARCH=amd64 CGO_ENABLED=1 CC="$W" \
      go build -trimpath -ldflags="$LDFLAGS" \
      -o "$BIN/${APP}-linux-amd64" "$PKG"
    rm -f "$W"
  fi
  ok "$BIN/${APP}-linux-amd64"

  # arm64 — native on aarch64 host, cross via zig on x86_64 host
  printf "  %-36s" "Linux  arm64 (CGO, dynamic ALSA)"
  if [[ "$(uname -m)" == "aarch64" ]]; then
    GOOS=linux GOARCH=arm64 CGO_ENABLED=1 \
      go build -trimpath -ldflags="$LDFLAGS" \
      -o "$BIN/${APP}-linux-arm64" "$PKG"
  else
    W=$(zig_cc aarch64-linux-gnu)
    GOOS=linux GOARCH=arm64 CGO_ENABLED=1 CC="$W" \
      go build -trimpath -ldflags="$LDFLAGS" \
      -o "$BIN/${APP}-linux-arm64" "$PKG"
    rm -f "$W"
  fi
  ok "$BIN/${APP}-linux-arm64"
fi

# ── Windows amd64 (zig MinGW-w64, no ALSA) ────────────────────────────────────
if [[ "$SKIP_WINDOWS" == "1" ]]; then
  skip "Windows amd64" "SKIP_WINDOWS=1"
else
  printf "  %-36s" "Windows amd64 (zig, MinGW-w64)"
  W=$(zig_cc x86_64-windows-gnu)
  GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC="$W" \
    go build -trimpath -ldflags="$LDFLAGS" \
    -o "$BIN/${APP}-windows-amd64.exe" "$PKG"
  rm -f "$W"
  ok "$BIN/${APP}-windows-amd64.exe"
fi

echo "──────────────────────────────────────────"
echo "All binaries in ./$BIN/"
ls -lh "$BIN"/ 2>/dev/null | awk 'NR>1 {printf "  %6s  %s\n", $5, $9}'
