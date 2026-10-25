#!/usr/bin/env bash
# Move GHCR :latest and the fb-nightly Helm release onto the same digests.
# Helm is applied first. latest is moved only after that install is healthy.
# If either step fails, latest is pointed back at the digests it had before
# this run and the Helm release is rolled back to the revision recorded first.
# A missing previous latest tag is left in place: deleting it would delete the
# shared sha-* manifest.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=deploy/server/lib.sh
source "${SCRIPT_DIR}/lib.sh"

RELEASE="${HELM_RELEASE:-foreverbull}"
NAMESPACE="${NAMESPACE:-}"
IMAGE_REPOSITORY="${IMAGE_REPOSITORY:-}"
WAIT_TIMEOUT="${WAIT_TIMEOUT:-20m}"

require_namespace "${NAMESPACE}" || die "unexpected namespace"
if [[ "${NAMESPACE}" != "fb-nightly" ]]; then
  die "refusing to move latest for namespace ${NAMESPACE}; only fb-nightly is served"
fi
require_helm_release "${RELEASE}" || die "unexpected Helm release name"
require_ghcr_repo "${IMAGE_REPOSITORY}" || die "IMAGE_REPOSITORY must be ghcr.io/<owner>"
require_digest "${FOREVERBULL_DIGEST:-}" || die "FOREVERBULL_DIGEST must be sha256:<64 hex>"
require_digest "${ZIPLINE_DIGEST:-}" || die "ZIPLINE_DIGEST must be sha256:<64 hex>"
require_digest "${GRAFANA_DIGEST:-}" || die "GRAFANA_DIGEST must be sha256:<64 hex>"
if [[ -z "${GRAFANA_URL:-}" ]]; then
  GRAFANA_URL="https://nightly.foreverbull.com"
  export GRAFANA_URL
fi
require_grafana_url "${GRAFANA_URL}" || die "GRAFANA_URL must be an https origin"
command -v helm >/dev/null 2>&1 || die "helm is not installed"
command -v docker >/dev/null 2>&1 || die "docker is not installed"

state="$(mktemp -d)"
printf '0\n' >"${state}/helm-started"
for name in foreverbull zipline fb-grafana; do
  printf '0\n' >"${state}/moved-${name}"
  printf 'missing\n' >"${state}/previous-${name}"
done

capture_latest() {
  local name="$1"
  local image="${IMAGE_REPOSITORY}/${name}"
  local digest err
  if digest="$(docker buildx imagetools inspect --format '{{.Manifest.Digest}}' "${image}:latest" 2>"${state}/inspect-err")" \
    && require_digest "${digest}"; then
    printf '%s\n' "${digest}" >"${state}/previous-${name}"
    return 0
  fi
  err="$(cat "${state}/inspect-err" 2>/dev/null || true)"
  if grep -qiE 'not found|manifest unknown|no such manifest|name unknown' <<<"${err}"; then
    printf 'missing\n' >"${state}/previous-${name}"
    return 0
  fi
  die "could not read ${image}:latest"
}

record_revision() {
  local err rev
  if err="$(helm status "${RELEASE}" --namespace "${NAMESPACE}" 2>&1)"; then
    rev="$(helm history "${RELEASE}" --namespace "${NAMESPACE}" --output json | jq -r 'map(select(.status == "deployed")) | if length == 0 then "" else max_by(.revision).revision end')"
    if [[ -z "${rev}" ]]; then
      die "release ${RELEASE} in ${NAMESPACE} has no deployed revision; refusing to upgrade"
    fi
    printf '%s\n' "${rev}" >"${state}/revision"
    return 0
  fi
  if grep -qi 'not found' <<<"${err}"; then
    printf '\n' >"${state}/revision"
    return 0
  fi
  die "could not read Helm release ${RELEASE} in ${NAMESPACE}"
}

restore_tags() {
  local name previous image
  for name in foreverbull zipline fb-grafana; do
    [[ "$(cat "${state}/moved-${name}")" == "1" ]] || continue
    previous="$(cat "${state}/previous-${name}")"
    image="${IMAGE_REPOSITORY}/${name}"
    if [[ "${previous}" == "missing" ]]; then
      echo "no previous latest for ${image}; leaving the tag so the sha-* manifest is not deleted" >&2
      continue
    fi
    "${SCRIPT_DIR}/ghcr.sh" move-latest "${image}" "${previous}" || echo "could not restore latest for ${image}" >&2
  done
}

restore_helm() {
  local rev latest
  [[ "$(cat "${state}/helm-started")" == "1" ]] || return 0
  rev="$(head -n 1 "${state}/revision" || true)"
  if [[ -z "${rev}" ]]; then
    helm uninstall "${RELEASE}" --namespace "${NAMESPACE}" --wait --timeout "${WAIT_TIMEOUT}" || true
    return 0
  fi
  latest="$(helm history "${RELEASE}" --namespace "${NAMESPACE}" --output json | jq -r 'max_by(.revision).revision')"
  if [[ "${latest}" == "${rev}" ]]; then
    echo "Helm release stayed at revision ${rev}" >&2
    return 0
  fi
  helm rollback "${RELEASE}" "${rev}" --namespace "${NAMESPACE}" --wait --timeout "${WAIT_TIMEOUT}" || echo "could not roll back Helm revision ${rev}" >&2
}

finish() {
  local status=$?
  trap - EXIT
  set +e
  if [[ "${status}" -ne 0 ]]; then
    echo "nightly retarget failed; restoring the previous digest" >&2
    restore_tags
    restore_helm
  fi
  rm -rf "${state}"
  exit "${status}"
}
trap finish EXIT

capture_latest foreverbull
capture_latest zipline
capture_latest fb-grafana
record_revision

printf '1\n' >"${state}/helm-started"
NAMESPACE="${NAMESPACE}" \
  HELM_RELEASE="${RELEASE}" \
  IMAGE_REPOSITORY="${IMAGE_REPOSITORY}" \
  FOREVERBULL_DIGEST="${FOREVERBULL_DIGEST}" \
  ZIPLINE_DIGEST="${ZIPLINE_DIGEST}" \
  GRAFANA_DIGEST="${GRAFANA_DIGEST}" \
  GRAFANA_URL="${GRAFANA_URL:-}" \
  WAIT_TIMEOUT="${WAIT_TIMEOUT}" \
  "${SCRIPT_DIR}/install.sh"

move_one() {
  local name="$1"
  local digest="$2"
  # Mark first so a failed move of a tag that already moved is restored.
  printf '1\n' >"${state}/moved-${name}"
  "${SCRIPT_DIR}/ghcr.sh" move-latest "${IMAGE_REPOSITORY}/${name}" "${digest}"
}

move_one foreverbull "${FOREVERBULL_DIGEST}"
move_one zipline "${ZIPLINE_DIGEST}"
move_one fb-grafana "${GRAFANA_DIGEST}"

echo "fb-nightly and latest now serve the same digests"
