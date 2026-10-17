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

# A set password is expanded raw into POSTGRES_URL/DATABASE_URL, so a
# value that is not URL-safe must stop the install with this message.
for case in 'postgres.password|postgres.password=p@ss/word' 'minio.secretKey|minio.secretKey=p@ss:word'; do
  key="${case%%|*}"
  setting="${case#*|}"
  if output="$(helm template "${RELEASE}" "${CHART}" --namespace "${NAMESPACE}" --set "${setting}" 2>&1)"; then
    echo "chart must refuse non-URL-safe ${key}" >&2
    exit 1
  fi
  if ! grep -Fq "${key} must be URL-safe (letters, digits and -._~ are safe)" <<< "${output}"; then
    echo "chart must explain ${key} must be URL-safe, got: ${output}" >&2
    exit 1
  fi
done
# URL-safe set values render.
helm template "${RELEASE}" "${CHART}" --namespace "${NAMESPACE}" \
  --set 'postgres.password=S3cure.pass_word-1~x' --set 'minio.secretKey=S3cure.pass_word-1~x' >/dev/null

# Generated credentials must stay stable across helm upgrade. The
# credential helper looks up the existing Secret in the release
# namespace and reuses its data before falling back to randAlphaNum.
# lookup only talks to a live cluster, so offline template renders
# cannot exercise it directly: guard the lookup pattern here, and
# simulate the upgrade's second pass by feeding the first render's
# generated values back as set values (what lookup returns live).
helpers="${CHART}/templates/_helpers.tpl"
if ! grep -Fq 'lookup "v1" "Secret" $root.Release.Namespace' "${helpers}" \
  || ! grep -Fq 'b64dec' "${helpers}" \
  || ! grep -Fq 'randAlphaNum 32' "${helpers}"; then
  echo "credential helper must reuse the existing Secret (lookup) before generating" >&2
  exit 1
fi
first_render="$(mktemp)"
second_render="$(mktemp)"
trap 'rm -f "${rendered}" "${broker_manifest}" "${first_render}" "${second_render}"' EXIT
helm template "${RELEASE}" "${CHART}" --namespace "${NAMESPACE}" > "${first_render}"
secret_value() {
  grep -E "^  $1: " "$2" | head -n 1 | sed -E 's/^[^:]+: "?(.*)"?$/\1/' | tr -d '"'
}
generated_pg="$(secret_value postgres-password "${first_render}")"
generated_minio="$(secret_value minio-secretKey "${first_render}")"
if [[ -z "${generated_pg}" || -z "${generated_minio}" ]]; then
  echo "first render must generate postgres and MinIO credentials" >&2
  exit 1
fi
generated_pg_clear="$(printf '%s' "${generated_pg}" | base64 -d)"
generated_minio_clear="$(printf '%s' "${generated_minio}" | base64 -d)"
if [[ ! "${generated_pg_clear}" =~ ^[-A-Za-z0-9._~]+$ || ! "${generated_minio_clear}" =~ ^[-A-Za-z0-9._~]+$ ]]; then
  echo "generated credentials must be URL-safe" >&2
  exit 1
fi
helm template "${RELEASE}" "${CHART}" --namespace "${NAMESPACE}" \
  --set "postgres.password=${generated_pg_clear}" --set "minio.secretKey=${generated_minio_clear}" > "${second_render}"
if [[ "$(secret_value postgres-password "${second_render}")" != "${generated_pg}" \
  || "$(secret_value minio-secretKey "${second_render}")" != "${generated_minio}" ]]; then
  echo "credentials fed back on upgrade must render the same Secret" >&2
  exit 1
fi

echo "helm chart checks passed"
