#!/usr/bin/env bash
# Offline checks for the server release scripts. No cluster and no registry.
# shellcheck disable=SC2030,SC2031,SC2016
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
# shellcheck source=deploy/server/lib.sh
source "${ROOT}/deploy/server/lib.sh"

failed=0

ok() {
  echo "ok $1"
}

bad() {
  echo "FAIL $1" >&2
  failed=1
}

assert_ok() {
  local desc="$1"
  shift
  if "$@"; then
    ok "${desc}"
  else
    bad "${desc}"
  fi
}

assert_fail() {
  local desc="$1"
  shift
  if "$@"; then
    bad "${desc}"
  else
    ok "${desc}"
  fi
}

pin() {
  python3 -c 'import sys; print(sys.argv[1] * 64)' "$1"
}

for script in "${ROOT}/deploy/server/"*.sh; do
  bash -n "${script}" || bad "bash -n ${script}"
done
ok "bash -n"

if grep -R -n -E 'docker[[:space:]]+push|docker[[:space:]]+buildx[[:space:]]+build|docker[[:space:]]+build[[:space:]]' "${ROOT}/deploy/server"; then
  bad "server scripts must not build or docker-push an image"
else
  ok "no image build in server scripts"
fi
if grep -R -n 'manifests/latest' "${ROOT}/deploy/server" --exclude self-test.sh; then
  bad "deleting the latest manifest would delete the shared sha tag"
else
  ok "latest manifest is not deleted"
fi
grep -q 'only fb-nightly is served' "${ROOT}/deploy/server/retarget.sh" || bad "retarget must refuse other namespaces"
grep -q 'imagetools create --tag "${image}:latest"' "${ROOT}/deploy/server/ghcr.sh" || bad "latest must be moved with imagetools"

A="$(pin a)"
B="$(pin b)"
C="$(pin c)"

assert_ok "digest" require_digest "sha256:${A}"
assert_fail "short digest" require_digest "sha256:abc"
assert_fail "upper digest" require_digest "sha256:${A^^}"
assert_ok "sha tag" require_sha_tag "sha-abc1234"
assert_fail "latest tag" require_sha_tag "latest"
assert_fail "short sha tag" require_sha_tag "sha-abc"
assert_ok "ghcr prefix" require_ghcr_repo "ghcr.io/lhjnilsson"
assert_fail "ghcr image as prefix" require_ghcr_repo "ghcr.io/lhjnilsson/foreverbull"
assert_fail "docker hub prefix" require_ghcr_repo "docker.io/lhjnilsson"
assert_ok "nightly namespace" require_release_namespace "fb-nightly"
assert_ok "e2e namespace" require_release_namespace "e2e-abc1234"
assert_fail "other namespace" require_release_namespace "kube-system"
assert_ok "grafana origin" require_grafana_url "https://nightly.foreverbull.com"
assert_fail "grafana http" require_grafana_url "http://nightly.foreverbull.com"
assert_fail "grafana path" require_grafana_url "https://nightly.foreverbull.com/d"

got="$(digest_from_ref "ghcr.io/example/foreverbull@sha256:${A}")"
[[ "${got}" == "sha256:${A}" ]] || bad "digest_from_ref printed ${got}"
assert_fail "tag ref" digest_from_ref "ghcr.io/example/foreverbull:sha-abc1234"

raw=$'apiVersion: v1\nclusters:\n- name: local\n'
b64="$(printf '%s' "${raw}" | base64 -w 0)"
norm="$(normalize_kubeconfig "${b64}")"
[[ "${norm}" == *"apiVersion:"* && "${norm}" == *"clusters:"* ]] || bad "base64 kubeconfig"
assert_fail "kubeconfig junk" normalize_kubeconfig "not-a-kubeconfig"

(
  export E2E_KUBECONFIG=x E2E_SSH_PRIVATE_KEY=x E2E_SSH_KNOWN_HOSTS=x E2E_SSH_HOST=x E2E_SSH_USER=x
  require_deploy_secrets
) || bad "secrets present"
(
  export E2E_KUBECONFIG=x E2E_SSH_PRIVATE_KEY=x E2E_SSH_KNOWN_HOSTS=x E2E_SSH_HOST=x
  unset E2E_SSH_USER
  require_deploy_secrets
) && bad "secrets missing user"
ok "deploy secrets"

values="$(mktemp)"
(
  export IMAGE_REPOSITORY=ghcr.io/example
  export FOREVERBULL_DIGEST="sha256:${A}"
  export ZIPLINE_DIGEST="sha256:${B}"
  export GRAFANA_DIGEST="sha256:${C}"
  export GRAFANA_URL=https://nightly.foreverbull.com
  write_pin_values "${values}"
)
grep -Fq "tag: sha256:${A}" "${values}" || bad "broker pin"
grep -Fq "url: https://nightly.foreverbull.com" "${values}" || bad "grafana url"
grep -Fq "imageTag: pinned" "${values}" || bad "fallback tag"
if grep -Eq 'latest|allowDefaults: true|hostPort: true' "${values}"; then
  bad "pin values include a moving tag or local defaults"
else
  ok "pin values"
fi
rm -f "${values}"
assert_fail "pin rejects bad repo" bash -c "
  set -euo pipefail
  source '${ROOT}/deploy/server/lib.sh'
  export IMAGE_REPOSITORY=\$'ghcr.io/example\nbroker:'
  export FOREVERBULL_DIGEST='sha256:${A}'
  export ZIPLINE_DIGEST='sha256:${B}'
  export GRAFANA_DIGEST='sha256:${C}'
  write_pin_values /tmp/foreverbull-pin-rejected
