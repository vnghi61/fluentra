#!/usr/bin/env bash
# Opens, updates or closes one GitHub issue per operations workflow.
#
#   ops-alert.sh failed    → open "<workflow> is failing", or comment on it if open
#   ops-alert.sh recovered → close that issue, if one is open
#
# The scheduled tts-render failed on 2026-10-12 with the catalogue half voiced
# and nobody knew: scheduled runs fail silently unless someone opens the
# Actions tab. An issue reaches whoever watches the repository, and repeated
# failures land as comments on one issue rather than a new one an hour.
#
# Needs GH_TOKEN with issues: write. Prints nothing secret: the run URL only.
set -euo pipefail

mode="${1:?usage: ops-alert.sh failed|recovered}"
repo="${GITHUB_REPOSITORY:?}"
workflow="${GITHUB_WORKFLOW:?}"
run_url="${GITHUB_SERVER_URL:-https://github.com}/${repo}/actions/runs/${GITHUB_RUN_ID:?}"
label="ops-alert"
title="${workflow} is failing"

existing="$(gh issue list --repo "$repo" --state open --label "$label" \
  --json number,title --jq ".[] | select(.title == \"${title}\") | .number" | head -n 1 || true)"

case "$mode" in
  failed)
    gh label create "$label" --repo "$repo" --color B60205 \
      --description "An operations workflow is failing" --force >/dev/null
    if [ -n "$existing" ]; then
      gh issue comment "$existing" --repo "$repo" --body "Failed again: ${run_url}"
    else
      gh issue create --repo "$repo" --label "$label" --title "$title" --body \
        "The **${workflow}** workflow failed: ${run_url}

This issue closes itself on the next successful run. Further failures are added below."
    fi
    ;;
  recovered)
    if [ -n "$existing" ]; then
      gh issue close "$existing" --repo "$repo" --comment "Recovered: ${run_url}"
    fi
    ;;
  *)
    echo "unknown mode: $mode" >&2
    exit 2
    ;;
esac
