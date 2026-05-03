#!/usr/bin/env bash
set -u

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT" || exit 1

passed=()
failed=()

run_gate() {
  local name="$1"
  shift

  printf '\n==> %s\n' "$name"
  if "$@"; then
    passed+=("$name")
    printf 'PASS %s\n' "$name"
  else
    failed+=("$name")
    printf 'FAIL %s\n' "$name"
    return 1
  fi
}

status=0

run_gate "Desktop dependencies" npm --prefix apps/desktop install || status=1
run_gate "Playwright browser install" npm --prefix apps/desktop exec playwright install chromium || status=1
run_gate "Go daemon contracts" go test ./services/daemon/... || status=1
run_gate "Desktop Vitest integration" npm --prefix apps/desktop run test || status=1
run_gate "Desktop Playwright flow" npm --prefix apps/desktop run test:e2e || status=1
run_gate "Desktop lint" npm --prefix apps/desktop run lint || status=1
run_gate "Desktop build" npm --prefix apps/desktop run build || status=1

printf '\nPhase 3 QA smoke summary\n'
printf 'PASS: %d\n' "${#passed[@]}"
for name in "${passed[@]}"; do
  printf '  - %s\n' "$name"
done

printf 'FAIL: %d\n' "${#failed[@]}"
if [ "${#failed[@]}" -gt 0 ]; then
  for name in "${failed[@]}"; do
    printf '  - %s\n' "$name"
  done
fi

if [ "$status" -eq 0 ]; then
  printf '\nRESULT: PASS\n'
else
  printf '\nRESULT: FAIL\n'
fi

exit "$status"
