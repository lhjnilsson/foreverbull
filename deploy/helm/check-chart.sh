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
# An installed release from before httpPort has no value. reuse-values must still render 8080.
helm template "${RELEASE}" "${CHART}" --namespace "${NAMESPACE}" --set expose.hostPort=true --set broker.httpPort=null >> "${rendered}"

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

broker_manifest="$(mktemp)"
trap 'rm -f "${rendered}" "${broker_manifest}"' EXIT
awk '
  function flush() {
    if (doc ~ /(^|\n)kind: Deployment(\n|$)/ && doc ~ /(^|\n)  name: foreverbull(\n|$)/) {
      printf "%s", doc
    }
  }
  $0 == "---" { flush(); doc = ""; next }
  { doc = doc $0 "\n" }
  END { flush() }
' "${rendered}" > "${broker_manifest}"

if [[ ! -s "${broker_manifest}" ]]; then
  echo "rendered chart is missing the broker Deployment" >&2
  exit 1
fi

for forbidden in \
  ALPACA_MARKETS_API_KEY \
  SAXO_ACCESS_TOKEN \
  IG_PASSWORD \
  IBKR_ACCESS_TOKEN \
  api.alpaca.markets
do
  if grep -F "${forbidden}" "${broker_manifest}"; then
    echo "broker manifest must not contain ${forbidden}" >&2
    exit 1
  fi
done

# Credentials come from Secret foreverbull-credentials, never a plain env value.
for env_name in POSTGRES_PASSWORD MINIO_ROOT_PASSWORD MINIO_SECRET; do
  if grep -A1 -E "^ +- name: ${env_name}$" "${rendered}" | grep -qE '^ +value:'; then
    echo "${env_name} must come from a secretKeyRef, not a plain value" >&2
    exit 1
  fi
done

# Without hostPort, the public default credentials must stop the install.
for default_value in postgres.password=foreverbull minio.secretKey=minioadmin; do
  if helm template "${RELEASE}" "${CHART}" --namespace "${NAMESPACE}" --set "${default_value}" >/dev/null 2>&1; then
    echo "chart must refuse ${default_value} without expose.hostPort" >&2
    exit 1
  fi
done
helm template "${RELEASE}" "${CHART}" --namespace "${NAMESPACE}" \
  --set postgres.password=foreverbull --set minio.secretKey=minioadmin --set credentials.allowDefaults=true >/dev/null

echo "helm chart checks passed"
