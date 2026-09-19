#!/usr/bin/env bash
# Lint and template the platform chart. Does not talk to a cluster.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHART="${HELM_CHART:-${SCRIPT_DIR}/foreverbull}"
RELEASE="${HELM_RELEASE:-foreverbull}"
NAMESPACE="${HELM_NAMESPACE:-foreverbull}"

if ! command -v helm >/dev/null 2>&1; then
  echo "helm is not installed. https://helm.sh/docs/intro/install/" >&2
  exit 1
fi

if [[ ! -f "${CHART}/Chart.yaml" ]]; then
  echo "Helm chart not found: ${CHART}" >&2
  exit 1
fi

helm lint "${CHART}"

rendered="$(mktemp)"
trap 'rm -f "${rendered}"' EXIT
helm template "${RELEASE}" "${CHART}" --namespace "${NAMESPACE}" > "${rendered}"
helm template "${RELEASE}" "${CHART}" --namespace "${NAMESPACE}" --set expose.hostPort=true >> "${rendered}"

if grep -F '/var/run/docker.sock' "${rendered}"; then
  echo "chart must not mount /var/run/docker.sock" >&2
  exit 1
fi

for kind in ServiceAccount Role RoleBinding; do
  if ! grep -q "^kind: ${kind}$" "${rendered}"; then
    echo "rendered chart is missing ${kind}" >&2
    exit 1
  fi
done

if ! grep -q 'name: GRAFANA_URL' "${rendered}"; then
  echo "broker must set GRAFANA_URL" >&2
  exit 1
fi

echo "helm chart checks passed"
