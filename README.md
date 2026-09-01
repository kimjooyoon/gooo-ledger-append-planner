# gooo-ledger-append-planner

This repository contains one operation: append exactly one adoption transaction to a caller-owned temporary copy of the v0.31.0 `gooo-self-improvement-ledger`. The planner parses `.gooo` and JSON into an operation graph and AST, then derives the decision from the metacode precedence and append contract.

The input repository is read-only. A successful run writes `repository/` below the caller-owned output directory and emits `patch-plan.json`, `replay-receipt.json`, `rollback-bundle.json`, and `human-dossier.md`. It never commits, pushes, merges, or writes to the input ledger.

The baseline is locked in [contracts/upstream-lock-v0.31.0.json](contracts/upstream-lock-v0.31.0.json). It records the exact GitHub API source archive digest, release ID `380120973`, annotated tag object, commit, and release asset `538664422` digest. The copied [testdata/ledger-v0.31](testdata/ledger-v0.31) tree is the v0.31.0 fixture.

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

CI also runs `cmd/gooo-ledger-append-verifier`, an independent consumer that does not import the planner executor. It independently reads the `.gooo` declarations, immutable baseline, transaction, generated plan/receipt, input tree, and materialized copy, then checks the seven-path append boundary. GitHub Actions evidence is uploaded per run; release tags additionally produce a REST-backed release audit artifact.

The historical v0.31.0 ledger observation `local_validation_executions=1/process=REFUTED` and multiple uncommitted wrong insertion-point attempts are retained only as motivation. No measured time improvement is claimed. Whole-language improvement remains `UNKNOWN` until a future real ledger adoption supplies a matched manual/tool before-after pair under the same digest; external utility remains `UNKNOWN/NOT_MADE`.
