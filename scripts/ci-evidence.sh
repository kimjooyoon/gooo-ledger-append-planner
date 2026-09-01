#!/usr/bin/env bash
set -euo pipefail

repository=${GITHUB_REPOSITORY:?GITHUB_REPOSITORY is required}
run_id=${GITHUB_RUN_ID:?GITHUB_RUN_ID is required}
event_number=${GITHUB_EVENT_NUMBER:-0}
event_name=${GITHUB_EVENT_NAME:-unknown}
server_url=${GITHUB_SERVER_URL:-https://github.com}

jq -n \
  --arg repository "$repository" \
  --arg run_url "$server_url/$repository/actions/runs/$run_id" \
  --arg event_name "$event_name" \
  --arg ref "${GITHUB_REF:-}" \
  --arg workflow "${GITHUB_WORKFLOW:-}" \
  --arg sha "${GITHUB_SHA:-}" \
  --argjson run_id "$run_id" \
  --argjson event_number "$event_number" \
  '{
    schema: "gooo/ledger-append-planner/github-actions-evidence/v1",
    append_only: true,
    authority: "GITHUB_ACTIONS",
    repository: $repository,
    event: {name: $event_name, number: $event_number, ref: $ref},
    workflow: $workflow,
    run: {id: $run_id, url: $run_url, head_sha: $sha},
    local_execution_policy: "NO_ADDITIONAL_LOCAL_TEST_BUILD_OR_CONFORMANCE",
    recorded_at: (now | todate)
  }'
