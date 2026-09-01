#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"

go test ./internal/... ./cmd/...

manifest_v2="$root/.gooo/append-transaction-manifest-v2.gooo"
manifest_v3="$root/.gooo/append-transaction-manifest-v3.gooo"
work=$(mktemp -d "${TMPDIR:-/tmp}/gooo-ledger-append-conformance-v3.XXXXXX")
trap 'rm -rf "$work" "${work}-summary.json"' EXIT

run_case() {
  local name=$1
  local repository=$2
  local transaction=$3
  local baseline_lock=$4
  local manifest=$5
  local expected=$6
  local case_dir="$work/$name"
  local output_dir="$case_dir/output"
  local status=0
  mkdir -p "$case_dir"
  go run ./cmd/gooo-ledger-append-planner \
    -repository "$repository" \
    -transaction "$transaction" \
    -metacode "$root/.gooo/append-planner.gooo" \
    -transaction-manifest "$manifest" \
    -baseline-lock "$baseline_lock" \
    -output-dir "$output_dir" >"$case_dir/summary.json" || status=$?
  if test "$expected" = CLOSED; then
    test "$status" = 0
    go run ./cmd/gooo-ledger-append-verifier \
      -repository "$repository" \
      -transaction "$transaction" \
      -metacode "$root/.gooo/append-planner.gooo" \
      -transaction-manifest "$manifest" \
      -baseline-lock "$baseline_lock" \
      -output "$output_dir"
    jq -e '.decision == "CLOSED" and .portfolio_decision == "REFUTED"' "$case_dir/summary.json" >/dev/null
    jq -e '.metrics.exact_files_changed == 7 and .metrics.exact_files_planned == 7 and .metrics.repository_writes == 0 and .input_repository_mutated == false' "$output_dir/patch-plan.json" >/dev/null
    jq -e '.state == "CLOSED" and .mismatches == [] and .rollback_ready == true and .repository_writes == 0' "$output_dir/replay-receipt.json" >/dev/null
  else
    test "$status" != 0
    jq -e --arg expected "$expected" '.decision == $expected and .portfolio_decision == $expected' "$case_dir/summary.json" >/dev/null
    jq -e '.repository_output == null and .input_repository_mutated == false' "$output_dir/patch-plan.json" >/dev/null
    test ! -e "$output_dir/repository"
  fi
  jq -n \
    --arg name "$name" \
    --arg expected "$expected" \
    --slurpfile summary "$case_dir/summary.json" \
    --slurpfile plan "$output_dir/patch-plan.json" \
    --slurpfile receipt "$output_dir/replay-receipt.json" \
    '{case:$name,expected:$expected,summary:$summary[0],plan:$plan[0],receipt:$receipt[0]}' >"$case_dir/record.json"
}

run_case v0.31-closed \
  "$root/testdata/ledger-v0.31" \
  "$root/examples/transactions/valid-append-v2-v0.31.json" \
  "$root/contracts/upstream-lock-v0.31.0.json" \
  "$manifest_v2" \
  CLOSED
run_case v0.32-closed \
  "$root/testdata/ledger-v0.32" \
  "$root/examples/transactions/valid-append-v2-v0.32.json" \
  "$root/contracts/upstream-lock-v0.32.0.json" \
  "$manifest_v2" \
  CLOSED
run_case wrong-source-tree-refuted \
  "$root/testdata/ledger-v0.31" \
  "$root/examples/transactions/wrong-source-tree-v0.32.json" \
  "$root/contracts/upstream-lock-v0.32.0.json" \
  "$manifest_v2" \
  REFUTED
run_case missing-binding-unknown \
  "$root/testdata/ledger-v0.32" \
  "$root/examples/transactions/missing-binding-v0.32.json" \
  "$root/contracts/upstream-lock-v0.32.0.json" \
  "$manifest_v2" \
  UNKNOWN

run_case v0.33-projection-closed \
  "$root/testdata/ledger-v0.33" \
  "$root/examples/transactions/valid-append-v3-v0.33.json" \
  "$root/contracts/upstream-lock-v0.33.0.json" \
  "$manifest_v3" \
  CLOSED

projection_digest_refuted="$work/projection-before-digest-refuted.gooo"
sed 's#projection-before-digest report "sha256:[^"]*"#projection-before-digest report "sha256:0000000000000000000000000000000000000000000000000000000000000000"#' "$manifest_v3" >"$projection_digest_refuted"
run_case projection-before-digest-refuted \
  "$root/testdata/ledger-v0.33" \
  "$root/examples/transactions/valid-append-v3-v0.33.json" \
  "$root/contracts/upstream-lock-v0.33.0.json" \
  "$projection_digest_refuted" \
  REFUTED

source_digest_refuted="$work/source-semantic-digest-refuted.gooo"
sed 's#projection-source-semantic-digest report "sha256:[^"]*"#projection-source-semantic-digest report "sha256:0000000000000000000000000000000000000000000000000000000000000000"#' "$manifest_v3" >"$source_digest_refuted"
run_case source-semantic-digest-refuted \
  "$root/testdata/ledger-v0.33" \
  "$root/examples/transactions/valid-append-v3-v0.33.json" \
  "$root/contracts/upstream-lock-v0.33.0.json" \
  "$source_digest_refuted" \
  REFUTED

