#!/usr/bin/env bash
#
# Fail when a workflow pins a GitHub Action to a SHA that does not exist.
#
# Pinning by SHA is the supply-chain control; a SHA nobody resolved is a string
# that looks like one. `github/codeql-action` was pinned to
# b56ba49b26e50535fa1ea49bd30e9dd3845e789d — forty plausible hex characters that
# are not a commit in that repository — and the CodeQL and Trivy jobs failed at
# "Set up job" with "unable to resolve action", which reads like an outage
# rather than like a typo that had been sitting in the file.
#
# Needs `gh` authenticated. Run it after changing any `uses:` line.

set -euo pipefail

status=0

while read -r ref; do
  path="${ref%@*}"
  sha="${ref#*@}"
  repo="$(echo "$path" | cut -d/ -f1-2)"

  if gh api "repos/$repo/commits/$sha" --jq .sha >/dev/null 2>&1; then
    printf 'ok   %s\n' "$ref"
  else
    printf 'BAD  %s  <- no such commit in %s\n' "$ref" "$repo"
    status=1
  fi
done < <(
  grep -ohE 'uses: [a-zA-Z0-9_.-]+/[a-zA-Z0-9_./-]+@[a-f0-9]{40}' .github/workflows/*.yml \
    | sed 's/^uses: //' | sort -u
)

if [ "$status" -ne 0 ]; then
  echo
  echo 'A pinned SHA does not resolve. Find the real one with:'
  echo '  gh api repos/<owner>/<repo>/git/ref/tags/<tag> --jq .object.sha'
  exit 1
fi
