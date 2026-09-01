#!/usr/bin/env bash
set -Eeuo pipefail

if [ "$#" -ne 3 ]; then
  echo "usage: verify-capability-evidence-registry.sh LOCK_JSON ASSESSMENT_JSON OUTPUT_DIR" >&2
  exit 64
fi

lock=$(realpath "$1")
assessment=$(realpath "$2")
output=$(realpath -m "$3")
repository=$(realpath .)
case "$output" in
  "$repository"|"$repository"/*)
    echo "registry output must be outside the input repository" >&2
    exit 65
    ;;
esac
mkdir -p "$output"

command -v gh >/dev/null
command -v curl >/dev/null
command -v jq >/dev/null
command -v sha256sum >/dev/null

jq -e '
  .schema == "gooo/non-completeness/capability-evidence-registry/lock/v1" and
  .registry_id == "non-completeness-capability-evidence-registry-v1" and
  .entry_count == 17 and (.entries|length) == 17 and
  (.entries|map(.entry_id)|length) == (.entries|map(.entry_id)|unique|length) and
  (.lineage|length) == 7 and
  (.lineage|map(.historical_entry_id)|sort) == ["adoption-regression-v0.1.0","counterexample-memory-v0.1.0","evaluator-lineage-v0.1.0","improvement-selector-v0.1.0","receipt-schema-migration-v0.1.1","receipt-schema-migration-v0.2.2","semantic-observer-v0.1.0"] and
  (.["lineage"] | map(select(.historical_entry_id == "receipt-schema-migration-v0.1.1" and .successor_entry_id == "receipt-schema-migration-v0.2.2" and .historical_state == "CLOSED" and .successor_state == "CLOSED" and .transition == "CLOSED_TO_CLOSED_SUCCESSOR")) | length) == 1 and
  (.lineage | map(select(.historical_entry_id == "receipt-schema-migration-v0.2.2" and .successor_entry_id == "receipt-schema-migration-v0.3.1" and .historical_state == "CLOSED" and .successor_state == "CLOSED" and .transition == "CLOSED_TO_CLOSED_SUCCESSOR")) | length) == 1 and
  (.lineage | map(select(.historical_entry_id == "semantic-observer-v0.1.0" and .successor_entry_id == "semantic-observer-v0.1.1" and .historical_state == "REFUTED" and .successor_state == "CLOSED" and .transition == "REFUTED_TO_CLOSED")) | length) == 1 and
  (.lineage | map(select(.historical_entry_id == "adoption-regression-v0.1.0" and .successor_entry_id == "adoption-regression-v0.1.1" and .historical_state == "CLOSED" and .successor_state == "CLOSED" and .transition == "CLOSED_TO_CLOSED_SUCCESSOR")) | length) == 1 and
  ((.lineage | map(select(.transition == "REFUTED_TO_CLOSED")) | length) == 4) and
  (.frontier_additions == ["receipt-schema-migration-v0.1.1","receipt-schema-migration-v0.2.2","receipt-schema-migration-v0.3.1","adoption-transaction-v0.1.0","self-repair-example-v0.1.0","semantic-observer-v0.1.0","semantic-observer-v0.1.1","adoption-regression-v0.1.0","adoption-regression-v0.1.1"]) and
  .policy.separate_from_portfolio_denominator == true and
  .policy.aggregate_percentage == false and .policy.aggregate_score == false
' "$lock" >/dev/null
jq -e '
  .schema == "gooo/non-completeness/capability-evidence-registry/assessment/v1" and
  .registry_id == "non-completeness-capability-evidence-registry-v1" and
  .entry_count == 18 and (.entries|length) == 18 and
  (.entries|map(.entry_id)|length) == (.entries|map(.entry_id)|unique|length) and
  (.lineage|length) == 7 and
  (.lineage|map(.historical_entry_id)|sort) == ["adoption-regression-v0.1.0","counterexample-memory-v0.1.0","evaluator-lineage-v0.1.0","improvement-selector-v0.1.0","receipt-schema-migration-v0.1.1","receipt-schema-migration-v0.2.2","semantic-observer-v0.1.0"] and
  (.["lineage"] | map(select(.historical_entry_id == "receipt-schema-migration-v0.1.1" and .successor_entry_id == "receipt-schema-migration-v0.2.2" and .historical_state == "CLOSED" and .successor_state == "CLOSED" and .transition == "CLOSED_TO_CLOSED_SUCCESSOR")) | length) == 1 and
  (.lineage | map(select(.historical_entry_id == "receipt-schema-migration-v0.2.2" and .successor_entry_id == "receipt-schema-migration-v0.3.1" and .historical_state == "CLOSED" and .successor_state == "CLOSED" and .transition == "CLOSED_TO_CLOSED_SUCCESSOR")) | length) == 1 and
  (.lineage | map(select(.historical_entry_id == "semantic-observer-v0.1.0" and .successor_entry_id == "semantic-observer-v0.1.1" and .historical_state == "REFUTED" and .successor_state == "CLOSED" and .transition == "REFUTED_TO_CLOSED")) | length) == 1 and
  (.lineage | map(select(.historical_entry_id == "adoption-regression-v0.1.0" and .successor_entry_id == "adoption-regression-v0.1.1" and .historical_state == "CLOSED" and .successor_state == "CLOSED" and .transition == "CLOSED_TO_CLOSED_SUCCESSOR")) | length) == 1 and
  ((.lineage | map(select(.transition == "REFUTED_TO_CLOSED")) | length) == 4) and
  (.frontier_additions == ["receipt-schema-migration-v0.1.1","receipt-schema-migration-v0.2.2","receipt-schema-migration-v0.3.1","adoption-transaction-v0.1.0","self-repair-example-v0.1.0","semantic-observer-v0.1.0","semantic-observer-v0.1.1","adoption-regression-v0.1.0","adoption-regression-v0.1.1","structural-ledger-append-planner-v0.2.0"]) and
  all(.entries[]; .state == "CLOSED" or .state == "UNKNOWN" or .state == "REFUTED")
' "$assessment" >/dev/null

measurement() {
  local start=$1
  local end=$2
  jq -n --argjson wall "$(( (end - start) / 1000000 ))" --argjson raw "$((end - start))" \
    '{wall_ms:$wall,duration_ns:$raw}'
}

mark_unknown() {
  if [ "$state" = "CLOSED" ]; then
    state="UNKNOWN"
    reason=$1
  fi
}

mark_refuted() {
  state="REFUTED"
  reason=$1
}

verification_start=$(date +%s%N)
for entry_id in $(jq -r '.entries[].entry_id' "$lock"); do
  repo=$(jq -r --arg id "$entry_id" '.entries[] | select(.entry_id==$id) | .repository' "$lock")
  tag=$(jq -r --arg id "$entry_id" '.entries[] | select(.entry_id==$id) | .tag' "$lock")
  release_url=$(jq -r --arg id "$entry_id" '.entries[] | select(.entry_id==$id) | .release_url' "$lock")
  target=$(jq -r --arg id "$entry_id" '.entries[] | select(.entry_id==$id) | .target_commit_sha' "$lock")
  release_json="$output/$entry_id.release.json"
  state="CLOSED"
  reason=""
  observed_release='{}'
  observed_graphql='{}'
  observed_target='null'
  observed_source_run='null'
  observed_source_artifact='null'
  observed_adoption_proposal='null'
  asset_results="$output/$entry_id.assets.ndjson"
  : > "$asset_results"

  if ! gh api "repos/$repo/releases/tags/$tag" > "$release_json" 2> "$output/$entry_id.release.error"; then
    state="UNKNOWN"
    reason="RELEASE_API_UNAVAILABLE"
    jq -S -n --arg id "$entry_id" --arg state "$state" --arg reason "$reason" \
      '{entry_id:$id,state:$state,observed:{},verified_assets:[],reason:$reason}' \
      > "$output/$entry_id.result.json"
    continue
  fi

  observed_release=$(jq -S '{tag_name,html_url,draft,prerelease,immutable,asset_count:(.assets|length)}' "$release_json")
  immutable_required=$(jq -r --arg id "$entry_id" '.entries[] | select(.entry_id==$id) | .immutable_required' "$lock")
  if ! jq -e --arg tag "$tag" --arg url "$release_url" --argjson immutable "$immutable_required" \
    '.tag_name==$tag and .html_url==$url and .draft==false and .prerelease==false and .immutable==$immutable' \
    "$release_json" >/dev/null; then
    mark_refuted "RELEASE_API_METADATA_MISMATCH"
  fi

  if jq -e --arg id "$entry_id" '.entries[] | select(.entry_id==$id) | (.api_surfaces.graphql // false) == true' "$lock" >/dev/null; then
    graphql_owner="${repo%%/*}"
    graphql_name="${repo#*/}"
    graphql_json="$output/$entry_id.graphql.json"
    if ! gh api graphql -F owner="$graphql_owner" -F name="$graphql_name" -F tag="$tag" \
      -f query='query($owner:String!,$name:String!,$tag:String!){repository(owner:$owner,name:$name){release(tagName:$tag){tagName,isDraft,isPrerelease,url,tagCommit{oid}}}}' \
      > "$graphql_json" 2> "$output/$entry_id.graphql.error"; then
      mark_unknown "GRAPHQL_RELEASE_API_UNAVAILABLE"
    else
      observed_graphql=$(jq -S '.data.repository.release // {} | {tag_name:.tagName,is_draft:.isDraft,is_prerelease:.isPrerelease,url:.url,tag_commit_sha:(.tagCommit.oid // null)}' "$graphql_json")
      if ! jq -e --arg tag "$tag" --arg url "$release_url" --arg target "$target" \
        '.data.repository.release != null and .data.repository.release.tagName==$tag and .data.repository.release.url==$url and .data.repository.release.isDraft==false and .data.repository.release.isPrerelease==false and .data.repository.release.tagCommit.oid==$target' \
        "$graphql_json" >/dev/null; then
        mark_refuted "GRAPHQL_RELEASE_METADATA_MISMATCH"
      fi
    fi
  fi

  tag_ref="$output/$entry_id.tag-ref.json"
  tag_target_json="$output/$entry_id.tag-target.json"
  if ! gh api "repos/$repo/git/ref/tags/$tag" > "$tag_ref" 2> "$output/$entry_id.tag-ref.error"; then
    mark_unknown "TAG_REF_UNAVAILABLE"
  else
    ref_type=$(jq -r '.object.type // empty' "$tag_ref")
    ref_sha=$(jq -r '.object.sha // empty' "$tag_ref")
    resolved_target="$ref_sha"
    if [ "$ref_type" = "tag" ] && [ -n "$ref_sha" ]; then
      if ! gh api "repos/$repo/git/tags/$ref_sha" > "$tag_target_json" 2> "$output/$entry_id.tag-target.error"; then
        resolved_target=""
        mark_unknown "TAG_OBJECT_UNAVAILABLE"
      else
        resolved_target=$(jq -r '.object.sha // empty' "$tag_target_json")
      fi
    fi
    if [ -z "$resolved_target" ]; then
      mark_unknown "TAG_TARGET_UNAVAILABLE"
    else
      observed_target=$(jq -n --arg type "$ref_type" --arg object "$ref_sha" --arg target "$resolved_target" \
        '{ref_type:$type,ref_object_sha:$object,resolved_target_sha:$target}')
      if [ "$resolved_target" != "$target" ]; then
        mark_refuted "TAG_TARGET_MISMATCH"
      fi
    fi
  fi

  expected_asset_count=$(jq -r --arg id "$entry_id" '.entries[] | select(.entry_id==$id) | (.release_assets_expected // (.assets|length))' "$lock")
  locked_asset_count=$(jq -r --arg id "$entry_id" '.entries[] | select(.entry_id==$id) | (.assets|length)' "$lock")
  if [ "$expected_asset_count" -ne "$locked_asset_count" ]; then
    mark_refuted "REGISTRY_ASSET_COUNT_DECLARATION_MISMATCH"
  fi
  actual_asset_count=$(jq -r '.assets|length' "$release_json")
  if [ "$actual_asset_count" -ne "$expected_asset_count" ]; then
    mark_refuted "RELEASE_ASSET_COUNT_MISMATCH"
  fi
  mkdir -p "$output/$entry_id/assets"
  if [ "$expected_asset_count" -gt 0 ]; then
    for index in $(seq 0 $((expected_asset_count - 1))); do
      name=$(jq -r --arg id "$entry_id" --argjson index "$index" '.entries[] | select(.entry_id==$id) | .assets[$index].name' "$lock")
      size=$(jq -r --arg id "$entry_id" --argjson index "$index" '.entries[] | select(.entry_id==$id) | .assets[$index].size_bytes' "$lock")
      sha=$(jq -r --arg id "$entry_id" --argjson index "$index" '.entries[] | select(.entry_id==$id) | .assets[$index].sha256' "$lock")
      download_url=$(jq -r --arg id "$entry_id" --argjson index "$index" '.entries[] | select(.entry_id==$id) | .assets[$index].download_url' "$lock")
      if ! jq -e --arg name "$name" --arg digest "$sha" --arg url "$download_url" --argjson size "$size" \
        '[.assets[] | select(.name==$name and .size==$size and .digest==$digest and .browser_download_url==$url)] | length == 1' \
        "$release_json" >/dev/null; then
        mark_refuted "RELEASE_ASSET_API_MISMATCH"
        continue
      fi
      asset_path="$output/$entry_id/assets/$index.bin"
      if ! curl --fail --location --retry 3 --silent --show-error "$download_url" -o "$asset_path"; then
        mark_unknown "RELEASE_ASSET_DOWNLOAD_UNAVAILABLE"
        continue
      fi
      actual_size=$(wc -c < "$asset_path" | tr -d ' ')
      actual_sha="sha256:$(sha256sum "$asset_path" | awk '{print $1}')"
      if [ "$actual_size" -ne "$size" ] || [ "$actual_sha" != "$sha" ]; then
        mark_refuted "RELEASE_ASSET_DIGEST_MISMATCH"
        continue
      fi
      jq -S -n --arg name "$name" --argjson size "$size" --arg sha "$sha" --arg url "$download_url" \
        '{name:$name,size_bytes:$size,sha256:$sha,download_url:$url,verified:true}' >> "$asset_results"
    done
  fi

  if jq -e --arg id "$entry_id" '.entries[] | select(.entry_id==$id) | has("adoption_proposal")' "$lock" >/dev/null; then
    proposal_asset_name=$(jq -r --arg id "$entry_id" '.entries[] | select(.entry_id==$id) | .adoption_proposal.asset_name' "$lock")
    proposal_path=$(jq -r --arg id "$entry_id" '.entries[] | select(.entry_id==$id) | .adoption_proposal.path' "$lock")
    proposal_sha=$(jq -r --arg id "$entry_id" '.entries[] | select(.entry_id==$id) | .adoption_proposal.sha256' "$lock")
    proposal_declared_digest=$(jq -r --arg id "$entry_id" '.entries[] | select(.entry_id==$id) | .adoption_proposal.declared_proposal_digest' "$lock")
    proposal_index=$(jq -r --arg id "$entry_id" --arg name "$proposal_asset_name" '.entries[] | select(.entry_id==$id) | .assets | to_entries[] | select(.value.name==$name) | .key' "$lock")
    if [ -z "$proposal_index" ] || [ ! -f "$output/$entry_id/assets/$proposal_index.bin" ]; then
      mark_unknown "ADOPTION_PROPOSAL_ASSET_UNAVAILABLE"
    else
      proposal_asset_path="$output/$entry_id/assets/$proposal_index.bin"
      proposal_json="$output/$entry_id.adoption-proposal.json"
      if ! tar --no-xattrs -xOzf "$proposal_asset_path" "$proposal_path" > "$proposal_json" 2> "$output/$entry_id.adoption-proposal.error"; then
        proposal_basename=${proposal_path##*/}
        proposal_member=$(tar -tzf "$proposal_asset_path" | awk -v base="$proposal_basename" '($0 == base || $0 ~ ("/" base "$")) {print}')
        if [ -z "$proposal_member" ] || ! tar --no-xattrs -xOzf "$proposal_asset_path" "$proposal_member" > "$proposal_json" 2>> "$output/$entry_id.adoption-proposal.error"; then
          mark_refuted "ADOPTION_PROPOSAL_CONTENT_UNAVAILABLE"
        fi
      fi
      if [ "$state" != "REFUTED" ] || [ -s "$proposal_json" ]; then
        if [ ! -s "$proposal_json" ]; then
          mark_refuted "ADOPTION_PROPOSAL_CONTENT_UNAVAILABLE"
        else
          actual_proposal_sha="sha256:$(sha256sum "$proposal_json" | awk '{print $1}')"
          observed_adoption_proposal=$(jq -S -n --arg path "$proposal_path" --arg sha "$actual_proposal_sha" \
            --arg declared "$(jq -r '.proposal_digest // empty' "$proposal_json")" \
            '{path:$path,sha256:$sha,declared_proposal_digest:$declared}')
          if [ "$actual_proposal_sha" != "$proposal_sha" ] || \
            ! jq -e --arg digest "$proposal_declared_digest" '.proposal_digest==$digest' "$proposal_json" >/dev/null; then
            mark_refuted "ADOPTION_PROPOSAL_DIGEST_MISMATCH"
          fi
        fi
      else
        :
      fi
    fi
  fi

  if jq -e --arg id "$entry_id" '.entries[] | select(.entry_id==$id) | has("source_run")' "$lock" >/dev/null; then
    source_run_id=$(jq -r --arg id "$entry_id" '.entries[] | select(.entry_id==$id) | .source_run.run_id' "$lock")
    source_run_json="$output/$entry_id.source-run.json"
    if ! gh api "repos/$repo/actions/runs/$source_run_id" > "$source_run_json" 2> "$output/$entry_id.source-run.error"; then
      mark_unknown "SOURCE_RUN_API_UNAVAILABLE"
    else
      observed_source_run=$(jq -S '{id,head_sha,conclusion,html_url}' "$source_run_json")
      source_run_url=$(jq -r --arg id "$entry_id" '.entries[] | select(.entry_id==$id) | .source_run.workflow_url' "$lock")
      source_run_expected_head=$(jq -r --arg id "$entry_id" '.entries[] | select(.entry_id==$id) | (.source_run.expected_head_sha // .target_commit_sha)' "$lock")
      if ! jq -e --arg sha "$source_run_expected_head" --argjson id "$source_run_id" --arg url "$source_run_url" \
        '.id==$id and .head_sha==$sha and .html_url==$url and .conclusion=="success"' "$source_run_json" >/dev/null; then
        mark_refuted "SOURCE_RUN_PROVENANCE_MISMATCH"
      fi
    fi
  fi

  if jq -e --arg id "$entry_id" '.entries[] | select(.entry_id==$id) | has("source_artifact")' "$lock" >/dev/null; then
    source_artifact_id=$(jq -r --arg id "$entry_id" '.entries[] | select(.entry_id==$id) | .source_artifact.artifact_id' "$lock")
    source_artifact_json="$output/$entry_id.artifacts.json"
    if ! gh api "repos/$repo/actions/artifacts?per_page=100" > "$source_artifact_json" 2> "$output/$entry_id.artifacts.error"; then
      mark_unknown "SOURCE_ARTIFACT_API_UNAVAILABLE"
    elif ! jq -e --argjson id "$source_artifact_id" '.artifacts[] | select(.id==$id)' "$source_artifact_json" >/dev/null; then
      mark_unknown "SOURCE_ARTIFACT_NOT_FOUND"
    else
      source_artifact_name=$(jq -r --arg id "$entry_id" '.entries[] | select(.entry_id==$id) | .source_artifact.name' "$lock")
      source_artifact_size=$(jq -r --arg id "$entry_id" '.entries[] | select(.entry_id==$id) | .source_artifact.size_bytes' "$lock")
      source_artifact_sha=$(jq -r --arg id "$entry_id" '.entries[] | select(.entry_id==$id) | .source_artifact.sha256' "$lock")
      source_artifact_url=$(jq -r --arg id "$entry_id" '.entries[] | select(.entry_id==$id) | .source_artifact.archive_download_url' "$lock")
      source_run_id=$(jq -r --arg id "$entry_id" '.entries[] | select(.entry_id==$id) | .source_artifact.run_id' "$lock")
      source_artifact_expected_head=$(jq -r --arg id "$entry_id" '.entries[] | select(.entry_id==$id) | (.source_artifact.expected_workflow_head_sha // .target_commit_sha)' "$lock")
      observed_source_artifact=$(jq -S --argjson id "$source_artifact_id" '.artifacts[] | select(.id==$id) | {id,name,size_in_bytes,digest,expired,workflow_run}' "$source_artifact_json")
      if ! jq -e --arg expected_head "$source_artifact_expected_head" --argjson id "$source_artifact_id" --arg name "$source_artifact_name" \
        --argjson size "$source_artifact_size" --arg sha "$source_artifact_sha" --arg url "$source_artifact_url" --argjson run_id "$source_run_id" \
        '.artifacts[] | select(.id==$id) | .name==$name and .size_in_bytes==$size and .digest==$sha and .archive_download_url==$url and .expired==false and .workflow_run.id==$run_id and .workflow_run.head_sha==$expected_head' \
        "$source_artifact_json" >/dev/null; then
        mark_refuted "SOURCE_ARTIFACT_METADATA_MISMATCH"
      fi
    fi
  fi

  if jq -e --arg id "$entry_id" '.entries[] | select(.entry_id==$id) | has("known_refutation")' "$lock" >/dev/null; then
    known_refutation_code=$(jq -r --arg id "$entry_id" '.entries[] | select(.entry_id==$id) | .known_refutation.code' "$lock")
    mark_refuted "$known_refutation_code"
  fi

  assets=$(jq -s . "$asset_results")
  jq -S -n \
    --arg id "$entry_id" --arg state "$state" --arg reason "$reason" \
    --argjson release "$observed_release" --argjson target "$observed_target" \
    --argjson assets "$assets" --argjson source_run "$observed_source_run" \
    --argjson source_artifact "$observed_source_artifact" --argjson graphql "$observed_graphql" \
    --argjson adoption_proposal "$observed_adoption_proposal" \
    '{entry_id:$id,state:$state,observed:{release:$release,graphql:$graphql,resolved_tag:$target,source_run:$source_run,source_artifact:$source_artifact,adoption_proposal:$adoption_proposal},verified_assets:$assets,reason:(if $reason=="" then null else $reason end)}' \
    > "$output/$entry_id.result.json"
done
verification_end=$(date +%s%N)

results='{}'
for entry_id in $(jq -r '.entries[].entry_id' "$lock"); do
  result=$(jq -c . "$output/$entry_id.result.json")
  results=$(jq -c --arg id "$entry_id" --argjson result "$result" '. + {($id):$result}' <<< "$results")
done

assessment_mismatches=0
for entry_id in $(jq -r '.entries[].entry_id' "$lock"); do
  expected=$(jq -r --arg id "$entry_id" '.entries[] | select(.entry_id==$id) | .state' "$assessment")
  actual=$(jq -r --arg id "$entry_id" '.[$id].state' <<< "$results")
  if [ "$expected" != "$actual" ]; then
    echo "registry state mismatch for $entry_id: assessment=$expected observed=$actual" >&2
    assessment_mismatches=$((assessment_mismatches + 1))
  fi
done

summary=$(jq -c '{entry_count:length,closed:([.[]|select(.state=="CLOSED")]|length),unknown:([.[]|select(.state=="UNKNOWN")]|length),refuted:([.[]|select(.state=="REFUTED")]|length)}' <<< "$results")
lineage=$(jq -c '.lineage' "$lock")
frontier_additions=$(jq -c '.frontier_additions' "$lock")
jq -S -n \
  --arg schema "gooo/non-completeness/capability-evidence-registry/verification/v1" \
  --arg registry "non-completeness-capability-evidence-registry-v1" \
  --arg source_lock "contracts/non-completeness-capability-evidence-registry-v1.json" \
  --argjson entries "$results" --argjson summary "$summary" --argjson lineage "$lineage" --argjson frontier_additions "$frontier_additions" \
  --argjson timing "$(measurement "$verification_start" "$verification_end")" \
  '{schema:$schema,registry_id:$registry,source_lock:$source_lock,entry_count:$summary.entry_count,summary:{closed:$summary.closed,unknown:$summary.unknown,refuted:$summary.refuted},entries:$entries,lineage:$lineage,frontier_additions:$frontier_additions,timing:{verification:$timing}}' \
  > "$output/verification.json"

if [ "$assessment_mismatches" -ne 0 ]; then
  exit 1
fi
