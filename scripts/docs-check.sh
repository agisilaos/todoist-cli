#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

die() {
  echo "error: $*" >&2
  exit 1
}

[[ -f README.md ]] || die "README.md not found"
[[ -f CHANGELOG.md ]] || die "CHANGELOG.md not found"

for file in RELEASING.md scripts/changelog-context.sh scripts/changelog-section.py; do
  [[ -f "$file" ]] || die "$file not found"
done

for target in changelog-context release-check release-check-ci release-dry-run release; do
  grep -qE "^${target}:" Makefile || die "Makefile missing target: $target"
done

echo "[docs-check] validating shared docs contract"
python3 ./scripts/docs-contract-check.py
python3 ./scripts/test_docs_contract.py

echo "[docs-check] checking CLI help snapshots"
./scripts/check-help.sh

echo "[docs-check] checking documented flags and help coverage against source"
go test ./internal/cli -run '^TestDocumentation' -count=1

echo "[docs-check] ok"
