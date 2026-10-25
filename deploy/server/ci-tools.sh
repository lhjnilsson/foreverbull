#!/usr/bin/env bash
# kubectl pin shared with the Kind workflows. Helm stays on azure/setup-helm.
set -euo pipefail

KUBECTL_VERSION="${KUBECTL_VERSION:-v1.35.0}"

if command -v kubectl >/dev/null 2>&1; then
  kubectl version --client
  exit 0
fi

curl -fsSL -o /tmp/kubectl "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl"
sudo install -m 755 /tmp/kubectl /usr/local/bin/kubectl
kubectl version --client
