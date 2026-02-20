#!/usr/bin/env bash
set -euo pipefail

check_pkg() {
  local pkg="$1"
  local min="$2"
  local out cov

  out=$(go test "$pkg" -cover)
  echo "$out"
  cov=$(echo "$out" | sed -nE 's/.*coverage: ([0-9.]+)%.*/\1/p' | tail -n1)
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

check_pkg ./internal/cli 30
check_pkg ./internal/output 80

echo "[coverage-check] ok"
