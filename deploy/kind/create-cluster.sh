#!/usr/bin/env bash
# Create the local Kind cluster. Safe to re-run: an existing cluster is left as-is.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLUSTER_NAME="${KIND_CLUSTER:-foreverbull}"
CLUSTER_CONFIG="${KIND_CONFIG:-${SCRIPT_DIR}/cluster.yaml}"

if ! command -v kind >/dev/null 2>&1; then
  echo "kind is not installed. https://kind.sigs.k8s.io/docs/user/quick-start/" >&2
  exit 1
fi

if [[ ! -f "${CLUSTER_CONFIG}" ]]; then
  echo "Kind config not found: ${CLUSTER_CONFIG}" >&2
  exit 1
fi

if kind get clusters 2>/dev/null | grep -qx "${CLUSTER_NAME}"; then
  echo "Kind cluster '${CLUSTER_NAME}' already exists (extraPortMappings are fixed at create time)."
  exit 0
fi

echo "Creating Kind cluster '${CLUSTER_NAME}' from ${CLUSTER_CONFIG}"
kind create cluster --name "${CLUSTER_NAME}" --config "${CLUSTER_CONFIG}"
