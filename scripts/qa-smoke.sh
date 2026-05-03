#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DESKTOP_DIR="$ROOT_DIR/apps/desktop"

passed=()

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing prerequisite: $1 is required for ./scripts/qa-smoke.sh" >&2
    exit 127
  fi
}

run_step() {
  local name="$1"
  shift
  echo "==> $name"
  "$@"
  passed+=("$name")
}

need_cmd go
need_cmd npm
need_cmd node

if [[ ! -d "$DESKTOP_DIR/node_modules" ]]; then
  echo "missing prerequisite: apps/desktop/node_modules not found; run 'npm install' in apps/desktop" >&2
  exit 1
fi

if [[ -n "${ZIN_HOME:-}" && "$ZIN_HOME" == "$HOME/.zin"* ]]; then
  echo "refusing to run with ZIN_HOME inside ~/.zin; unset it or point it at a temp directory" >&2
  exit 1
fi

export ZIN_HOME="${ZIN_HOME:-$(mktemp -d "${TMPDIR:-/tmp}/zin-qa-smoke.XXXXXX")}"
trap 'rm -rf "$ZIN_HOME"' EXIT

run_step "daemon contract tests" go test ./services/daemon/...
run_step "desktop unit/integration tests" npm --prefix "$DESKTOP_DIR" run test
run_step "desktop lint" npm --prefix "$DESKTOP_DIR" run lint
run_step "desktop build" npm --prefix "$DESKTOP_DIR" run build
run_step "playwright browser install" npm --prefix "$DESKTOP_DIR" exec playwright install chromium
run_step "playwright phase 3 smoke" npm --prefix "$DESKTOP_DIR" run test:e2e -- --project=chromium

echo
echo "qa-smoke passed:"
for step in "${passed[@]}"; do
  echo " - $step"
done
