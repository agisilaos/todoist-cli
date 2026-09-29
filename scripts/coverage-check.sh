#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

output="$(mktemp)"
trap 'rm -f "$output"' EXIT
# pipefail preserves test failures while tee keeps their diagnostics visible.
go test ./... -cover | tee "$output"

check_pkg() {
  local pkg="$1"
  local min="$2"
  local cov

  cov=$(awk -v pkg="$pkg" '$1 == "ok" && $2 == pkg { print }' "$output" |
    sed -nE 's/.*coverage: ([0-9.]+)%.*/\1/p')
  if [[ -z "$cov" ]]; then
    echo "[coverage-check] failed to parse coverage for $pkg" >&2
    return 1
  fi
  if ! awk -v c="$cov" -v m="$min" 'BEGIN { exit (c+0 >= m+0) ? 0 : 1 }'; then
    echo "[coverage-check] $pkg coverage ${cov}% is below required ${min}%" >&2
    return 1
  fi
  echo "[coverage-check] $pkg coverage ${cov}% (required ${min}%)"
}

check_pkg github.com/agisilaos/todoist-cli/internal/cli 30
check_pkg github.com/agisilaos/todoist-cli/internal/output 80

echo "[coverage-check] ok"
