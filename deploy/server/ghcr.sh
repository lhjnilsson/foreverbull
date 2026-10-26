#!/usr/bin/env bash
# GHCR tag operations for the server channel.
# Pushes of sha-* happen in the workflow via build-push-action.
# This script inspects sha-* tags, signs a digest with keyless Cosign,
# moves the latest tag onto an existing digest, and asks GitHub to mark
# the package public.
# It never deletes a manifest: a sha-* tag and latest share one digest, and
# deleting the manifest would remove the immutable tag.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=deploy/server/lib.sh
source "${SCRIPT_DIR}/lib.sh"

usage() {
  echo "usage: ghcr.sh existing-digest|move-latest|ensure-public|sign ..." >&2
  exit 2
}

# Prints the digest, or the word "absent" when the sha-* tag is not there.
# Refuses when the tag exists for a different git revision.
cmd_existing_digest() {
  local image="$1"
  local tag="$2"
  local revision="$3"
  local ref err digest ann
  require_ghcr_image "$image" || die "image must be ghcr.io/<owner>/<name> without a tag or digest"
  require_sha_tag "$tag" || die "refusing to inspect a tag that is not sha-<hex>"
  [[ "$revision" =~ ^[0-9a-f]{40}$ ]] || die "revision must be the full git sha"
  ref="${image}:${tag}"
  if ! err="$(docker buildx imagetools inspect "$ref" 2>&1)"; then
    if grep -qiE 'not found|manifest unknown|no such manifest|name unknown|unauthorized|denied' <<<"$err"; then
      printf 'absent\n'
      return 0
    fi
    die "could not inspect ${ref}"
  fi
  digest="$(docker buildx imagetools inspect --format '{{.Manifest.Digest}}' "$ref")"
  ann="$(docker buildx imagetools inspect --format '{{if .Manifest.Annotations}}{{index .Manifest.Annotations "org.opencontainers.image.revision"}}{{end}}' "$ref" 2>/dev/null || true)"
  require_digest "$digest" || die "existing ${ref} did not resolve to a digest"
  if [[ "$ann" != "$revision" ]]; then
    die "tag ${ref} already exists for revision '${ann:-unknown}', not ${revision}. Refusing to overwrite a sha-* tag."
  fi
  printf '%s\n' "$digest"
}

# Point :latest at an existing digest. Does not build and does not retag sha-*.
cmd_move_latest() {
  local image="$1"
  local digest="$2"
  local got
  require_ghcr_image "$image" || die "image must be ghcr.io/<owner>/<name> without a tag or digest"
  require_digest "$digest" || die "digest must be sha256:<64 hex>"
  docker buildx imagetools create --tag "${image}:latest" "${image}@${digest}"
  got="$(docker buildx imagetools inspect --format '{{.Manifest.Digest}}' "${image}:latest")"
  if [[ "$got" != "$digest" ]]; then
    die "latest on ${image} resolved to ${got}, not ${digest}"
  fi
  echo "latest -> ${image}@${digest}"
}

# Sign one image by digest. The workflow passes the multi-arch index digest
# that build-push-action just pushed. Keyless: the job's OIDC token is the
# signing identity, and there is no key file. A tag is not accepted here.
cmd_sign() {
  local image="$1"
  local digest="$2"
  require_ghcr_image "$image" || die "image must be ghcr.io/<owner>/<name> without a tag or digest"
  require_digest "$digest" || die "digest must be sha256:<64 hex>"
  command -v cosign >/dev/null 2>&1 || die "cosign is not on PATH"
  if ! cosign sign --yes "${image}@${digest}"; then
    die "could not sign ${image}@${digest}. The sha-* tag is already in GHCR; the next run reuses it and skips signing. Delete that tag before re-running this commit."
  fi
  echo "signed ${image}@${digest}"
}

# Best-effort public visibility. A confirmed private package stops the job.
# An unknown visibility does not: the server pull fails e2e before latest moves.
cmd_ensure_public() {
  local name="$1"
  local attempt vis=""
  [[ "$name" =~ ^[a-z0-9][a-z0-9._-]*$ ]] || die "unexpected package name"
  if [[ -z "${GH_TOKEN:-}" ]]; then
    die "GH_TOKEN is required to read GHCR package visibility"
  fi
  export GH_TOKEN
  for attempt in 1 2 3 4 5 6; do
    if vis="$(gh api "/user/packages/container/${name}" --jq .visibility 2>/dev/null)"; then
      break
    fi
    vis=""
    if [[ "$attempt" -lt 6 ]]; then
      sleep 5
    fi
  done
  if [[ "$vis" != "public" ]]; then
    gh api --method PATCH \
      -H "Accept: application/vnd.github+json" \
      "/user/packages/container/${name}/settings" \
      -f visibility='public' >/dev/null 2>&1 || true
    vis="$(gh api "/user/packages/container/${name}" --jq .visibility 2>/dev/null || true)"
  fi
  if [[ "$vis" == "private" ]]; then
    die "GHCR package ${name} is private. Make it public so the server can pull it. latest was not moved."
  fi
  if [[ "$vis" != "public" ]]; then
    echo "could not confirm GHCR package ${name} is public (visibility: ${vis:-unknown}). Server e2e fails closed if the node cannot pull." >&2
    return 0
  fi
  echo "GHCR package ${name} is public"
}

[[ $# -ge 1 ]] || usage
case "$1" in
  existing-digest)
    [[ $# -eq 4 ]] || usage
    cmd_existing_digest "$2" "$3" "$4"
    ;;
  move-latest)
    [[ $# -eq 3 ]] || usage
    cmd_move_latest "$2" "$3"
    ;;
  ensure-public)
    [[ $# -eq 2 ]] || usage
    cmd_ensure_public "$2"
    ;;
  sign)
    [[ $# -eq 3 ]] || usage
    cmd_sign "$2" "$3"
    ;;
  *)
    usage
    ;;
esac
