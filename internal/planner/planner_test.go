package planner

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const testRoot = "../../"

func testOptions(input, output string) Options {
	return Options{
		InputRepository:  input,
		OutputDirectory:  output,
		MetaCodePath:     filepath.Join(testRoot, ".gooo/append-planner.gooo"),
		BaselineLockPath: filepath.Join(testRoot, "contracts/upstream-lock-v0.31.0.json"),
	}
}

func TestCanonicalValidAppendClosedAndPreservesInput(t *testing.T) {
	input := filepath.Join(testRoot, "testdata/ledger-v0.31")
	before, err := sourceTreeDigest(input)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Execute(filepath.Join(testRoot, "examples/transactions/valid-append.json"), testOptions(input, t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	if result.Plan.OperationDecision != DecisionClosed || result.Plan.PortfolioDecision != DecisionRefuted {
		t.Fatalf("decisions = %s/%s", result.Plan.OperationDecision, result.Plan.PortfolioDecision)
	}
	if len(result.Plan.Files) != 7 || result.Plan.Metrics.RepositoryWrites != 0 || result.Plan.InputRepositoryMutated {
		t.Fatalf("unexpected plan files/authority: files=%d writes=%d mutated=%v", len(result.Plan.Files), result.Plan.Metrics.RepositoryWrites, result.Plan.InputRepositoryMutated)
	}
	after, err := sourceTreeDigest(input)
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatalf("input tree changed: %s -> %s", before, after)
	}
	if result.Receipt.State != DecisionClosed || len(result.Receipt.Mismatches) != 0 {
		t.Fatalf("replay receipt = %+v", result.Receipt)
	}
}

func TestCanonicalMissingImmutableLockUnknown(t *testing.T) {
	transaction := transactionMap(t)
	delete(transaction["baseline"].(map[string]any), "immutable")
	result := executeTransactionMap(t, transaction)
	if result.Plan.OperationDecision != DecisionUnknown {
		t.Fatalf("decision = %s", result.Plan.OperationDecision)
	}
	if result.Plan.InputRepositoryMutated || result.Plan.RepositoryOutput != "" {
		t.Fatalf("unknown case planned a repository mutation")
	}
	if len(result.Receipt.Mismatches) == 0 {
		t.Fatal("unknown case did not preserve a finding")
	}
}

func TestCanonicalDuplicateAndTextualInsertionRefuted(t *testing.T) {
	duplicate := transactionMap(t)
	duplicate["cell"].(map[string]any)["id"] = "CORE_SEMANTIC_AUTHORITY"
	duplicateResult := executeTransactionMap(t, duplicate)
	if duplicateResult.Plan.OperationDecision != DecisionRefuted {
		t.Fatalf("duplicate decision = %s", duplicateResult.Plan.OperationDecision)
	}

	textual := transactionMap(t)
	textual["insertion_strategy"] = "textual"
	textualResult := executeTransactionMap(t, textual)
	if textualResult.Plan.OperationDecision != DecisionRefuted {
		t.Fatalf("textual decision = %s", textualResult.Plan.OperationDecision)
	}
}

func TestCanonicalRollbackReplayClosed(t *testing.T) {
	result, err := Execute(filepath.Join(testRoot, "examples/transactions/valid-append.json"), testOptions(filepath.Join(testRoot, "testdata/ledger-v0.31"), t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := ReplayRollback(filepath.Join(result.OutputDirectory, "rollback-bundle.json"), result.Plan.RepositoryOutput)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.State != DecisionClosed || len(receipt.Mismatches) != 0 {
		t.Fatalf("rollback receipt = %+v", receipt)
	}
	if _, err := os.Stat(filepath.Join(result.Plan.RepositoryOutput, "evidence/report-v1.json")); !os.IsNotExist(err) {
		t.Fatalf("report projection survived rollback: %v", err)
	}
}

func transactionMap(t *testing.T) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(testRoot, "examples/transactions/valid-append.json"))
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := decodeJSON(data, &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func executeTransactionMap(t *testing.T, value map[string]any) Result {
	t.Helper()
	transactionPath := filepath.Join(t.TempDir(), "transaction.json")
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(transactionPath, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	return mustExecute(t, transactionPath)
}

func mustExecute(t *testing.T, transactionPath string) Result {
	t.Helper()
	result, err := Execute(transactionPath, testOptions(filepath.Join(testRoot, "testdata/ledger-v0.31"), t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestCanonicalJSONIgnoresObjectKeyOrdering(t *testing.T) {
	left := map[string]any{"b": json.Number("2"), "a": []any{json.Number("1")}}
	right := map[string]any{"a": []any{json.Number("1")}, "b": json.Number("2")}
	if !sameCanonical(left, right) {
		t.Fatal("canonical JSON comparison is order-sensitive")
	}
	if !bytes.Equal(mustCanonical(t, left), mustCanonical(t, right)) {
		t.Fatal("canonical bytes differ")
	}
}

func mustCanonical(t *testing.T, value any) []byte {
	t.Helper()
	data, err := canonicalJSON(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
