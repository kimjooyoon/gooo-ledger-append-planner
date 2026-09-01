#!/usr/bin/env bash
set -Eeuo pipefail
trap 'status=$?; echo "release verification failed at line ${BASH_LINENO[0]}: ${BASH_COMMAND}" >&2; exit "$status"' ERR

if [ "$#" -ne 2 ]; then
  echo "usage: verify-releases.sh LOCK_JSON OUTPUT_DIR" >&2
  exit 64
fi

lock=$(realpath "$1")
output=$(realpath "$2" 2>/dev/null || { mkdir -p "$2"; realpath "$2"; })
repository=$(realpath .)
case "$output" in
  "$repository"|"$repository"/*)
    echo "release output must be outside the input repository" >&2
    exit 65
    ;;
esac
mkdir -p "$output"

command -v gh >/dev/null
command -v curl >/dev/null
command -v git >/dev/null
command -v jq >/dev/null
command -v sha256sum >/dev/null

measurement() {
  local start=$1
  local end=$2
  local peak=${3:-0}
  jq -n --argjson wall "$(( (end - start) / 1000000 ))" --argjson raw "$((end - start))" --argjson peak "$peak" \
    '{wall_ms:$wall,duration_ns:$raw,peak_rss_kib:$peak}'
}

fetch_start=$(date +%s%N)
for key in $(jq -r '.releases | keys[]' "$lock"); do
  repo=$(jq -r --arg key "$key" '.releases[$key].repository' "$lock")
  tag=$(jq -r --arg key "$key" '.releases[$key].tag' "$lock")
  /usr/bin/time -f '%M' -o "$output/$key.fetch-rss" \
    gh api "repos/$repo/releases/tags/$tag" > "$output/$key.release.json"
done
for counterexample_id in $(jq -r '(.counterexamples // [])[] | .counterexample_id' "$lock"); do
  repo=$(jq -r --arg id "$counterexample_id" '.counterexamples[] | select(.counterexample_id==$id) | .repository' "$lock")
  tag=$(jq -r --arg id "$counterexample_id" '.counterexamples[] | select(.counterexample_id==$id) | .tag' "$lock")
  if jq -e --arg id "$counterexample_id" '.counterexamples[] | select(.counterexample_id==$id) | .historical_release == true' "$lock" >/dev/null; then
    release_endpoint=$(jq -r --arg id "$counterexample_id" '.counterexamples[] | select(.counterexample_id==$id) | .release_api_endpoint' "$lock")
    if gh api "$release_endpoint" > "$output/$counterexample_id.release.json"; then
      echo "historical release unexpectedly present: $release_endpoint" >&2
      exit 1
    fi
    printf '{"message":"Not Found","status":404}\n' > "$output/$counterexample_id.release.json"
    printf '0\n' > "$output/$counterexample_id.fetch-rss"
  else
    /usr/bin/time -f '%M' -o "$output/$counterexample_id.fetch-rss" \
      gh api "repos/$repo/releases/tags/$tag" > "$output/$counterexample_id.release.json"
  fi
done
for counterexample_id in $(jq -r '(.counterexample_runs // [])[] | .counterexample_id' "$lock"); do
  repo=$(jq -r --arg id "$counterexample_id" '.counterexample_runs[] | select(.counterexample_id==$id) | .repository' "$lock")
  run_id=$(jq -r --arg id "$counterexample_id" '.counterexample_runs[] | select(.counterexample_id==$id) | .run_id' "$lock")
  job_id=$(jq -r --arg id "$counterexample_id" '.counterexample_runs[] | select(.counterexample_id==$id) | .job_id' "$lock")
  /usr/bin/time -f '%M' -o "$output/$counterexample_id.fetch-rss" \
    gh api "repos/$repo/actions/runs/$run_id" > "$output/$counterexample_id.run.json"
  gh api "repos/$repo/actions/jobs/$job_id" > "$output/$counterexample_id.job.json"
done
for counterexample_id in $(jq -r '(.failed_release_triggers // [])[] | .counterexample_id' "$lock"); do
  repo=$(jq -r --arg id "$counterexample_id" '.failed_release_triggers[] | select(.counterexample_id==$id) | .repository' "$lock")
  tag=$(jq -r --arg id "$counterexample_id" '.failed_release_triggers[] | select(.counterexample_id==$id) | .tag' "$lock")
  run_id=$(jq -r --arg id "$counterexample_id" '.failed_release_triggers[] | select(.counterexample_id==$id) | .failed_run.run_id' "$lock")
  job_id=$(jq -r --arg id "$counterexample_id" '.failed_release_triggers[] | select(.counterexample_id==$id) | .failed_run.job_id' "$lock")
  if gh api "repos/$repo/releases/tags/$tag" > "$output/$counterexample_id.release.json"; then
    release_exit=0
  else
    release_exit=$?
  fi
  test "$release_exit" -ne 0
  gh api "repos/$repo/actions/runs/$run_id" > "$output/$counterexample_id.failed-run.json"
  gh api "repos/$repo/actions/jobs/$job_id" > "$output/$counterexample_id.failed-job.json"
  printf '0\n' > "$output/$counterexample_id.fetch-rss"
done
fetch_end=$(date +%s%N)

verify_start=$(date +%s%N)
for key in $(jq -r '.releases | keys[]' "$lock"); do
  repo=$(jq -r --arg key "$key" '.releases[$key].repository' "$lock")
  tag=$(jq -r --arg key "$key" '.releases[$key].tag' "$lock")
  release_url=$(jq -r --arg key "$key" '.releases[$key].release_url' "$lock")
  target=$(jq -r --arg key "$key" '.releases[$key].target_commit_sha' "$lock")
  release_id=$(jq -r --arg key "$key" '.releases[$key].release_id // empty' "$lock")
  expected_tag_object=$(jq -r --arg key "$key" '.releases[$key].tag_object_sha // empty' "$lock")
  release_json="$output/$key.release.json"

  if [ -n "$release_id" ]; then
    if ! jq -e --arg tag "$tag" --arg url "$release_url" --argjson release_id "$release_id" \
      '.id==$release_id and .tag_name==$tag and .html_url==$url and .draft==false and .prerelease==false and .immutable==true' \
      "$release_json" >/dev/null; then
      echo "release metadata mismatch for $repo@$tag" >&2
      jq -c '{id,tag_name,html_url,draft,prerelease,immutable}' "$release_json" >&2
      exit 1
    fi
  else
    if ! jq -e --arg tag "$tag" --arg url "$release_url" \
      '.tag_name==$tag and .html_url==$url and .draft==false and .prerelease==false and .immutable==true' \
      "$release_json" >/dev/null; then
      echo "release metadata mismatch for $repo@$tag" >&2
      jq -c '{id,tag_name,html_url,draft,prerelease,immutable}' "$release_json" >&2
      exit 1
    fi
  fi

  tag_target=""
  remote_refs=""
  for attempt in 1 2 3; do
    if remote_refs=$(git ls-remote "https://github.com/$repo.git" "refs/tags/$tag" "refs/tags/$tag^{}"); then
      tag_target=$(awk -v direct="refs/tags/$tag" -v peeled="refs/tags/$tag^{}" '$2==peeled {p=$1} $2==direct {d=$1} END {if (p!="") print p; else print d}' <<< "$remote_refs")
      break
    fi
    echo "tag lookup attempt $attempt failed for $repo@$tag" >&2
    sleep 1
  done
  tag_object=$(awk -v direct="refs/tags/$tag" '$2==direct {print $1}' <<< "$remote_refs")
  if test "$tag_target" != "$target" || { [ -n "$expected_tag_object" ] && test "$tag_object" != "$expected_tag_object"; }; then
    echo "tag target mismatch for $repo@$tag: expected $target, observed ${tag_target:-<empty>}" >&2
    exit 1
  fi

  observed_source_run='null'
  observed_source_job='null'
  observed_source_artifact='null'
  observed_release_artifact='null'
  if jq -e --arg key "$key" '.releases[$key] | has("source_run")' "$lock" >/dev/null; then
    source_run_id=$(jq -r --arg key "$key" '.releases[$key].source_run.run_id' "$lock")
    source_run_url=$(jq -r --arg key "$key" '.releases[$key].source_run.workflow_url' "$lock")
    source_run_head=$(jq -r --arg key "$key" '.releases[$key].source_run.head_sha' "$lock")
    source_run_conclusion=$(jq -r --arg key "$key" '.releases[$key].source_run.conclusion' "$lock")
    source_job_id=$(jq -r --arg key "$key" '.releases[$key].source_run.job_id' "$lock")
    source_job_name=$(jq -r --arg key "$key" '.releases[$key].source_run.job_name' "$lock")
    source_job_url=$(jq -r --arg key "$key" '.releases[$key].source_run.job_url' "$lock")
    source_artifact_ids=$(jq -c --arg key "$key" '.releases[$key].source_run.artifact_ids // []' "$lock")
    source_run_json="$output/$key.source-run.json"
    source_job_json="$output/$key.source-job.json"
    source_artifacts_json="$output/$key.source-artifacts.json"
    gh api "repos/$repo/actions/runs/$source_run_id" > "$source_run_json"
    gh api "repos/$repo/actions/jobs/$source_job_id" > "$source_job_json"
  gh api "repos/$repo/actions/runs/$source_run_id/artifacts?per_page=100" > "$source_artifacts_json"
    jq -e --argjson run_id "$source_run_id" --arg url "$source_run_url" --arg head "$source_run_head" --arg conclusion "$source_run_conclusion" \
      '.id==$run_id and .html_url==$url and .head_sha==$head and .status=="completed" and .conclusion==$conclusion' "$source_run_json" >/dev/null
    jq -e --argjson job_id "$source_job_id" --argjson run_id "$source_run_id" --arg name "$source_job_name" --arg url "$source_job_url" --arg head "$source_run_head" \
      '.id==$job_id and .run_id==$run_id and .name==$name and .html_url==$url and .head_sha==$head and .status=="completed" and .conclusion=="success"' "$source_job_json" >/dev/null
    jq -e --argjson ids "$source_artifact_ids" '[.artifacts[].id] == $ids' "$source_artifacts_json" >/dev/null
    observed_source_run=$(jq -S --argjson id "$source_run_id" '. | {id,run_number,event,status,conclusion,head_sha,html_url,workflow_id,workflow_name}' "$source_run_json")
    observed_source_job=$(jq -S --argjson id "$source_job_id" '. | {id,run_id,name,status,conclusion,head_sha,html_url}' "$source_job_json")
  fi

  observed_release_run='null'
  observed_release_job='null'
  if jq -e --arg key "$key" '.releases[$key] | has("release_run")' "$lock" >/dev/null; then
    release_run_id=$(jq -r --arg key "$key" '.releases[$key].release_run.run_id' "$lock")
    release_run_url=$(jq -r --arg key "$key" '.releases[$key].release_run.workflow_url' "$lock")
    release_run_head=$(jq -r --arg key "$key" '.releases[$key].release_run.head_sha' "$lock")
    release_run_conclusion=$(jq -r --arg key "$key" '.releases[$key].release_run.conclusion' "$lock")
    release_job_id=$(jq -r --arg key "$key" '.releases[$key].release_run.job_id' "$lock")
    release_job_name=$(jq -r --arg key "$key" '.releases[$key].release_run.job_name' "$lock")
    release_job_url=$(jq -r --arg key "$key" '.releases[$key].release_run.job_url' "$lock")
    release_run_json="$output/$key.release-run.json"
    release_job_json="$output/$key.release-job.json"
    gh api "repos/$repo/actions/runs/$release_run_id" > "$release_run_json"
    gh api "repos/$repo/actions/jobs/$release_job_id" > "$release_job_json"
    jq -e --argjson run_id "$release_run_id" --arg url "$release_run_url" --arg head "$release_run_head" --arg conclusion "$release_run_conclusion" \
      '.id==$run_id and .html_url==$url and .head_sha==$head and .status=="completed" and .conclusion==$conclusion' "$release_run_json" >/dev/null
    jq -e --argjson job_id "$release_job_id" --argjson run_id "$release_run_id" --arg name "$release_job_name" --arg url "$release_job_url" --arg head "$release_run_head" \
      '.id==$job_id and .run_id==$run_id and .name==$name and .html_url==$url and .head_sha==$head and .status=="completed" and .conclusion=="success"' "$release_job_json" >/dev/null
    observed_release_run=$(jq -S '. | {id,run_number,event,status,conclusion,head_sha,html_url,workflow_id,workflow_name}' "$release_run_json")
    observed_release_job=$(jq -S '. | {id,run_id,name,status,conclusion,head_sha,html_url}' "$release_job_json")
  fi

  if jq -e --arg key "$key" '.releases[$key] | has("source_artifact")' "$lock" >/dev/null; then
    source_artifact_run_id=$(jq -r --arg key "$key" '.releases[$key].source_artifact.run_id' "$lock")
    source_artifact_id=$(jq -r --arg key "$key" '.releases[$key].source_artifact.artifact_id' "$lock")
    source_artifact_name=$(jq -r --arg key "$key" '.releases[$key].source_artifact.name' "$lock")
    source_artifact_size=$(jq -r --arg key "$key" '.releases[$key].source_artifact.size_bytes' "$lock")
    source_artifact_sha=$(jq -r --arg key "$key" '.releases[$key].source_artifact.sha256' "$lock")
    source_artifact_json="$output/$key.source-artifact.json"
    gh api "repos/$repo/actions/artifacts/$source_artifact_id" > "$source_artifact_json"
    jq -e --argjson run_id "$source_artifact_run_id" --argjson artifact_id "$source_artifact_id" --arg name "$source_artifact_name" --argjson size "$source_artifact_size" --arg sha "$source_artifact_sha" \
      '.id==$artifact_id and .workflow_run.id==$run_id and .name==$name and .size_in_bytes==$size and .digest==$sha and .expired==false' "$source_artifact_json" >/dev/null
    observed_source_artifact=$(jq -S --argjson id "$source_artifact_id" '. | {id,name,size_in_bytes,expired,digest,workflow_run}' "$source_artifact_json")
  fi

  if jq -e --arg key "$key" '.releases[$key] | has("release_artifact")' "$lock" >/dev/null; then
    release_artifact_run_id=$(jq -r --arg key "$key" '.releases[$key].release_artifact.run_id' "$lock")
    release_artifact_id=$(jq -r --arg key "$key" '.releases[$key].release_artifact.artifact_id' "$lock")
    release_artifact_name=$(jq -r --arg key "$key" '.releases[$key].release_artifact.name' "$lock")
    release_artifact_size=$(jq -r --arg key "$key" '.releases[$key].release_artifact.size_bytes' "$lock")
    release_artifact_sha=$(jq -r --arg key "$key" '.releases[$key].release_artifact.sha256' "$lock")
    release_artifact_json="$output/$key.release-artifact.json"
    gh api "repos/$repo/actions/artifacts/$release_artifact_id" > "$release_artifact_json"
    jq -e --argjson run_id "$release_artifact_run_id" --argjson artifact_id "$release_artifact_id" --arg name "$release_artifact_name" --argjson size "$release_artifact_size" --arg sha "$release_artifact_sha" \
      '.id==$artifact_id and .workflow_run.id==$run_id and .name==$name and .size_in_bytes==$size and .digest==$sha and .expired==false' "$release_artifact_json" >/dev/null
    observed_release_artifact=$(jq -S --argjson id "$release_artifact_id" '. | {id,name,size_in_bytes,expired,digest,workflow_run}' "$release_artifact_json")
  fi

  if jq -e --arg key "$key" '.releases[$key] | has("post_main_validation")' "$lock" >/dev/null; then
    post_run_id=$(jq -r --arg key "$key" '.releases[$key].post_main_validation.run_id' "$lock")
    post_run_url=$(jq -r --arg key "$key" '.releases[$key].post_main_validation.workflow_url' "$lock")
    post_run_head=$(jq -r --arg key "$key" '.releases[$key].post_main_validation.head_sha' "$lock")
    post_run_conclusion=$(jq -r --arg key "$key" '.releases[$key].post_main_validation.conclusion' "$lock")
    post_job_id=$(jq -r --arg key "$key" '.releases[$key].post_main_validation.job_id' "$lock")
    post_job_name=$(jq -r --arg key "$key" '.releases[$key].post_main_validation.job_name' "$lock")
    post_job_url=$(jq -r --arg key "$key" '.releases[$key].post_main_validation.job_url' "$lock")
    post_run_json="$output/$key.post-main-run.json"
    post_job_json="$output/$key.post-main-job.json"
    gh api "repos/$repo/actions/runs/$post_run_id" > "$post_run_json"
    gh api "repos/$repo/actions/jobs/$post_job_id" > "$post_job_json"
    jq -e --argjson run_id "$post_run_id" --arg url "$post_run_url" --arg head "$post_run_head" --arg conclusion "$post_run_conclusion" \
      '.id==$run_id and .html_url==$url and .head_sha==$head and .status=="completed" and .conclusion==$conclusion' "$post_run_json" >/dev/null
    jq -e --argjson job_id "$post_job_id" --argjson run_id "$post_run_id" --arg name "$post_job_name" --arg url "$post_job_url" --arg head "$post_run_head" \
      '.id==$job_id and .run_id==$run_id and .name==$name and .html_url==$url and .head_sha==$head and .status=="completed" and .conclusion=="success"' "$post_job_json" >/dev/null
  fi

  if jq -e --arg key "$key" '.releases[$key] | has("pre_merge_validation")' "$lock" >/dev/null; then
    pre_run_id=$(jq -r --arg key "$key" '.releases[$key].pre_merge_validation.run_id' "$lock")
    pre_run_url=$(jq -r --arg key "$key" '.releases[$key].pre_merge_validation.workflow_url' "$lock")
    pre_run_head=$(jq -r --arg key "$key" '.releases[$key].pre_merge_validation.head_sha' "$lock")
    pre_run_conclusion=$(jq -r --arg key "$key" '.releases[$key].pre_merge_validation.conclusion' "$lock")
    pre_job_id=$(jq -r --arg key "$key" '.releases[$key].pre_merge_validation.job_id' "$lock")
    pre_job_name=$(jq -r --arg key "$key" '.releases[$key].pre_merge_validation.job_name' "$lock")
    pre_job_url=$(jq -r --arg key "$key" '.releases[$key].pre_merge_validation.job_url' "$lock")
    pre_run_json="$output/$key.pre-merge-run.json"
    pre_job_json="$output/$key.pre-merge-job.json"
    gh api "repos/$repo/actions/runs/$pre_run_id" > "$pre_run_json"
    gh api "repos/$repo/actions/jobs/$pre_job_id" > "$pre_job_json"
    jq -e --argjson run_id "$pre_run_id" --arg url "$pre_run_url" --arg head "$pre_run_head" --arg conclusion "$pre_run_conclusion" \
      '.id==$run_id and .html_url==$url and .head_sha==$head and .status=="completed" and .conclusion==$conclusion' "$pre_run_json" >/dev/null
    jq -e --argjson job_id "$pre_job_id" --argjson run_id "$pre_run_id" --arg name "$pre_job_name" --arg url "$pre_job_url" --arg head "$pre_run_head" \
      '.id==$job_id and .run_id==$run_id and .name==$name and .html_url==$url and .head_sha==$head and .status=="completed" and .conclusion=="success"' "$pre_job_json" >/dev/null
  fi

  mkdir -p "$output/$key/assets"
  asset_results="$output/$key/assets.ndjson"
  : > "$asset_results"
  asset_count=$(jq -r --arg key "$key" '.releases[$key].assets | length' "$lock")
  for index in $(seq 0 $((asset_count - 1))); do
    name=$(jq -r --arg key "$key" --argjson index "$index" '.releases[$key].assets[$index].name' "$lock")
    asset_id=$(jq -r --arg key "$key" --argjson index "$index" '.releases[$key].assets[$index].id // empty' "$lock")
    size=$(jq -r --arg key "$key" --argjson index "$index" '.releases[$key].assets[$index].size_bytes' "$lock")
    sha=$(jq -r --arg key "$key" --argjson index "$index" '.releases[$key].assets[$index].sha256' "$lock")
    download_url=$(jq -r --arg key "$key" --argjson index "$index" '.releases[$key].assets[$index].download_url' "$lock")
    if [ -n "$asset_id" ]; then
      jq -e --arg name "$name" --arg digest "$sha" --arg url "$download_url" --argjson size "$size" --argjson asset_id "$asset_id" \
        '[.assets[] | select(.id==$asset_id and .name==$name and .size==$size and .digest==$digest and .browser_download_url==$url)] | length == 1' \
        "$release_json" >/dev/null
    else
      jq -e --arg name "$name" --arg digest "$sha" --arg url "$download_url" --argjson size "$size" \
        '[.assets[] | select(.name==$name and .size==$size and .digest==$digest and .browser_download_url==$url)] | length == 1' \
        "$release_json" >/dev/null
    fi
    curl --fail --location --retry 3 --silent --show-error "$download_url" -o "$output/$key/assets/$name"
    actual_size=$(wc -c < "$output/$key/assets/$name" | tr -d ' ')
    actual_sha="sha256:$(sha256sum "$output/$key/assets/$name" | awk '{print $1}')"
    test "$actual_size" -eq "$size"
    test "$actual_sha" = "$sha"
    jq -S -n --arg name "$name" --argjson id "${asset_id:-null}" --argjson size "$size" --arg sha "$sha" --arg url "$download_url" \
      '{id:$id,name:$name,size_bytes:$size,sha256:$sha,download_url:$url,verified:true}' >> "$asset_results"
  done
  manifest_name=$(jq -r --arg key "$key" '.releases[$key].manifest.name // empty' "$lock")
  if [ -n "$manifest_name" ]; then
    manifest_path="$output/$key/assets/$manifest_name"
    if jq -e --arg key "$key" '.releases[$key] | has("source_artifact") and has("release_manifest_lock")' "$lock" >/dev/null; then
      source_run_id=$(jq -r --arg key "$key" '.releases[$key].source_artifact.run_id' "$lock")
      source_artifact_id=$(jq -r --arg key "$key" '.releases[$key].source_artifact.artifact_id' "$lock")
      source_artifact_name=$(jq -r --arg key "$key" '.releases[$key].source_artifact.name' "$lock")
      source_artifact_size=$(jq -r --arg key "$key" '.releases[$key].source_artifact.size_bytes' "$lock")
      source_artifact_sha=$(jq -r --arg key "$key" '.releases[$key].source_artifact.sha256' "$lock")
      jq -e --arg target "$target" --argjson run_id "$source_run_id" --argjson artifact_id "$source_artifact_id" \
        --arg name "$source_artifact_name" --argjson size "$source_artifact_size" --arg sha "$source_artifact_sha" \
        '.provenance.merge_commit_sha==$target and .provenance.post_main_workflow_run_id==$run_id and
         .provenance.actions_artifact_id==$artifact_id and .provenance.actions_artifact_name==$name and
         .provenance.actions_artifact_size_bytes==$size and .provenance.actions_artifact_digest==$sha' \
        "$manifest_path" >/dev/null
    fi
    if jq -e --arg key "$key" '.releases[$key] | has("release_manifest_lock")' "$lock" >/dev/null; then
      lock_path=$(jq -r --arg key "$key" '.releases[$key].release_manifest_lock.path' "$lock")
      lock_sha=$(jq -r --arg key "$key" '.releases[$key].release_manifest_lock.sha256' "$lock")
      jq -e --arg path "$lock_path" --arg sha "$lock_sha" \
        '.external_inputs.lock.path==$path and .external_inputs.lock.sha256==$sha' \
        "$manifest_path" >/dev/null
    fi
  fi
  receipt_name=$(jq -r --arg key "$key" '.releases[$key].protocol_receipt.asset_name // empty' "$lock")
  if [ -n "$receipt_name" ]; then
    receipt_path="$output/$key/assets/$receipt_name"
    receipt_expected=$(jq -c --arg key "$key" '.releases[$key].protocol_receipt | del(.asset_name)' "$lock")
    jq -e --argjson expected "$receipt_expected" '. == $expected' "$receipt_path" >/dev/null
  fi
  assets=$(jq -s . "$asset_results")
  fetch_rss=$(cat "$output/$key.fetch-rss")
  jq -S -n \
    --arg state "CLOSED" --arg repository "$repo" --arg tag "$tag" --arg release_url "$release_url" \
    --arg target "$target" --argjson release_id "${release_id:-null}" --arg tag_object_sha "$tag_object" \
    --argjson assets "$assets" --argjson fetch_rss "$fetch_rss" --argjson source_run "$observed_source_run" --argjson source_job "$observed_source_job" \
    --argjson source_artifact "$observed_source_artifact" --argjson release_artifact "$observed_release_artifact" --argjson release_run "$observed_release_run" --argjson release_job "$observed_release_job" \
    '{state:$state,verified:true,repository:$repository,tag:$tag,release_id:$release_id,release_url:$release_url,target_commit_sha:$target,tag_object_sha:$tag_object_sha,source_run:$source_run,source_job:$source_job,source_artifact:$source_artifact,release_artifact:$release_artifact,release_run:$release_run,release_job:$release_job,assets:$assets,fetch:{wall_ms:0,duration_ns:0,peak_rss_kib:$fetch_rss},verify:{wall_ms:0,duration_ns:0,peak_rss_kib:0},reason:""}' \
    > "$output/$key.result.json"
done

failed_release_trigger_results='{}'
for counterexample_id in $(jq -r '(.failed_release_triggers // [])[] | .counterexample_id' "$lock"); do
  repo=$(jq -r --arg id "$counterexample_id" '.failed_release_triggers[] | select(.counterexample_id==$id) | .repository' "$lock")
  tag=$(jq -r --arg id "$counterexample_id" '.failed_release_triggers[] | select(.counterexample_id==$id) | .tag' "$lock")
  release_url=$(jq -r --arg id "$counterexample_id" '.failed_release_triggers[] | select(.counterexample_id==$id) | .release_url' "$lock")
  target=$(jq -r --arg id "$counterexample_id" '.failed_release_triggers[] | select(.counterexample_id==$id) | .target_commit_sha' "$lock")
  expected_tag_object=$(jq -r --arg id "$counterexample_id" '.failed_release_triggers[] | select(.counterexample_id==$id) | .tag_object_sha' "$lock")
  reason=$(jq -r --arg id "$counterexample_id" '.failed_release_triggers[] | select(.counterexample_id==$id) | .reason' "$lock")
  release_json="$output/$counterexample_id.release.json"
  jq -e --arg id "$counterexample_id" '.failed_release_triggers[] | select(.counterexample_id==$id) | .append_only==true and .release_absent==true and .release_api_status==404 and .reason=="FAILED_RELEASE_TRIGGER"' "$lock" >/dev/null
  jq -e --arg url "$release_url" '(.status==404 or .status=="404") and .message=="Not Found"' "$release_json" >/dev/null

  remote_refs=""
  for attempt in 1 2 3; do
    if remote_refs=$(git ls-remote "https://github.com/$repo.git" "refs/tags/$tag" "refs/tags/$tag^{}"); then
      tag_target=$(awk -v peeled="refs/tags/$tag^{}" '$2==peeled {print $1}' <<< "$remote_refs")
      tag_object=$(awk -v direct="refs/tags/$tag" '$2==direct {print $1}' <<< "$remote_refs")
      break
    fi
    echo "tag lookup attempt $attempt failed for $repo@$tag" >&2
    sleep 1
  done
  test "$tag_target" = "$target"
  test "$tag_object" = "$expected_tag_object"

  run_id=$(jq -r --arg id "$counterexample_id" '.failed_release_triggers[] | select(.counterexample_id==$id) | .failed_run.run_id' "$lock")
  run_url=$(jq -r --arg id "$counterexample_id" '.failed_release_triggers[] | select(.counterexample_id==$id) | .failed_run.run_url' "$lock")
  event=$(jq -r --arg id "$counterexample_id" '.failed_release_triggers[] | select(.counterexample_id==$id) | .failed_run.event' "$lock")
  head_branch=$(jq -r --arg id "$counterexample_id" '.failed_release_triggers[] | select(.counterexample_id==$id) | .failed_run.head_branch' "$lock")
  head_sha=$(jq -r --arg id "$counterexample_id" '.failed_release_triggers[] | select(.counterexample_id==$id) | .failed_run.head_sha' "$lock")
  job_id=$(jq -r --arg id "$counterexample_id" '.failed_release_triggers[] | select(.counterexample_id==$id) | .failed_run.job_id' "$lock")
  job_name=$(jq -r --arg id "$counterexample_id" '.failed_release_triggers[] | select(.counterexample_id==$id) | .failed_run.job_name' "$lock")
  job_url=$(jq -r --arg id "$counterexample_id" '.failed_release_triggers[] | select(.counterexample_id==$id) | .failed_run.job_url' "$lock")
  run_json="$output/$counterexample_id.failed-run.json"
  job_json="$output/$counterexample_id.failed-job.json"
  gh api "repos/$repo/actions/runs/$run_id" > "$run_json"
  gh api "repos/$repo/actions/jobs/$job_id" > "$job_json"
  jq -e --argjson run_id "$run_id" --arg url "$run_url" --arg event "$event" --arg branch "$head_branch" --arg head "$head_sha" \
    '.id==$run_id and .html_url==$url and .event==$event and .head_branch==$branch and .head_sha==$head and .status=="completed" and .conclusion=="failure"' "$run_json" >/dev/null
  jq -e --argjson job_id "$job_id" --argjson run_id "$run_id" --arg name "$job_name" --arg url "$job_url" --arg head "$head_sha" \
    '.id==$job_id and .run_id==$run_id and .name==$name and .html_url==$url and .head_sha==$head and .status=="completed" and .conclusion=="failure"' "$job_json" >/dev/null
  fetch_rss=$(cat "$output/$counterexample_id.fetch-rss")
  result=$(jq -S -n --arg id "$counterexample_id" --arg repository "$repo" --arg tag "$tag" --arg url "$release_url" --arg target "$target" --arg tag_object_sha "$tag_object" --arg reason "$reason" \
    --argjson run_id "$run_id" --arg run_url "$run_url" --arg event "$event" --arg branch "$head_branch" --arg head "$head_sha" --argjson job_id "$job_id" --arg job_name "$job_name" --arg job_url "$job_url" --argjson fetch_rss "$fetch_rss" \
    '{state:"REFUTED",counterexample:true,verified:true,release_absent:true,repository:$repository,tag:$tag,release_url:$url,target_commit_sha:$target,tag_object_sha:$tag_object_sha,release_api_status:404,failed_run:{run_id:$run_id,run_url:$run_url,event:$event,head_branch:$branch,head_sha:$head,conclusion:"failure",job_id:$job_id,job_name:$job_name,job_url:$job_url},fetch:{wall_ms:0,duration_ns:0,peak_rss_kib:$fetch_rss},reason:$reason}')
  failed_release_trigger_results=$(jq -c --arg id "$counterexample_id" --argjson result "$result" '. + {($id):$result}' <<< "$failed_release_trigger_results")
done

counterexample_results='{}'
for counterexample_id in $(jq -r '(.counterexamples // [])[] | .counterexample_id' "$lock"); do
  repo=$(jq -r --arg id "$counterexample_id" '.counterexamples[] | select(.counterexample_id==$id) | .repository' "$lock")
  tag=$(jq -r --arg id "$counterexample_id" '.counterexamples[] | select(.counterexample_id==$id) | .tag' "$lock")
  release_url=$(jq -r --arg id "$counterexample_id" '.counterexamples[] | select(.counterexample_id==$id) | .release_url' "$lock")
  target=$(jq -r --arg id "$counterexample_id" '.counterexamples[] | select(.counterexample_id==$id) | .target_commit_sha' "$lock")
  expected_tag_object=$(jq -r --arg id "$counterexample_id" '.counterexamples[] | select(.counterexample_id==$id) | .tag_object_sha' "$lock")
  release_id=$(jq -r --arg id "$counterexample_id" '.counterexamples[] | select(.counterexample_id==$id) | .release_id' "$lock")
  reason=$(jq -r --arg id "$counterexample_id" '.counterexamples[] | select(.counterexample_id==$id) | .reason' "$lock")
  historical_release=$(jq -r --arg id "$counterexample_id" '.counterexamples[] | select(.counterexample_id==$id) | (.historical_release // false)' "$lock")
  release_json="$output/$counterexample_id.release.json"
  if [ "$historical_release" = true ]; then
    jq -e --arg id "$counterexample_id" '.counterexamples[] | select(.counterexample_id==$id) | .immutable==false and .append_only==true and .release_absent==true and .release_api_status==404 and .reason=="RELEASE_HISTORY_REWRITE_PROCESS"' "$lock" >/dev/null
    jq -e '.status==404 and .message=="Not Found"' "$release_json" >/dev/null
    run_id=$(jq -r --arg id "$counterexample_id" '.counterexamples[] | select(.counterexample_id==$id) | .failed_release_audit.run_id' "$lock")
    run_url=$(jq -r --arg id "$counterexample_id" '.counterexamples[] | select(.counterexample_id==$id) | .failed_release_audit.run_url' "$lock")
    run_event=$(jq -r --arg id "$counterexample_id" '.counterexamples[] | select(.counterexample_id==$id) | .failed_release_audit.event' "$lock")
    run_branch=$(jq -r --arg id "$counterexample_id" '.counterexamples[] | select(.counterexample_id==$id) | .failed_release_audit.head_branch' "$lock")
    run_head=$(jq -r --arg id "$counterexample_id" '.counterexamples[] | select(.counterexample_id==$id) | .failed_release_audit.head_sha' "$lock")
    job_id=$(jq -r --arg id "$counterexample_id" '.counterexamples[] | select(.counterexample_id==$id) | .failed_release_audit.job_id' "$lock")
    job_name=$(jq -r --arg id "$counterexample_id" '.counterexamples[] | select(.counterexample_id==$id) | .failed_release_audit.job_name' "$lock")
    job_url=$(jq -r --arg id "$counterexample_id" '.counterexamples[] | select(.counterexample_id==$id) | .failed_release_audit.job_url' "$lock")
    run_json="$output/$counterexample_id.historical-run.json"
    job_json="$output/$counterexample_id.historical-job.json"
    artifact_json="$output/$counterexample_id.historical-artifact.json"
    gh api "repos/$repo/actions/runs/$run_id" > "$run_json"
    gh api "repos/$repo/actions/jobs/$job_id" > "$job_json"
    artifact_id=$(jq -r --arg id "$counterexample_id" '.counterexamples[] | select(.counterexample_id==$id) | .failed_release_audit.artifact_id' "$lock")
    gh api "repos/$repo/actions/artifacts/$artifact_id" > "$artifact_json"
    jq -e --argjson run_id "$run_id" --arg url "$run_url" --arg event "$run_event" --arg branch "$run_branch" --arg head "$run_head" \
      '.id==$run_id and .html_url==$url and .event==$event and .head_branch==$branch and .head_sha==$head and .status=="completed" and .conclusion=="failure"' "$run_json" >/dev/null
    jq -e --argjson job_id "$job_id" --argjson run_id "$run_id" --arg name "$job_name" --arg url "$job_url" --arg head "$run_head" \
      '.id==$job_id and .run_id==$run_id and .name==$name and .html_url==$url and .head_sha==$head and .status=="completed" and .conclusion=="failure"' "$job_json" >/dev/null
    artifact_name=$(jq -r --arg id "$counterexample_id" '.counterexamples[] | select(.counterexample_id==$id) | .failed_release_audit.artifact_name' "$lock")
    artifact_size=$(jq -r --arg id "$counterexample_id" '.counterexamples[] | select(.counterexample_id==$id) | .failed_release_audit.artifact_size_bytes' "$lock")
    artifact_sha=$(jq -r --arg id "$counterexample_id" '.counterexamples[] | select(.counterexample_id==$id) | .failed_release_audit.artifact_sha256' "$lock")
    jq -e --argjson artifact_id "$artifact_id" --argjson run_id "$run_id" --arg name "$artifact_name" --argjson size "$artifact_size" --arg sha "$artifact_sha" \
      '.id==$artifact_id and .workflow_run.id==$run_id and .name==$name and .size_in_bytes==$size and .digest==$sha and .expired==false' "$artifact_json" >/dev/null
    mkdir -p "$output/counterexamples/$counterexample_id/assets"
    asset_results="$output/counterexamples/$counterexample_id/assets.ndjson"
    : > "$asset_results"
    asset_count=$(jq -r --arg id "$counterexample_id" '.counterexamples[] | select(.counterexample_id==$id) | .assets | length' "$lock")
    for index in $(seq 0 $((asset_count - 1))); do
      name=$(jq -r --arg id "$counterexample_id" --argjson index "$index" '.counterexamples[] | select(.counterexample_id==$id) | .assets[$index].name' "$lock")
      asset_id=$(jq -r --arg id "$counterexample_id" --argjson index "$index" '.counterexamples[] | select(.counterexample_id==$id) | .assets[$index].id' "$lock")
      size=$(jq -r --arg id "$counterexample_id" --argjson index "$index" '.counterexamples[] | select(.counterexample_id==$id) | .assets[$index].size_bytes' "$lock")
      sha=$(jq -r --arg id "$counterexample_id" --argjson index "$index" '.counterexamples[] | select(.counterexample_id==$id) | .assets[$index].sha256' "$lock")
      download_url=$(jq -r --arg id "$counterexample_id" --argjson index "$index" '.counterexamples[] | select(.counterexample_id==$id) | .assets[$index].download_url' "$lock")
      jq -S -n --arg name "$name" --argjson id "$asset_id" --argjson size "$size" --arg sha "$sha" --arg url "$download_url" \
        '{id:$id,name:$name,size_bytes:$size,sha256:$sha,download_url:$url,verified:true,historical:true}' >> "$asset_results"
    done
    assets=$(jq -s . "$asset_results")
    fetch_rss=$(cat "$output/$counterexample_id.fetch-rss")
    result=$(jq -S -n --arg repository "$repo" --arg tag "$tag" --arg release_url "$release_url" --arg target "$target" --arg tag_object_sha "$expected_tag_object" --arg reason "$reason" \
      --argjson release_id "$release_id" --argjson assets "$assets" --argjson fetch_rss "$fetch_rss" --argjson run_id "$run_id" --arg run_url "$run_url" --arg event "$run_event" --arg branch "$run_branch" --arg head "$run_head" --argjson job_id "$job_id" --arg job_name "$job_name" --arg job_url "$job_url" \
      '{state:"REFUTED",counterexample:true,verified:true,historical_release:true,release_absent:true,repository:$repository,tag:$tag,release_id:$release_id,release_url:$release_url,target_commit_sha:$target,tag_object_sha:$tag_object_sha,assets:$assets,failed_release:{run_id:$run_id,run_url:$run_url,event:$event,head_branch:$branch,head_sha:$head,conclusion:"failure",job_id:$job_id,job_name:$job_name,job_url:$job_url},fetch:{wall_ms:0,duration_ns:0,peak_rss_kib:$fetch_rss},reason:$reason}')
    counterexample_results=$(jq -c --arg id "$counterexample_id" --argjson result "$result" '. + {($id):$result}' <<< "$counterexample_results")
    continue
  fi
  jq -e --arg id "$counterexample_id" '.counterexamples[] | select(.counterexample_id==$id) | .immutable==false and .append_only==true and (.reason=="RELEASE_API_IMMUTABLE_FALSE" or .reason=="FAILED_RELEASE_IMMUTABILITY" or .reason=="SELF_ASSERTED_IMMUTABILITY_CONTRADICTED_BY_PLATFORM")' "$lock" >/dev/null
  jq -e --arg tag "$tag" --arg url "$release_url" --argjson release_id "$release_id" \
    '.id==$release_id and .tag_name==$tag and .html_url==$url and .draft==false and .prerelease==false and .immutable==false' \
    "$release_json" >/dev/null

  remote_refs=""
  for attempt in 1 2 3; do
    if remote_refs=$(git ls-remote "https://github.com/$repo.git" "refs/tags/$tag" "refs/tags/$tag^{}"); then
      break
    fi
    echo "tag lookup attempt $attempt failed for $repo@$tag" >&2
    sleep 1
  done
  tag_target=$(awk -v direct="refs/tags/$tag" -v peeled="refs/tags/$tag^{}" '$2==peeled {p=$1} $2==direct {d=$1} END {if (p!="") print p; else print d}' <<< "$remote_refs")
  tag_object=$(awk -v direct="refs/tags/$tag" '$2==direct {print $1}' <<< "$remote_refs")
  test "$tag_target" = "$target"
  test "$tag_object" = "$expected_tag_object"

  asset_count=$(jq -r --arg id "$counterexample_id" '.counterexamples[] | select(.counterexample_id==$id) | .assets | length' "$lock")
  test "$(jq -r '.assets|length' "$release_json")" -eq "$asset_count"
  mkdir -p "$output/counterexamples/$counterexample_id/assets"
  asset_results="$output/counterexamples/$counterexample_id/assets.ndjson"
  : > "$asset_results"
  for index in $(seq 0 $((asset_count - 1))); do
    name=$(jq -r --arg id "$counterexample_id" --argjson index "$index" '.counterexamples[] | select(.counterexample_id==$id) | .assets[$index].name' "$lock")
    asset_id=$(jq -r --arg id "$counterexample_id" --argjson index "$index" '.counterexamples[] | select(.counterexample_id==$id) | .assets[$index].id' "$lock")
    size=$(jq -r --arg id "$counterexample_id" --argjson index "$index" '.counterexamples[] | select(.counterexample_id==$id) | .assets[$index].size_bytes' "$lock")
    sha=$(jq -r --arg id "$counterexample_id" --argjson index "$index" '.counterexamples[] | select(.counterexample_id==$id) | .assets[$index].sha256' "$lock")
    download_url=$(jq -r --arg id "$counterexample_id" --argjson index "$index" '.counterexamples[] | select(.counterexample_id==$id) | .assets[$index].download_url' "$lock")
    jq -e --arg name "$name" --arg digest "$sha" --arg url "$download_url" --argjson size "$size" --argjson asset_id "$asset_id" \
      '[.assets[] | select(.id==$asset_id and .name==$name and .size==$size and .digest==$digest and .browser_download_url==$url)] | length == 1' \
      "$release_json" >/dev/null
    curl --fail --location --retry 3 --silent --show-error "$download_url" -o "$output/counterexamples/$counterexample_id/assets/$name"
    actual_size=$(wc -c < "$output/counterexamples/$counterexample_id/assets/$name" | tr -d ' ')
    actual_sha="sha256:$(sha256sum "$output/counterexamples/$counterexample_id/assets/$name" | awk '{print $1}')"
    test "$actual_size" -eq "$size"
    test "$actual_sha" = "$sha"
    jq -S -n --arg name "$name" --argjson id "$asset_id" --argjson size "$size" --arg sha "$sha" --arg url "$download_url" \
      '{id:$id,name:$name,size_bytes:$size,sha256:$sha,download_url:$url,verified:true}' >> "$asset_results"
  done
  assets=$(jq -s . "$asset_results")
  fetch_rss=$(cat "$output/$counterexample_id.fetch-rss")
  result=$(jq -S -n --arg id "$counterexample_id" --arg repository "$repo" --arg tag "$tag" --argjson release_id "$release_id" \
    --arg release_url "$release_url" --arg target "$target" --arg tag_object_sha "$tag_object" --arg reason "$reason" \
    --argjson assets "$assets" --argjson fetch_rss "$fetch_rss" \
    '{state:"REFUTED",counterexample:true,verified:true,repository:$repository,tag:$tag,release_id:$release_id,release_url:$release_url,target_commit_sha:$target,tag_object_sha:$tag_object_sha,assets:$assets,fetch:{wall_ms:0,duration_ns:0,peak_rss_kib:$fetch_rss},reason:$reason}')
  counterexample_results=$(jq -c --arg id "$counterexample_id" --argjson result "$result" '. + {($id):$result}' <<< "$counterexample_results")
done

counterexample_run_results='{}'
for counterexample_id in $(jq -r '(.counterexample_runs // [])[] | .counterexample_id' "$lock"); do
  repo=$(jq -r --arg id "$counterexample_id" '.counterexample_runs[] | select(.counterexample_id==$id) | .repository' "$lock")
  run_id=$(jq -r --arg id "$counterexample_id" '.counterexample_runs[] | select(.counterexample_id==$id) | .run_id' "$lock")
  run_url=$(jq -r --arg id "$counterexample_id" '.counterexample_runs[] | select(.counterexample_id==$id) | .run_url' "$lock")
  event=$(jq -r --arg id "$counterexample_id" '.counterexample_runs[] | select(.counterexample_id==$id) | .event' "$lock")
  head_branch=$(jq -r --arg id "$counterexample_id" '.counterexample_runs[] | select(.counterexample_id==$id) | .head_branch' "$lock")
  head_sha=$(jq -r --arg id "$counterexample_id" '.counterexample_runs[] | select(.counterexample_id==$id) | .head_sha' "$lock")
  job_id=$(jq -r --arg id "$counterexample_id" '.counterexample_runs[] | select(.counterexample_id==$id) | .job_id' "$lock")
  job_name=$(jq -r --arg id "$counterexample_id" '.counterexample_runs[] | select(.counterexample_id==$id) | .job_name' "$lock")
  job_url=$(jq -r --arg id "$counterexample_id" '.counterexample_runs[] | select(.counterexample_id==$id) | .job_url' "$lock")
  run_json="$output/$counterexample_id.run.json"
  job_json="$output/$counterexample_id.job.json"
  jq -e --argjson run_id "$run_id" --arg url "$run_url" --arg event "$event" --arg branch "$head_branch" --arg head "$head_sha" \
    '.id==$run_id and .html_url==$url and .event==$event and .head_branch==$branch and .head_sha==$head and .status=="completed" and .conclusion=="failure"' \
    "$run_json" >/dev/null
  jq -e --argjson job_id "$job_id" --argjson run_id "$run_id" --arg name "$job_name" --arg url "$job_url" --arg head "$head_sha" \
    '.id==$job_id and .run_id==$run_id and .name==$name and .html_url==$url and .head_sha==$head and .status=="completed" and .conclusion=="failure"' \
    "$job_json" >/dev/null
  fetch_rss=$(cat "$output/$counterexample_id.fetch-rss")
  result=$(jq -S -n --arg repository "$repo" --argjson run_id "$run_id" --arg run_url "$run_url" \
    --arg event "$event" --arg branch "$head_branch" --arg head "$head_sha" --argjson job_id "$job_id" --arg job_name "$job_name" --arg job_url "$job_url" --argjson fetch_rss "$fetch_rss" \
    '{counterexample:true,verified:true,repository:$repository,run_id:$run_id,run_url:$run_url,event:$event,head_branch:$branch,head_sha:$head,conclusion:"failure",job_id:$job_id,job_name:$job_name,job_url:$job_url,fetch:{wall_ms:0,duration_ns:0,peak_rss_kib:$fetch_rss}}')
  counterexample_run_results=$(jq -c --arg id "$counterexample_id" --argjson result "$result" '. + {($id):$result}' <<< "$counterexample_run_results")
done
verify_end=$(date +%s%N)

releases='{}'
for key in $(jq -r '.releases | keys[]' "$lock"); do
  result=$(cat "$output/$key.result.json")
  releases=$(jq -c --arg key "$key" --argjson result "$result" '. + {($key):$result}' <<< "$releases")
done

fetch_peak=$(for file in "$output"/*.fetch-rss; do cat "$file"; done | sort -n | tail -1)
fetch_timing=$(measurement "$fetch_start" "$fetch_end" "$fetch_peak")
verify_timing=$(measurement "$verify_start" "$verify_end")
jq -S -n \
  --arg schema "gooo/self-improvement-portfolio/release-verification/v1" \
  --argjson releases "$releases" --argjson counterexamples "$counterexample_results" --argjson counterexample_runs "$counterexample_run_results" --argjson failed_release_triggers "$failed_release_trigger_results" --argjson fetch "$fetch_timing" --argjson verify "$verify_timing" \
  '{schema:$schema,releases:$releases,counterexamples:$counterexamples,counterexample_runs:$counterexample_runs,failed_release_triggers:$failed_release_triggers,summary:{total:($releases|length),verified:([ $releases[] | select(.state=="CLOSED") ]|length),unknown:([ $releases[] | select(.state=="UNKNOWN") ]|length),refuted:([ $releases[] | select(.state=="REFUTED") ]|length)},timing:{fetch:$fetch,verify:$verify,report:{wall_ms:0,duration_ns:0,peak_rss_kib:0}}}' \
  > "$output/verification.json"
