#!/usr/bin/env bash
# Install the platform chart and wait until the six workloads are ready.
# Does not create a Kind cluster (CI uses helm/kind-action; local: task kind:create).
# Does not enable trading. Does not need Alpaca secrets.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

CHART="${HELM_CHART:-${ROOT}/deploy/helm/foreverbull}"
RELEASE="${HELM_RELEASE:-foreverbull}"
NAMESPACE="${HELM_NAMESPACE:-foreverbull}"
VERSION="${VERSION:-latest}"
WAIT_TIMEOUT="${WAIT_TIMEOUT:-15m}"

WORKLOADS=(postgres nats minio zipline foreverbull grafana)

if ! command -v helm >/dev/null 2>&1; then
  echo "helm is not installed. https://helm.sh/docs/intro/install/" >&2
  exit 1
fi
if ! command -v kubectl >/dev/null 2>&1; then
  echo "kubectl is not installed. https://kubernetes.io/docs/tasks/tools/" >&2
  exit 1
fi
if [[ ! -d "${CHART}" ]]; then
  echo "Helm chart not found: ${CHART}" >&2
  exit 1
fi

dump_cluster() {
  echo "Kind/Helm e2e failed; dumping ${NAMESPACE} state" >&2
  kubectl get pods,deploy,svc,pvc -n "${NAMESPACE}" -o wide || true
  for name in "${WORKLOADS[@]}"; do
    kubectl describe -n "${NAMESPACE}" "deployment/${name}" || true
    kubectl logs -n "${NAMESPACE}" "deploy/${name}" --tail=80 || true
  done
}
trap dump_cluster ERR

echo "helm upgrade --install ${RELEASE} (namespace=${NAMESPACE}, imageTag=${VERSION})"
helm upgrade --install "${RELEASE}" "${CHART}" \
  --namespace "${NAMESPACE}" \
  --create-namespace \
  --wait \
  --timeout "${WAIT_TIMEOUT}" \
  --set "global.imageTag=${VERSION}" \
  --set "expose.hostPort=false" \
  --set "broker.trading.enabled=false"

deploys=()
for name in "${WORKLOADS[@]}"; do
  deploys+=("deployment/${name}")
done
echo "kubectl wait --for=condition=available (${deploys[*]})"
kubectl wait \
  --namespace "${NAMESPACE}" \
  --for=condition=available \
  --timeout="${WAIT_TIMEOUT}" \
  "${deploys[@]}"

# Broker image ships grpc_health_probe; SERVING means Postgres, NATS, and Minio responded.
echo "broker gRPC health"
kubectl exec --namespace "${NAMESPACE}" deploy/foreverbull -- \
  grpc_health_probe -addr=:50055

echo "Kind/Helm e2e: six workloads ready, broker gRPC health SERVING"
