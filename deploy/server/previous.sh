#!/usr/bin/env bash
# Print FOREVERBULL_DIGEST / ZIPLINE_DIGEST / GRAFANA_DIGEST for the Helm
# revision before the one currently deployed. Stdout is only those three
# assignments. Does not build and does not change the release.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=deploy/server/lib.sh
source "${SCRIPT_DIR}/lib.sh"

RELEASE="${HELM_RELEASE:-foreverbull}"
NAMESPACE="${NAMESPACE:-}"
IMAGE_REPOSITORY="${IMAGE_REPOSITORY:-}"

require_release_namespace "${NAMESPACE}" || die "namespace must be fb-nightly or e2e-<sha>"
require_helm_release "${RELEASE}" || die "unexpected Helm release name"
require_ghcr_repo "${IMAGE_REPOSITORY}" || die "IMAGE_REPOSITORY must be ghcr.io/<owner>"
command -v helm >/dev/null 2>&1 || die "helm is not installed"
command -v jq >/dev/null 2>&1 || die "jq is not installed"

history="$(helm history "${RELEASE}" --namespace "${NAMESPACE}" --output json)"
if ! rev="$(printf '%s' "${history}" | select_previous_revision)"; then
  die "no previous Helm revision to restore in ${NAMESPACE}"
fi
[[ "${rev}" =~ ^[0-9]+$ ]] || die "unexpected revision ${rev}"

manifest="$(helm get manifest "${RELEASE}" --namespace "${NAMESPACE}" --revision "${rev}")"
declare -A digests=()
while read -r dep ref; do
  [[ -n "${dep}" ]] || continue
  suffix="$(image_suffix_for_deploy "${dep}")" || die "unexpected deployment ${dep}"
  if [[ "${ref}" != "${IMAGE_REPOSITORY}/${suffix}@"* && "${ref}" != "${IMAGE_REPOSITORY}/${suffix}:"* ]]; then
    die "previous ${dep} image is outside ${IMAGE_REPOSITORY}"
  fi
  if ! digest="$(digest_from_ref "${ref}")"; then
    command -v docker >/dev/null 2>&1 || die "docker is required to resolve ${ref}"
    digest="$(docker buildx imagetools inspect --format '{{.Manifest.Digest}}' "${ref}")"
  fi
  require_digest "${digest}" || die "previous ${dep} image did not resolve to a digest"
  digests["${dep}"]="${digest}"
done < <(printf '%s\n' "${manifest}" | parse_release_images)

for dep in foreverbull zipline grafana; do
  [[ -n "${digests[${dep}]:-}" ]] || die "revision ${rev} is missing ${dep}"
done

printf 'FOREVERBULL_DIGEST=%s\n' "${digests[foreverbull]}"
printf 'ZIPLINE_DIGEST=%s\n' "${digests[zipline]}"
printf 'GRAFANA_DIGEST=%s\n' "${digests[grafana]}"
