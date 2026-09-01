#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"

go test ./internal/... ./cmd/...

manifest="$root/.gooo/append-transaction-manifest-v2.gooo"
work=$(mktemp -d "${TMPDIR:-/tmp}/gooo-ledger-append-conformance-v2.XXXXXX")
trap 'rm -rf "$work" "${work}-summary.json"' EXIT

run_case() {
  local name=$1
  local repository=$2
  local transaction=$3
  local baseline_lock=$4
  local expected=$5
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
  CLOSED
run_case v0.32-closed \
  "$root/testdata/ledger-v0.32" \
  "$root/examples/transactions/valid-append-v2-v0.32.json" \
  "$root/contracts/upstream-lock-v0.32.0.json" \
  CLOSED
run_case wrong-source-tree-refuted \
  "$root/testdata/ledger-v0.31" \
  "$root/examples/transactions/wrong-source-tree-v0.32.json" \
  "$root/contracts/upstream-lock-v0.32.0.json" \
  REFUTED
run_case missing-binding-unknown \
  "$root/testdata/ledger-v0.32" \
  "$root/examples/transactions/missing-binding-v0.32.json" \
  "$root/contracts/upstream-lock-v0.32.0.json" \
  UNKNOWN

jq -s '.' "$work"/*/record.json >"$work/corpus.json"
jq -e '
  (map(.case) | sort) == ["missing-binding-unknown", "v0.31-closed", "v0.32-closed", "wrong-source-tree-refuted"] and
  (map(select(.case == "v0.31-closed" or .case == "v0.32-closed") | .plan.metrics.exact_files_changed) | unique) == [7] and
  (map(select(.case == "v0.31-closed" or .case == "v0.32-closed") | .plan.metrics.repository_writes) | unique) == [0] and
  (map(select(.case == "v0.31-closed" or .case == "v0.32-closed") | .receipt.mismatches) | flatten | length) == 0
' "$work/corpus.json" >/dev/null

jq -n \
  --slurpfile corpus "$work/corpus.json" \
  --arg schema "gooo/ledger-append-planner/conformance-corpus/v2" \
  --arg manifest "$manifest" \
  '(
    $corpus[0] as $cases |
    ($cases | map(select(.case == "v0.31-closed" or .case == "v0.32-closed"))) as $matched |
    {
      schema: $schema,
      authority: "GITHUB_ACTIONS",
      transaction_manifest: $manifest,
      cases: $cases,
      matched_v0_1_v0_2_pair: {
        exact: (($matched|length) == 2 and ($matched|map(.plan.operation)|unique) == ["append_exactly_one_adoption_transaction"] and ($matched|map(.plan.manifest_key)|sort) == ["v0.31.0", "v0.32.0"] and ($matched|map(.plan.target_before_digest)|unique|length) == 2 and ($matched|map(.plan.metrics.exact_files_planned)|unique) == [7] and ($matched|map(.plan.metrics.repository_writes)|unique) == [0] and ($matched|map(.receipt.mismatches)|flatten|length) == 0),
        supported_immutable_target_bindings: {before: 1, after: 2},
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

echo "conformance: v0.31 CLOSED, v0.32 CLOSED, wrong source tree REFUTED, missing binding UNKNOWN"
