#!/usr/bin/env bash
# start-docling.sh — launch docling-serve from the Seshat-managed Python venv.
#
# Usage:
#   ./scripts/start-docling.sh
#   DOCLING_PORT=5002 ./scripts/start-docling.sh
#   SESHAT_RUNTIME_ROOT=/custom/path ./scripts/start-docling.sh
#
# Environment variables:
#   SESHAT_RUNTIME_ROOT   Config/data root (default: ~/.config/seshat-cli)
#   DOCLING_HOST          Bind address (default: 127.0.0.1)
#   DOCLING_PORT          HTTP port (default: 5001)
#   DOCLING_WORKERS       Parallel conversion workers (default: 1)
#
# Seshat auto-starts docling-serve at launch when the venv is installed.
# Use this script only if you want to run it manually (e.g. as a daemon).

set -euo pipefail

_default_runtime_root() {
    if [ -n "${XDG_CONFIG_HOME:-}" ]; then
        echo "$XDG_CONFIG_HOME/seshat-cli"
    else
        echo "$HOME/.config/seshat-cli"
    fi
}

SESHAT_RUNTIME_ROOT="${SESHAT_RUNTIME_ROOT:-$(_default_runtime_root)}"
VENV_DIR="$SESHAT_RUNTIME_ROOT/.venv"

# `uv venv` produces a Windows-layout venv (Scripts\docling-serve.exe) when
# uv.exe is the native Windows binary - true even under Git Bash, where
# `uname`-based OS checks don't reliably catch it. Probe the venv's own
# layout instead of guessing from the OS. Called both before and after the
# install step below, since the venv may not exist yet on the first probe.
resolve_docling_bin() {
    if [ -f "$VENV_DIR/Scripts/docling-serve.exe" ]; then
        DOCLING_BIN="$VENV_DIR/Scripts/docling-serve.exe"
    else
        DOCLING_BIN="$VENV_DIR/bin/docling-serve"
    fi
}
resolve_docling_bin

PORT="${DOCLING_PORT:-5001}"
HOST="${DOCLING_HOST:-127.0.0.1}"
WORKERS="${DOCLING_WORKERS:-1}"

# ── Ensure venv + docling-serve are installed ─────────────────────────────────
if [ ! -f "$DOCLING_BIN" ]; then
    echo "[docling] docling-serve not found. Running: ./scripts/install-python-env.sh"
    "$(dirname "$0")/install-python-env.sh"
    resolve_docling_bin
fi

# ── Launch ────────────────────────────────────────────────────────────────────
echo "[docling] Starting docling-serve on ${HOST}:${PORT} (workers: ${WORKERS})"
echo "[docling] Logs: $SESHAT_RUNTIME_ROOT/logs/docling.log"
echo ""

exec "$DOCLING_BIN" run \
  --host "${HOST}" \
  --port "${PORT}" \
  --workers "${WORKERS}"
