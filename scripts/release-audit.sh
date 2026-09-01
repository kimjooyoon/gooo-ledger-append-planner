#!/usr/bin/env bash
set -euo pipefail

repository=${GITHUB_REPOSITORY:?GITHUB_REPOSITORY is required}
tag=${GITHUB_REF_NAME:?GITHUB_REF_NAME is required}
run_id=${GITHUB_RUN_ID:?GITHUB_RUN_ID is required}
sha=${GITHUB_SHA:?GITHUB_SHA is required}
server_url=${GITHUB_SERVER_URL:-https://github.com}
api_version=2022-11-28

immutable=$(gh api \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: $api_version" \
  "repos/$repository/immutable-releases")
test "$(jq -r '.enabled' <<<"$immutable")" = true

tag_ref=$(gh api \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: $api_version" \
  "repos/$repository/git/ref/tags/$tag")
test "$(jq -r '.object.type' <<<"$tag_ref")" = tag
tag_object_sha=$(jq -r '.object.sha' <<<"$tag_ref")
tag_object=$(gh api \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: $api_version" \
  "repos/$repository/git/tags/$tag_object_sha")
target_commit_sha=$(jq -r '.object.sha' <<<"$tag_object")
test "$(jq -r '.object.type' <<<"$tag_object")" = commit
test "$target_commit_sha" = "$sha"

release=$(gh api \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: $api_version" \
  "repos/$repository/releases/tags/$tag")
release_id=$(jq -r '.id' <<<"$release")
test "$(jq -r '.immutable' <<<"$release")" = true
test "$(jq -r '.target_commitish' <<<"$release")" = "$sha"

assets=$(gh api --paginate \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: $api_version" \
  "repos/$repository/releases/$release_id/assets?per_page=100" | jq -s 'add | map({id, name, size, digest, browser_download_url})')
run=$(gh api \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: $api_version" \
  "repos/$repository/actions/runs/$run_id")
jobs=$(gh api --paginate \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: $api_version" \
  "repos/$repository/actions/runs/$run_id/jobs?per_page=100" | jq -s 'map(.jobs) | add | map({id, name, status, conclusion, started_at, completed_at, html_url})')
artifacts=$(gh api --paginate \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: $api_version" \
  "repos/$repository/actions/runs/$run_id/artifacts?per_page=100" | jq -s 'map(.artifacts) | add | map({id, name, size_in_bytes, expired, archive_download_url})')

local_files=$(for file in release/*; do
  test -f "$file"
  digest=$(sha256sum "$file" | awk '{print $1}')
  jq -n --arg name "$(basename "$file")" --arg digest "sha256:$digest" '{name: $name, sha256: $digest}'
done | jq -s .)

jq -n \
  --arg repository "$repository" \
  --arg tag "$tag" \
  --arg sha "$sha" \
  --arg tag_object_sha "$tag_object_sha" \
  --arg target_commit_sha "$target_commit_sha" \
  --argjson immutable "$immutable" \
  --argjson tag_ref "$tag_ref" \
  --argjson tag_object "$tag_object" \
  --argjson release "$release" \
  --argjson assets "$assets" \
  --argjson run "$run" \
  --argjson jobs "$jobs" \
  --argjson artifacts "$artifacts" \
  --argjson local_files "$local_files" \
  '{
    schema: "gooo/ledger-append-planner/release-audit/v1",
    append_only: true,
    authority: "GITHUB_ACTIONS_REST",
    repository: $repository,
    tag: {name: $tag, ref_object_sha: $tag_ref.object.sha, ref_object_type: $tag_ref.object.type, tag_object_sha: $tag_object_sha, annotated_target_commit_sha: $target_commit_sha, tag_url: $tag_ref.url},
    immutable_releases: {enabled: $immutable.enabled, enforced_by_owner: $immutable.enforced_by_owner},
    release: {id: $release.id, immutable: $release.immutable, target_commitish: $release.target_commitish, html_url: $release.html_url, published_at: $release.published_at},
    workflow_run: {id: $run.id, name: $run.name, event: $run.event, status: $run.status, conclusion: $run.conclusion, head_sha: $run.head_sha, html_url: $run.html_url},
    jobs: $jobs,
    artifacts: $artifacts,
    release_assets: $assets,
    local_release_files: $local_files,
    observed_commit_sha: $sha
  }'
