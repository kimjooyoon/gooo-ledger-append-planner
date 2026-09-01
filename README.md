# gooo-ledger-append-planner

This repository contains one operation: append exactly one adoption transaction to a caller-owned temporary copy of the v0.31.0 `gooo-self-improvement-ledger`. The planner parses `.gooo` and JSON into an operation graph and AST, then derives the decision from the metacode precedence and append contract.

The input repository is read-only. A successful run writes `repository/` below the caller-owned output directory and emits `patch-plan.json`, `replay-receipt.json`, `rollback-bundle.json`, and `human-dossier.md`. It never commits, pushes, merges, or writes to the input ledger.

The v0.31.0, v0.32.0, and immutable v0.33.0 targets are locked in [contracts](contracts). The copied [testdata/ledger-v0.31](testdata/ledger-v0.31), [testdata/ledger-v0.32](testdata/ledger-v0.32), and [testdata/ledger-v0.33](testdata/ledger-v0.33) trees are read-only fixtures. The v0.33→v0.34 transaction uses the immutable upstream v0.33.0 tree; an upstream v0.34.0 tag/release was not observed and is not fabricated.

The v2 transaction contract is declared by [.gooo/append-transaction-manifest-v2.gooo](.gooo/append-transaction-manifest-v2.gooo). It binds each immutable target to its exact before source-tree digest, denominator, state counts, structural anchor IDs, and the exact seven-file mutation set. The executor rejects a source-tree mismatch as `REFUTED` and treats an absent target binding as `UNKNOWN`; neither condition is made `CLOSED` by weakening the digest guard.

The v3 contract is declared by [.gooo/append-transaction-manifest-v3.gooo](.gooo/append-transaction-manifest-v3.gooo). `.gooo` declares five `SEMANTIC_APPEND_ONLY` sources and two `DERIVED_PROJECTION` targets. Existing semantic files can never be overwritten; existing projections are replaced only in the caller-owned temporary copy when the manifest binds the exact before digest, projection kind, source semantic digest, and expected after invariant. Missing projection authority is `UNKNOWN` with the complete six-field frontier; digest mismatch, source mutation, undeclared semantic targets, and unexpected deletion remain `REFUTED`.

## Run the canonical append

```sh
go run ./cmd/gooo-ledger-append-planner \
  -repository testdata/ledger-v0.31 \
  -transaction examples/transactions/valid-append.json \
  -metacode .gooo/append-planner.gooo \
  -baseline-lock contracts/upstream-lock-v0.31.0.json \
  -output-dir /tmp/gooo-ledger-append-result
```

The result is `decision=CLOSED` for the operation. The portfolio decision remains `REFUTED` because the v0.31.0 assessment already contains refuted cells; this is the declared `REFUTED > UNKNOWN > CLOSED` precedence, not a success override.

Canonical conformance covers a valid append, missing immutable lock (`UNKNOWN`), duplicate/overwrite (`REFUTED`), ambiguous textual insertion (`REFUTED`), and rollback replay (`CLOSED`). The initial development session recorded 9 local Go test invocations, 1 vet invocation, 3 `go run` compilations, and 2 conformance invocations; those observations make `development_process=REFUTED` and are not product conformance authority. GitHub Actions is the only product conformance authority for release evidence.

The v2 corpus covers v0.31.0 `CLOSED`, v0.32.0 `CLOSED`, a v0.32 binding applied to the v0.31 tree (`SOURCE_TREE_DIGEST_MISMATCH`, `REFUTED`), and a missing manifest target binding (`UNKNOWN`). A matched v0.1→v0.2 pair supports immutable target bindings `1→2`; whole-language improvement and external utility remain `UNKNOWN`.

The v3 corpus preserves those v0.31/v0.32 cases and adds v0.33 projection regeneration as `CLOSED`, projection before-digest and source-semantic-digest mismatches as `REFUTED`, and missing projection authority as `UNKNOWN`. Only the exact matched v0.2→v0.3 pair supports projection transition cardinality `0→1`; broader projection or language claims remain `UNKNOWN`. The adopted explanation-carrying compiler evidence is the immutable `kimjooyoon/gooo-reflexive-compiler-slice` v0.3.0 release, scoped to `ONE_COMPILER_PHASE_ONLY`.

CI also runs `cmd/gooo-ledger-append-verifier`, an independent consumer that does not import the planner executor. It independently reads the `.gooo` declarations, immutable baseline, transaction, generated plan/receipt, input tree, and materialized copy, then checks the manifest-declared seven-path boundary, including projection replacement bindings. GitHub Actions evidence is uploaded per run; release tags additionally produce a REST-backed release audit artifact.

CI conformance runs also upload the exact patch plan and replay receipt used for metric evidence; local validation remains non-authoritative.

The historical v0.31.0 ledger observation `local_validation_executions=1/process=REFUTED` and multiple uncommitted wrong insertion-point attempts are retained only as motivation. No measured time improvement is claimed. Whole-language improvement remains `UNKNOWN` until a future real ledger adoption supplies a matched manual/tool before-after pair under the same digest; external utility remains `UNKNOWN/NOT_MADE`.
