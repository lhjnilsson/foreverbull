#!/usr/bin/env bash
# Materialize the e2e kubeconfig and open an SSH tunnel to the API.
# The kubeconfig server must be https://127.0.0.1:6443. Nothing is printed
# from the key, the kubeconfig, or the known_hosts file.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=deploy/server/lib.sh
source "${SCRIPT_DIR}/lib.sh"

usage() {
  echo "usage: session.sh start|preflight|stop" >&2
  exit 2
}

session_dir() {
  printf '%s\n' "${FOREVERBULL_SESSION_DIR:-${RUNNER_TEMP:-/tmp}/foreverbull-e2e}"
}

cmd_start() {
  local dir text user host port remote_port local_port sock
  umask 077
  require_deploy_secrets || exit 1
  require_ssh_user "${E2E_SSH_USER}" || die "E2E_SSH_USER must be a single account name"
  require_ssh_host "${E2E_SSH_HOST}" || die "E2E_SSH_HOST must be a hostname or address"
  port="${E2E_SSH_PORT:-22}"
  remote_port="${E2E_SSH_REMOTE_PORT:-6443}"
  local_port="${E2E_SSH_LOCAL_PORT:-6443}"
  require_port "${port}" || die "E2E_SSH_PORT must be a TCP port"
  require_port "${remote_port}" || die "E2E_SSH_REMOTE_PORT must be a TCP port"
  require_port "${local_port}" || die "E2E_SSH_LOCAL_PORT must be a TCP port"
  if [[ "${E2E_SSH_PRIVATE_KEY}" != *"PRIVATE KEY"* || "${E2E_SSH_PRIVATE_KEY}" != *"BEGIN"* ]]; then
    die "E2E_SSH_PRIVATE_KEY must be a PEM private key"
  fi

  text="$(normalize_kubeconfig "${E2E_KUBECONFIG}")" || die "E2E_KUBECONFIG must be kubeconfig YAML, or base64 of that YAML"
  if ! grep -Eq "server:[[:space:]]*\"?https://127\\.0\\.0\\.1:${local_port}(/)?\"?" <<<"${text}"; then
    die "kubeconfig server must be https://127.0.0.1:${local_port}; the workflow forwards that address over SSH"
  fi

  dir="$(session_dir)"
  rm -rf "${dir}"
  mkdir -p "${dir}"
  chmod 700 "${dir}"
  printf '%s\n' "${text}" >"${dir}/kubeconfig"
  chmod 600 "${dir}/kubeconfig"
  printf '%s\n' "${E2E_SSH_PRIVATE_KEY}" | sed 's/\\n/\n/g' >"${dir}/id"
  chmod 600 "${dir}/id"
  if ! grep -q 'PRIVATE KEY' "${dir}/id"; then
    die "E2E_SSH_PRIVATE_KEY must be a PEM private key"
  fi
  printf '%s\n' "${E2E_SSH_KNOWN_HOSTS}" | sed 's/\\n/\n/g' >"${dir}/known_hosts"
  chmod 644 "${dir}/known_hosts"
  if ! grep -qE 'ssh-(ed25519|rsa)|ecdsa-sha2-' "${dir}/known_hosts"; then
    die "E2E_SSH_KNOWN_HOSTS must contain a host key"
  fi

  sock="${FOREVERBULL_SSH_SOCKET:-/tmp/fb-e2e-${GITHUB_RUN_ID:-$$}.sock}"
  if [[ -S "${sock}" ]]; then
    ssh -S "${sock}" -O exit dummy >/dev/null 2>&1 || true
    rm -f "${sock}"
  fi
  printf '%s\n' "${sock}" >"${dir}/socket"
  printf '%s\n' "${E2E_SSH_USER}" >"${dir}/user"
  printf '%s\n' "${E2E_SSH_HOST}" >"${dir}/host"
  printf '%s\n' "${port}" >"${dir}/port"

  ssh -f -N -M \
    -S "${sock}" \
    -i "${dir}/id" \
    -o BatchMode=yes \
    -o IdentitiesOnly=yes \
    -o UserKnownHostsFile="${dir}/known_hosts" \
    -o StrictHostKeyChecking=yes \
    -o ExitOnForwardFailure=yes \
    -o ServerAliveInterval=30 \
    -p "${port}" \
    -L "127.0.0.1:${local_port}:127.0.0.1:${remote_port}" \
    "${E2E_SSH_USER}@${E2E_SSH_HOST}"

  export KUBECONFIG="${dir}/kubeconfig"
  if [[ -n "${GITHUB_ENV:-}" ]]; then
    echo "KUBECONFIG=${dir}/kubeconfig" >>"${GITHUB_ENV}"
  fi
  echo "cluster access is configured"
}

cmd_preflight() {
  local scope verb resource
  if [[ -z "${KUBECONFIG:-}" || ! -s "${KUBECONFIG}" ]]; then
    die "KUBECONFIG is not set; start the session first"
  fi
  command -v kubectl >/dev/null 2>&1 || die "kubectl is not installed"
  kubectl --request-timeout=20s cluster-info >/dev/null
  # Namespaced checks use --all-namespaces so a Role bound only in one
  # namespace cannot pass. e2e-<sha> is a new namespace on every commit.
  while read -r scope verb resource; do
    [[ -n "${scope}" ]] || continue
    if [[ "${scope}" == "cluster" ]]; then
      if ! kubectl --request-timeout=20s auth can-i "${verb}" "${resource}" >/dev/null; then
        echo "kubeconfig cannot ${verb} ${resource}" >&2
        return 1
      fi
    elif ! kubectl --request-timeout=20s auth can-i "${verb}" "${resource}" --all-namespaces >/dev/null; then
      echo "kubeconfig cannot ${verb} ${resource} in every namespace" >&2
      return 1
    fi
  done <<'EOF'
cluster create namespaces
cluster delete namespaces
namespaced create deployments.apps
namespaced get deployments.apps
namespaced delete deployments.apps
namespaced create services
namespaced get services
namespaced delete services
namespaced create secrets
namespaced get secrets
namespaced create persistentvolumeclaims
namespaced create serviceaccounts
namespaced create roles.rbac.authorization.k8s.io
namespaced create rolebindings.rbac.authorization.k8s.io
namespaced create networkpolicies.networking.k8s.io
namespaced get pods
namespaced delete pods
EOF
  if ! kubectl --request-timeout=20s auth can-i get --subresource=log pods --all-namespaces >/dev/null; then
    echo "kubeconfig cannot get pod logs" >&2
    return 1
  fi
  if ! kubectl --request-timeout=20s auth can-i create --subresource=exec pods --all-namespaces >/dev/null; then
    echo "kubeconfig cannot exec into pods" >&2
    return 1
  fi
  echo "kubeconfig can install a release"
}

cmd_stop() {
  local dir sock user host port
  dir="$(session_dir)"
  if [[ -f "${dir}/socket" && -f "${dir}/user" && -f "${dir}/host" && -f "${dir}/port" ]]; then
    sock="$(cat "${dir}/socket")"
    user="$(cat "${dir}/user")"
    host="$(cat "${dir}/host")"
    port="$(cat "${dir}/port")"
    if [[ -n "${sock}" && -S "${sock}" ]]; then
      ssh -S "${sock}" -O exit -p "${port}" "${user}@${host}" >/dev/null 2>&1 || true
      rm -f "${sock}"
    fi
  fi
  rm -rf "${dir}"
  echo "cluster access is closed"
}

[[ $# -eq 1 ]] || usage
case "$1" in
  start) cmd_start ;;
  preflight) cmd_preflight ;;
  stop) cmd_stop ;;
  *) usage ;;
esac
