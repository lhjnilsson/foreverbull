#!/usr/bin/env bash
# Point latest and fb-nightly at an existing digest. Does not build.
# Empty digest inputs use the previous Helm revision. A partial set is refused.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=deploy/server/lib.sh
source "${SCRIPT_DIR}/lib.sh"

mode="$(classify_rollback_inputs)" || die "rollback digests must be all set or all empty"
if [[ "${mode}" == "previous" ]]; then
  lines_file="$(mktemp)"
  "${SCRIPT_DIR}/previous.sh" >"${lines_file}"
  mapfile -t lines <"${lines_file}"
  rm -f "${lines_file}"
  if [[ "${#lines[@]}" -ne 3 ]]; then
    die "expected three digests from the previous revision"
  fi
  for line in "${lines[@]}"; do
    if [[ ! "${line}" =~ ^(FOREVERBULL_DIGEST|ZIPLINE_DIGEST|GRAFANA_DIGEST)=sha256:[0-9a-f]{64}$ ]]; then
      die "refusing an unexpected previous digest"
    fi
  done
  for line in "${lines[@]}"; do
    key="${line%%=*}"
    val="${line#*=}"
    printf -v "${key}" '%s' "${val}"
    export "${key?}"
  done
fi

exec "${SCRIPT_DIR}/retarget.sh"
