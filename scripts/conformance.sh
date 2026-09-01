#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
cd "$root"

go test ./internal/... ./cmd/...

fixture="$root/testdata/ledger-v0.31"
transaction="$root/examples/transactions/valid-append.json"
output=$(mktemp -d "${TMPDIR:-/tmp}/gooo-ledger-append-conformance.XXXXXX")
trap 'rm -rf "$output" "${output}-summary.json"' EXIT

go run ./cmd/gooo-ledger-append-planner \
  -repository "$fixture" \
  -transaction "$transaction" \
  -metacode "$root/.gooo/append-planner.gooo" \
  -baseline-lock "$root/contracts/upstream-lock-v0.31.0.json" \
  -output-dir "$output" >"$output-summary.json"

go run ./cmd/gooo-ledger-append-verifier \
  -repository "$fixture" \
  -transaction "$transaction" \
  -metacode "$root/.gooo/append-planner.gooo" \
  -baseline-lock "$root/contracts/upstream-lock-v0.31.0.json" \
  -output "$output"

jq -e '.decision == "CLOSED" and .portfolio_decision == "REFUTED"' "$output-summary.json" >/dev/null
jq -e '.metrics.exact_files_changed == 7 and .metrics.repository_writes == 0 and .input_repository_mutated == false' "$output/patch-plan.json" >/dev/null
jq -e '.state == "CLOSED" and .mismatches == []' "$output/replay-receipt.json" >/dev/null
test ! -e "$fixture/evidence/report-v1.json"
test ! -e "$fixture/evidence/history-v1.json"

if test -n "${CONFORMANCE_EVIDENCE_DIR:-}"; then
  mkdir -p "$CONFORMANCE_EVIDENCE_DIR"
  cp "$output-summary.json" "$CONFORMANCE_EVIDENCE_DIR/summary.json"
  cp "$output/patch-plan.json" "$CONFORMANCE_EVIDENCE_DIR/patch-plan.json"
  cp "$output/replay-receipt.json" "$CONFORMANCE_EVIDENCE_DIR/replay-receipt.json"
fi

echo "conformance: CLOSED structural append in caller-owned temporary copy"
