#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

# An alternate modfile keeps both successful tidy runs and failures read-only
# for the checkout, including on Go 1.22 (which has no tidy -diff).
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
cp go.mod "$work/check.mod"
if [[ -f go.sum ]]; then
  cp go.sum "$work/check.sum"
fi
go mod tidy -modfile="$work/check.mod"

status=0
diff -u go.mod "$work/check.mod" || status=1
before_sum=go.sum
after_sum="$work/check.sum"
[[ -f "$before_sum" ]] || before_sum=/dev/null
[[ -f "$after_sum" ]] || after_sum=/dev/null
diff -u "$before_sum" "$after_sum" || status=1
if [[ "$status" -ne 0 ]]; then
  echo "[mod-check] go.mod/go.sum drift detected; run go mod tidy" >&2
  exit 1
fi
echo "[mod-check] ok"
