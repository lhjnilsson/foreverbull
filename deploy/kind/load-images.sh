#!/usr/bin/env bash
# Load ghcr.io/lhjnilsson/foreverbull, zipline, and fb-grafana into Kind. Safe to re-run.
# Tag is VERSION (default: local). Cluster name is KIND_CLUSTER (default: foreverbull).
set -euo pipefail

CLUSTER_NAME="${KIND_CLUSTER:-foreverbull}"
VERSION="${VERSION:-local}"

IMAGES=(
  "ghcr.io/lhjnilsson/foreverbull:${VERSION}"
  "ghcr.io/lhjnilsson/zipline:${VERSION}"
  "ghcr.io/lhjnilsson/fb-grafana:${VERSION}"
)

if ! command -v kind >/dev/null 2>&1; then
  echo "kind is not installed. https://kind.sigs.k8s.io/docs/user/quick-start/" >&2
  exit 1
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "docker is not installed." >&2
  exit 1
fi

if ! kind get clusters 2>/dev/null | grep -qx "${CLUSTER_NAME}"; then
  echo "Kind cluster '${CLUSTER_NAME}' not found. Create it first:" >&2
  echo "  kind create cluster --name ${CLUSTER_NAME} --config deploy/kind/cluster.yaml" >&2
  echo "  # or: task kind:create" >&2
  exit 1
fi

missing=()
for img in "${IMAGES[@]}"; do
  if ! docker image inspect "${img}" >/dev/null 2>&1; then
    missing+=("${img}")
  fi
done

if ((${#missing[@]} > 0)); then
  if [[ "${VERSION}" == "local" ]]; then
    echo "Missing local images:" >&2
    for img in "${missing[@]}"; do
      echo "  ${img}" >&2
    done
    echo "Build them first:" >&2
    echo "  task build:docker            # ghcr.io/lhjnilsson/foreverbull:local" >&2
    echo "  task client:docker:zipline   # ghcr.io/lhjnilsson/zipline:local" >&2
    echo "  task grafana:build:docker    # ghcr.io/lhjnilsson/fb-grafana:local" >&2
    exit 1
  fi
  echo "Pulling missing images for VERSION=${VERSION}:"
  for img in "${missing[@]}"; do
    echo "  ${img}"
    docker pull "${img}"
  done
fi

echo "Loading images into Kind cluster '${CLUSTER_NAME}' (tag=${VERSION}):"
for img in "${IMAGES[@]}"; do
  echo "  ${img}"
done

kind load docker-image "${IMAGES[@]}" --name "${CLUSTER_NAME}"
echo "Loaded ${#IMAGES[@]} images into '${CLUSTER_NAME}'."
