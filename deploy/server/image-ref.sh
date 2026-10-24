#!/usr/bin/env bash
# Print the GHCR prefix and sha tag for this commit. Optional image name
# (foreverbull, zipline, or fb-grafana) adds IMAGE=<prefix>/<name>.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=deploy/server/lib.sh
source "${SCRIPT_DIR}/lib.sh"

owner="$(printf '%s' "${GITHUB_REPOSITORY_OWNER:?}" | tr '[:upper:]' '[:lower:]')"
repo="ghcr.io/${owner}"
require_ghcr_repo "${repo}" || die "refusing repository ${repo}"
full="${GITHUB_SHA:?}"
[[ "${full}" =~ ^[0-9a-f]{40}$ ]] || die "GITHUB_SHA must be 40 hex characters"
short="${full::7}"
require_sha_tag "sha-${short}" || die "refusing tag sha-${short}"

printf 'IMAGE_REPOSITORY=%s\n' "${repo}"
printf 'SHORT_SHA=%s\n' "${short}"
printf 'SHA_TAG=%s\n' "sha-${short}"
if [[ $# -eq 0 ]]; then
  exit 0
fi
if [[ $# -ne 1 ]]; then
  die "usage: image-ref.sh [foreverbull|zipline|fb-grafana]"
fi
case "$1" in
  foreverbull | zipline | fb-grafana) ;;
  *) die "unknown image ${1}" ;;
esac
printf 'IMAGE=%s\n' "${repo}/$1"
