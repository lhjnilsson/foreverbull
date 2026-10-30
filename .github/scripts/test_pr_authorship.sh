#!/usr/bin/env bash
# Runs pr_authorship.sh against fixture commit lists and checks each exit
# status. The fixtures follow the shape of the pulls commits API.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
testdata="${here}/testdata/pr_authorship"
bot=lhjnilsson-bot
failures=0

# expect STATUS PR_AUTHOR FIXTURE
expect() {
  local want="$1" author="$2" fixture="$3" got=0
  PR_AUTHOR="${author}" BOT_LOGINS="${bot}" \
    "${here}/pr_authorship.sh" "${testdata}/${fixture}" >/dev/null || got=$?
  if [[ "${got}" -ne "${want}" ]]; then
    echo "FAIL ${fixture}: exit ${got}, want ${want}"
    failures=$((failures + 1))
    return
  fi
  echo "ok   ${fixture}: exit ${got}"
}

expect 0 "${bot}" pass_all_by_author.json
expect 0 "${bot}" pass_update_branch_merge.json
expect 1 "${bot}" fail_hand_merge_not_web_flow.json
expect 1 "${bot}" fail_hand_merge_unverified.json
expect 1 "${bot}" fail_hand_merge_github_email_other_login.json
expect 1 "${bot}" fail_hand_merge_unlinked_committer.json
expect 1 "${bot}" fail_hand_merge_null_login_github_email.json
expect 1 "${bot}" fail_hand_merge_web_flow_other_email.json
expect 1 "${bot}" fail_non_merge_other_author.json
expect 0 maintainer warn_human_mismatch.json

if [[ "${failures}" -ne 0 ]]; then
  echo "${failures} case(s) failed"
  exit 1
fi
