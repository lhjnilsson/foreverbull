# Security policy

## 🔒️ Reporting a vulnerability

Report a vulnerability in private through
[GitHub Security Advisories](https://github.com/lhjnilsson/foreverbull/security/advisories/new).

Do not open a public issue for a vulnerability. Leave trading
credentials, tokens, and the contents of a profiles file out of the
report. Name the component, the commit or image you ran, and the
impact.

## What a report should cover

The broker, the Zipline and Grafana images built from this repository,
the Helm chart, and the Python client and CLIs (`fbull`, `fbenv`).

Fixes land on the default branch. This repository does not publish a
maintained release branch.

## Runtime

The broker, Zipline, and Grafana containers are built and scheduled to
run as a non-root user, with the runtime's default seccomp profile and
no Linux capabilities. Postgres, NATS, and the object-store image stay
on the user those upstream images start as. The chart contract is
[Container security](deploy/README.md#container-security).

## Published images

A push to `main` publishes three multi-arch images:

- `ghcr.io/lhjnilsson/foreverbull`
- `ghcr.io/lhjnilsson/zipline`
- `ghcr.io/lhjnilsson/fb-grafana`

`.github/workflows/e2e_server.yaml` builds `linux/amd64` and
`linux/arm64` and tags the index `sha-<short sha>`. BuildKit attaches
an SBOM and SLSA provenance (`mode=max`) to that index. Keyless Cosign
then signs the index digest. The signing certificate is the GitHub
Actions identity for this workflow on `refs/heads/main`. There is no
maintainer key.

`latest` moves to that index only after the nightly deploy succeeds.
Verify the digest. A `sha-*` tag is the immutable name of the signed
index. A digest published before this signing step has no signature.

Local images (`VERSION=local`) and images built by other workflows are
unsigned. `fbenv` does not run Cosign, and installing the chart does
not require it.

### Signature

The digest below is the index. Read it from the `sha-*` tag:

```bash
docker buildx imagetools inspect --format '{{.Manifest.Digest}}' \
  ghcr.io/lhjnilsson/foreverbull:sha-<short sha>
```

```bash
cosign verify \
  --certificate-identity=https://github.com/lhjnilsson/foreverbull/.github/workflows/e2e_server.yaml@refs/heads/main \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com \
  ghcr.io/lhjnilsson/foreverbull@sha256:<digest>
```

`zipline` and `fb-grafana` use the same identity and issuer. A non-zero
exit means this workflow on `main` did not sign that digest. Use the
index digest from a `sha-*` tag published by a run that includes the
signing step. The signature is on that index.

### SBOM and provenance

BuildKit stores both on the index. This reads them:

```bash
docker buildx imagetools inspect ghcr.io/lhjnilsson/foreverbull@sha256:<digest>
```

`cosign verify` checks the signature. The SBOM and provenance are the
BuildKit attestations in that inspect output.
