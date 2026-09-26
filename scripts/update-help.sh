#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_DIR="${ROOT_DIR}/docs/help"
WORK_DIR="$(mktemp -d)"
trap 'rm -rf "${WORK_DIR}"' EXIT
BIN_PATH="${WORK_DIR}/todoist-help"
SNAPSHOT_LIST="${ROOT_DIR}/scripts/help-snapshots.txt"

usage() {
  cat <<USAGE
Usage: $(basename "$0") [--out-dir <path>]
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --out-dir)
      [[ -n "${2:-}" ]] || { echo "error: --out-dir requires a path" >&2; usage >&2; exit 2; }
      OUT_DIR="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "error: unknown arg: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

cd "${ROOT_DIR}"
mkdir -p "${OUT_DIR}"
go build -o "${BIN_PATH}" ./cmd/todoist

while IFS=$'\t' read -r out_file cmdline || [[ -n "${out_file:-}" ]]; do
  [[ -z "${out_file:-}" || "${out_file}" =~ ^# ]] && continue
  read -r -a cmd_args <<< "${cmdline}"
  # A temporary config and cwd keep help independent of local credentials/config.
  (cd "${WORK_DIR}" && env -u TODOIST_TOKEN -u TODOIST_PROFILE \
    -u TODOIST_TIMEOUT -u TODOIST_BASE_URL -u TODOIST_FUZZY \
    -u TODOIST_ACCESSIBLE -u TODOIST_TABLE_WIDTH -u TODOIST_PLANNER_CMD \
    "${BIN_PATH}" --config "${WORK_DIR}/config.json" "${cmd_args[@]}") \
    > "${OUT_DIR}/${out_file}" 2>&1
done < "${SNAPSHOT_LIST}"

echo "Updated help snapshots in ${OUT_DIR}"
