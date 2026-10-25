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
