#!/usr/bin/env bash
# Install one release pinned to three image digests, then wait until the broker
# gRPC health probe is SERVING. Same workloads and probe as deploy/kind/e2e.sh.
# Does not move the GHCR latest tag and does not touch any other namespace.
set -eEuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
# shellcheck source=deploy/server/lib.sh
source "${SCRIPT_DIR}/lib.sh"

CHART="${HELM_CHART:-${ROOT}/deploy/helm/foreverbull}"
RELEASE="${HELM_RELEASE:-foreverbull}"
NAMESPACE="${NAMESPACE:-}"
WAIT_TIMEOUT="${WAIT_TIMEOUT:-20m}"
IMAGE_REPOSITORY="${IMAGE_REPOSITORY:-}"

WORKLOADS=(postgres nats minio zipline foreverbull grafana)

dump() {
  echo "server install failed; dumping ${NAMESPACE}" >&2
  kubectl get pods,deploy,svc,pvc -n "${NAMESPACE}" -o wide || true
  local name
  for name in "${WORKLOADS[@]}"; do
    kubectl describe -n "${NAMESPACE}" "deployment/${name}" || true
    kubectl logs -n "${NAMESPACE}" "deploy/${name}" --tail=80 || true
  done
}
trap 'rm -f "${values:-}" "${rendered:-}"; dump; exit 1' ERR

require_release_namespace "${NAMESPACE}" || die "namespace must be fb-nightly or e2e-<sha>"
require_helm_release "${RELEASE}" || die "unexpected Helm release name"
require_wait_timeout "${WAIT_TIMEOUT}" || die "WAIT_TIMEOUT must look like 20m"
require_ghcr_repo "${IMAGE_REPOSITORY}" || die "IMAGE_REPOSITORY must be ghcr.io/<owner>"
require_digest "${FOREVERBULL_DIGEST:-}" || die "FOREVERBULL_DIGEST must be sha256:<64 hex>"
require_digest "${ZIPLINE_DIGEST:-}" || die "ZIPLINE_DIGEST must be sha256:<64 hex>"
require_digest "${GRAFANA_DIGEST:-}" || die "GRAFANA_DIGEST must be sha256:<64 hex>"
if [[ "${NAMESPACE}" == "fb-nightly" ]]; then
  require_grafana_url "${GRAFANA_URL:-}" || die "fb-nightly requires GRAFANA_URL to be an https origin"
elif [[ -n "${GRAFANA_URL:-}" ]]; then
  die "GRAFANA_URL is only set for the fb-nightly release"
fi
if [[ ! -d "${CHART}" ]]; then
  die "Helm chart not found: ${CHART}"
fi
command -v helm >/dev/null 2>&1 || die "helm is not installed"
command -v kubectl >/dev/null 2>&1 || die "kubectl is not installed"

values="$(mktemp)"
rendered="$(mktemp)"
trap 'rm -f "${values}" "${rendered}"; dump; exit 1' ERR
write_pin_values "${values}" || die "refusing to render image pins"
echo "image pins:"
cat "${values}"

helm template "${RELEASE}" "${CHART}" --namespace "${NAMESPACE}" -f "${values}" >"${rendered}"
expect_broker="${IMAGE_REPOSITORY}/foreverbull@${FOREVERBULL_DIGEST}"
expect_zipline="${IMAGE_REPOSITORY}/zipline@${ZIPLINE_DIGEST}"
expect_grafana="${IMAGE_REPOSITORY}/fb-grafana@${GRAFANA_DIGEST}"
for expect in "${expect_broker}" "${expect_zipline}" "${expect_grafana}"; do
  if ! grep -Fq "image: ${expect}" "${rendered}"; then
    die "chart would not pin ${expect}"
  fi
done
for tagged in \
  "${IMAGE_REPOSITORY}/foreverbull:" \
  "${IMAGE_REPOSITORY}/zipline:" \
  "${IMAGE_REPOSITORY}/fb-grafana:"
do
  if grep -Fq "image: ${tagged}" "${rendered}"; then
    die "chart rendered a tag (${tagged}); expected a digest pin"
  fi
done

kubectl --request-timeout=20s cluster-info >/dev/null

echo "helm upgrade --install ${RELEASE} (namespace=${NAMESPACE})"
helm upgrade --install "${RELEASE}" "${CHART}" \
  --namespace "${NAMESPACE}" \
  --create-namespace \
  --wait \
  --timeout "${WAIT_TIMEOUT}" \
  -f "${values}"

deploys=()
for name in "${WORKLOADS[@]}"; do
  deploys+=("deployment/${name}")
done
kubectl wait \
  --namespace "${NAMESPACE}" \
  --for=condition=available \
  --timeout="${WAIT_TIMEOUT}" \
  "${deploys[@]}"

kubectl exec --namespace "${NAMESPACE}" deploy/foreverbull -- \
  grpc_health_probe -addr=:50055

verify_live() {
  local deploy="$1"
  local expect="$2"
  local actual
  actual="$(kubectl get deploy "${deploy}" -n "${NAMESPACE}" -o jsonpath='{.spec.template.spec.containers[0].image}')"
  if [[ "${actual}" != "${expect}" ]]; then
    echo "${deploy} is running ${actual}, want ${expect}" >&2
    return 1
  fi
}
verify_live foreverbull "${expect_broker}"
verify_live zipline "${expect_zipline}"
verify_live grafana "${expect_grafana}"

trap - ERR
rm -f "${values}" "${rendered}"
echo "server install: six workloads ready, broker gRPC health SERVING, digests pinned"
