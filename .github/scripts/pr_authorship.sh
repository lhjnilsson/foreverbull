#!/usr/bin/env bash
# Compares a pull request's author with the authors of its commits.
#
# Usage: pr_authorship.sh COMMITS_JSON
#   COMMITS_JSON is the output of `gh api --paginate
#   repos/OWNER/REPO/pulls/N/commits`: one JSON array per page.
#   PR_AUTHOR is the pull request author's login.
#   BOT_LOGINS is a space-separated list of bot logins.
#
# Mixed authorship is a warning; it fails only when a bot account is on one
# side of the mismatch.
set -euo pipefail

commits_json="${1:?usage: pr_authorship.sh COMMITS_JSON}"
: "${PR_AUTHOR:?PR_AUTHOR is required}"
: "${BOT_LOGINS:?BOT_LOGINS is required}"

# A merge commit GitHub made for "Update branch" is authored by whoever
# clicked, so it is skipped. Only a verified merge committed by web-flow
# qualifies: a hand-made merge can carry arbitrary changes (an evil merge)
# and is checked like any other commit. A missing committer login does not
# count as web-flow.
# Commits with no linked GitHub account show their git author name.
# shellcheck disable=SC2016 # $c is a jq variable.
filter='
  .[]
  | . as $c
  | if (($c.parents | length) > 1)
       and ($c.commit.committer.email == "noreply@github.com")
       and ($c.committer.login == "web-flow")
       and ($c.commit.verification.verified == true)
    then empty
    else ($c.author.login // ("git:" + $c.commit.author.name))
    end
'
authors="$(jq -r "${filter}" "${commits_json}" | sort -u)"

others="$(grep -vxF "${PR_AUTHOR}" <<<"${authors}" || true)"
if [[ -z "${others}" ]]; then
  echo "All commits are authored by ${PR_AUTHOR}."
  exit 0
fi
others_list="$(paste -sd ',' <<<"${others}" | sed 's/,/, /g')"
message="Pull request author ${PR_AUTHOR} differs from commit author(s): ${others_list}."
bot_involved=false
for bot in ${BOT_LOGINS}; do
  if [[ "${PR_AUTHOR}" == "${bot}" ]] || grep -qxF "${bot}" <<<"${others}"; then
    bot_involved=true
  fi
done
if [[ "${bot_involved}" == true ]]; then
  echo "::error title=Pull request authorship::${message} A bot must open the pull requests for its own commits, from its own account."
  exit 1
fi
echo "::warning title=Pull request authorship::${message}"