missing_projection_authority="$work/missing-projection-authority-unknown.gooo"
sed \
  -e '/target-file DERIVED_PROJECTION report/d' \
  -e '/projection-kind report/d' \
  -e '/projection-before-digest report/d' \
  -e '/projection-source-semantic-digest report/d' \
  -e '/projection-after-invariant report/d' \
  "$manifest_v3" >"$missing_projection_authority"
run_case missing-projection-authority-unknown \
  "$root/testdata/ledger-v0.33" \
  "$root/examples/transactions/valid-append-v3-v0.33.json" \
  "$root/contracts/upstream-lock-v0.33.0.json" \
  "$missing_projection_authority" \
  UNKNOWN

jq -s '.' "$work"/*/record.json >"$work/corpus.json"
jq -e '
  (map(.case) | sort) == ["missing-binding-unknown", "missing-projection-authority-unknown", "projection-before-digest-refuted", "source-semantic-digest-refuted", "v0.31-closed", "v0.32-closed", "v0.33-projection-closed", "wrong-source-tree-refuted"] and
  (map(select(.case == "v0.31-closed" or .case == "v0.32-closed") | .plan.metrics.exact_files_changed) | unique) == [7] and
  (map(select(.case == "v0.31-closed" or .case == "v0.32-closed") | .plan.metrics.repository_writes) | unique) == [0] and
  (map(select(.case == "v0.31-closed" or .case == "v0.32-closed" or .case == "v0.33-projection-closed") | .receipt.mismatches) | flatten | length) == 0 and
  (map(select(.case == "v0.33-projection-closed") | .plan.manifest_file_targets | map(.kind) | sort) | unique) == [["DERIVED_PROJECTION", "DERIVED_PROJECTION", "SEMANTIC_APPEND_ONLY", "SEMANTIC_APPEND_ONLY", "SEMANTIC_APPEND_ONLY", "SEMANTIC_APPEND_ONLY", "SEMANTIC_APPEND_ONLY"]]
' "$work/corpus.json" >/dev/null

jq -n \
  --slurpfile corpus "$work/corpus.json" \
  --arg schema "gooo/ledger-append-planner/conformance-corpus/v3" \
  --arg manifest "$manifest_v3" \
  '(
    $corpus[0] as $cases |
    ($cases | map(select(.case == "v0.31-closed" or .case == "v0.32-closed"))) as $matched_v0_1_v0_2 |
    ($cases | map(select(.case == "v0.32-closed" or .case == "v0.33-projection-closed"))) as $matched_v0_2_v0_3 |
    {
      schema: $schema,
      authority: "GITHUB_ACTIONS",
      transaction_manifest: $manifest,
      cases: $cases,
      matched_v0_1_v0_2_pair: {
        exact: (($matched_v0_1_v0_2|length) == 2 and ($matched_v0_1_v0_2|map(.plan.operation)|unique) == ["append_exactly_one_adoption_transaction"] and ($matched_v0_1_v0_2|map(.plan.manifest_key)|sort) == ["v0.31.0", "v0.32.0"] and ($matched_v0_1_v0_2|map(.plan.target_before_digest)|unique|length) == 2 and ($matched_v0_1_v0_2|map(.plan.metrics.exact_files_planned)|unique) == [7] and ($matched_v0_1_v0_2|map(.plan.metrics.repository_writes)|unique) == [0] and ($matched_v0_1_v0_2|map(.receipt.mismatches)|flatten|length) == 0),
        supported_immutable_target_bindings: {before: 1, after: 2},
        remainder: "UNKNOWN"
      },
      matched_v0_2_v0_3_projection_pair: {
        exact: (($matched_v0_2_v0_3|length) == 2 and ($matched_v0_2_v0_3|map(.plan.metrics.exact_files_planned)|unique) == [7] and ($matched_v0_2_v0_3|map(.plan.metrics.repository_writes)|unique) == [0] and ($matched_v0_2_v0_3|map(.receipt.mismatches)|flatten|length) == 0),
        supported_projection_transition_cardinality: {before: 0, after: 1},
        remainder: "UNKNOWN"
      }
    }
  )' >"$work/metrics.json"

if test -n "${CONFORMANCE_EVIDENCE_DIR:-}"; then
  mkdir -p "$CONFORMANCE_EVIDENCE_DIR"
  cp "$work/v0.31-closed/summary.json" "$CONFORMANCE_EVIDENCE_DIR/summary.json"
  cp "$work/v0.31-closed/output/patch-plan.json" "$CONFORMANCE_EVIDENCE_DIR/patch-plan.json"
  cp "$work/v0.31-closed/output/replay-receipt.json" "$CONFORMANCE_EVIDENCE_DIR/replay-receipt.json"
  cp "$work/corpus.json" "$CONFORMANCE_EVIDENCE_DIR/corpus.json"
  cp "$work/metrics.json" "$CONFORMANCE_EVIDENCE_DIR/metrics.json"
fi

echo "conformance: v0.31 CLOSED, v0.32 CLOSED, v0.33 projection CLOSED, projection digest/source authority REFUTED, missing bindings UNKNOWN"
