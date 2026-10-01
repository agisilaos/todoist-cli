#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
source ./scripts/release-config.sh
export GOTOOLCHAIN="$RELEASE_GO_TOOLCHAIN"
source ./scripts/cli-shared/release-core.sh
cli_release_preflight "$@"
# Keep candidate CI portable on stock runners, preserving the existing guard.
if grep -R -nE '(^|[[:space:]])(r[g]|j[q]|y[q]|f[d])([[:space:]]|$)' scripts >/dev/null; then
  cli_release_die "scripts/ uses non-portable tooling (rg/jq/yq/fd). Use grep/sed/awk or install tools explicitly in workflow."
fi
make verify
cli_release_build_check "$version"
