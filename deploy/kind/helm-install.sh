#!/usr/bin/env bash
# helm upgrade --install into the local Kind cluster. Safe to re-run.
# Extra arguments are passed through to helm (e.g. --set, -f).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

CHART="${HELM_CHART:-${ROOT}/deploy/helm/foreverbull}"
RELEASE="${HELM_RELEASE:-foreverbull}"
NAMESPACE="${HELM_NAMESPACE:-foreverbull}"
VERSION="${VERSION:-local}"
HOST_PORT="${HOST_PORT:-true}"
CLUSTER_NAME="${KIND_CLUSTER:-foreverbull}"
KUBE_CONTEXT="${KUBE_CONTEXT:-kind-${CLUSTER_NAME}}"

if ! command -v helm >/dev/null 2>&1; then
  echo "helm is not installed. https://helm.sh/docs/intro/install/" >&2
  exit 1
fi

if [[ ! -d "${CHART}" ]]; then
  echo "Helm chart not found: ${CHART}" >&2
  exit 1
fi

echo "helm upgrade --install ${RELEASE} (namespace=${NAMESPACE}, imageTag=${VERSION}, hostPort=${HOST_PORT}, context=${KUBE_CONTEXT})"
helm upgrade --install "${RELEASE}" "${CHART}" \
  --namespace "${NAMESPACE}" \
  --create-namespace \
  --kube-context "${KUBE_CONTEXT}" \
  --set "expose.hostPort=${HOST_PORT}" \
  --set "global.imageTag=${VERSION}" \
  "$@"