"

(
  export GITHUB_REPOSITORY_OWNER=LhjNilsson
  export GITHUB_SHA=0123456789abcdef0123456789abcdef01234567
  ref="$(bash "${ROOT}/deploy/server/image-ref.sh" foreverbull)"
  grep -qx 'IMAGE_REPOSITORY=ghcr.io/lhjnilsson' <<<"${ref}" || exit 1
  grep -qx 'SHORT_SHA=0123456' <<<"${ref}" || exit 1
  grep -qx 'SHA_TAG=sha-0123456' <<<"${ref}" || exit 1
  grep -qx 'IMAGE=ghcr.io/lhjnilsson/foreverbull' <<<"${ref}" || exit 1
) || bad "image ref"
ok "image ref"

(
  unset FOREVERBULL_DIGEST ZIPLINE_DIGEST GRAFANA_DIGEST
  [[ "$(classify_rollback_inputs)" == "previous" ]]
) || bad "empty rollback"
(
  export FOREVERBULL_DIGEST="sha256:${A}"
  export ZIPLINE_DIGEST="sha256:${B}"
  export GRAFANA_DIGEST="sha256:${C}"
  [[ "$(classify_rollback_inputs)" == "explicit" ]]
) || bad "explicit rollback"
(
  export FOREVERBULL_DIGEST="sha256:${A}"
  unset ZIPLINE_DIGEST GRAFANA_DIGEST
  classify_rollback_inputs
) && bad "partial rollback"
ok "rollback inputs"

rev="$(
  select_previous_revision <<'JSON'
[
  {"revision": 1, "status": "superseded"},
  {"revision": 2, "status": "deployed"}
]
JSON
)"
[[ "${rev}" == "1" ]] || bad "previous of deployed was ${rev}"
rev="$(
  select_previous_revision <<'JSON'
[
  {"revision": 1, "status": "deployed"},
  {"revision": 2, "status": "failed"}
]
JSON
)"
[[ "${rev}" == "1" ]] || bad "previous of failed was ${rev}"
rev="$(
  select_previous_revision <<'JSON'
[
  {"revision": 1, "status": "superseded"},
  {"revision": 2, "status": "deployed"},
  {"revision": 3, "status": "failed"}
]
JSON
)"
[[ "${rev}" == "2" ]] || bad "last good after a failed upgrade was ${rev}"
if select_previous_revision >/dev/null <<'JSON'
[{"revision": 1, "status": "deployed"}]
JSON
then
  bad "sole revision must be refused"
else
  ok "sole revision"
fi

manifest="$(mktemp)"
cat >"${manifest}" <<EOF
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: foreverbull
spec:
  template:
    spec:
      containers:
        - name: foreverbull
          image: ghcr.io/example/foreverbull@sha256:${A}
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: postgres
spec:
  template:
    spec:
      containers:
        - name: postgres
          image: postgres:13.3-alpine
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: grafana
spec:
  template:
    spec:
      containers:
        - name: grafana
          image: "ghcr.io/example/fb-grafana@sha256:${B}"
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: zipline
spec:
  template:
    spec:
      containers:
        - name: zipline
          image: ghcr.io/example/zipline:sha-abc1234
EOF
parsed="$(parse_release_images <"${manifest}")"
rm -f "${manifest}"
grep -Fq "foreverbull ghcr.io/example/foreverbull@sha256:${A}" <<<"${parsed}" || bad "parse broker"
grep -Fq "grafana ghcr.io/example/fb-grafana@sha256:${B}" <<<"${parsed}" || bad "parse grafana quotes"
grep -Fq "zipline ghcr.io/example/zipline:sha-abc1234" <<<"${parsed}" || bad "parse zipline tag"
if grep -q 'postgres' <<<"${parsed}"; then
  bad "parse kept postgres"
else
  ok "parse release images"
fi

wf="${ROOT}/.github/workflows/e2e_server.yaml"
[[ -f "${wf}" ]] || bad "missing e2e_server workflow"
head_on="$(sed -n '1,45p' "${wf}")"
if grep -q 'pull_request:' <<<"${head_on}"; then
  bad "e2e_server must not run on pull_request"
else
  ok "trigger is not a pull request"
fi
grep -q -- '- main' <<<"${head_on}" || bad "trigger must include main"
if grep -E -q 'PYPI|DOCKERHUB|docker\.io|cron:' "${wf}"; then
  bad "e2e_server pulls in an out-of-scope publisher"
else
  ok "publisher scope"
fi
grep -q 'environment: e2e' "${wf}" || bad "workflow must use the e2e environment"
grep -q 'packages: write' "${wf}" || bad "publish jobs need packages:write"
grep -q 'fb-nightly' "${wf}" || bad "workflow must name fb-nightly"
grep -q 'image-ref.sh' "${wf}" || bad "workflow must name images through image-ref.sh"
grep -q 'existing-digest' "${wf}" || bad "workflow must reuse sha tags instead of overwriting them"
grep -q 'retarget.sh' "${wf}" || bad "workflow must move latest and the nightly release together"

if [[ "${failed}" -ne 0 ]]; then
  echo "server release self-test failed" >&2
  exit 1
fi
echo "server release self-test passed"
