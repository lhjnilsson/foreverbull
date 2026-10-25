#!/usr/bin/env bash
# Exit before any registry or cluster write when the e2e environment is incomplete.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=deploy/server/lib.sh
source "${SCRIPT_DIR}/lib.sh"

require_deploy_secrets
echo "e2e environment secrets are set"
