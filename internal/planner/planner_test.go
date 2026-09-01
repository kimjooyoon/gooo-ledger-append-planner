package planner

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

func TestTransactionManifestCorpus(t *testing.T) {
	cases := []struct {
		name        string
		input       string
		transaction string
		lock        string
		decision    string
	}{
		{"v0.31-closed", "testdata/ledger-v0.31", "examples/transactions/valid-append-v2-v0.31.json", "contracts/upstream-lock-v0.31.0.json", DecisionClosed},
		{"v0.32-closed", "testdata/ledger-v0.32", "examples/transactions/valid-append-v2-v0.32.json", "contracts/upstream-lock-v0.32.0.json", DecisionClosed},
		{"wrong-source-tree-refuted", "testdata/ledger-v0.31", "examples/transactions/wrong-source-tree-v0.32.json", "contracts/upstream-lock-v0.32.0.json", DecisionRefuted},
		{"missing-binding-unknown", "testdata/ledger-v0.32", "examples/transactions/missing-binding-v0.32.json", "contracts/upstream-lock-v0.32.0.json", DecisionUnknown},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			input := filepath.Join(testRoot, testCase.input)
			before, err := sourceTreeDigest(input)
			if err != nil {
				t.Fatal(err)
			}
			options := testOptions(input, t.TempDir())
			options.BaselineLockPath = filepath.Join(testRoot, testCase.lock)
			result, err := Execute(filepath.Join(testRoot, testCase.transaction), options)
			if err != nil {
				t.Fatal(err)
			}
			if result.Plan.OperationDecision != testCase.decision {
				t.Fatalf("decision = %s, want %s; findings=%+v", result.Plan.OperationDecision, testCase.decision, result.Plan.Findings)
			}
			after, err := sourceTreeDigest(input)
			if err != nil {
				t.Fatal(err)
			}
			if before != after || result.Plan.InputRepositoryMutated || result.Plan.Metrics.RepositoryWrites != 0 {
				t.Fatalf("input authority changed: before=%s after=%s mutated=%v writes=%d", before, after, result.Plan.InputRepositoryMutated, result.Plan.Metrics.RepositoryWrites)
			}
			if testCase.decision != DecisionClosed && result.Plan.RepositoryOutput != "" {
				t.Fatalf("non-closed case materialized repository output: %s", result.Plan.RepositoryOutput)
			}
		})
	}
}

func TestProjectionManifestV3CorpusAndRollback(t *testing.T) {
	input := filepath.Join(testRoot, "testdata/ledger-v0.33")
	options := Options{
		InputRepository:         input,
		OutputDirectory:         t.TempDir(),
		MetaCodePath:            filepath.Join(testRoot, ".gooo/append-planner.gooo"),
		TransactionManifestPath: filepath.Join(testRoot, ".gooo/append-transaction-manifest-v3.gooo"),
		BaselineLockPath:        filepath.Join(testRoot, "contracts/upstream-lock-v0.33.0.json"),
	}
	result, err := Execute(filepath.Join(testRoot, "examples/transactions/valid-append-v3-v0.33.json"), options)
	if err != nil {
		t.Fatal(err)
	}
	if result.Plan.OperationDecision != DecisionClosed || len(result.Plan.Files) != 7 || result.Plan.Metrics.GeneratedFiles != 2 || result.Plan.Metrics.RepositoryWrites != 0 {
		t.Fatalf("unexpected v3 projection plan: decision=%s files=%d generated=%d writes=%d", result.Plan.OperationDecision, len(result.Plan.Files), result.Plan.Metrics.GeneratedFiles, result.Plan.Metrics.RepositoryWrites)
	}
	for _, path := range []string{"evidence/report-v1.json", "evidence/history-v1.json"} {
		mutation, ok := mutationByPath(result.Plan.Files, path)
		if !ok || mutation.Action != "replace" || !mutation.BeforeExists || mutation.BeforeDigest == "" || !mutation.AfterExists {
			t.Fatalf("projection mutation is not an exact replacement: %+v", mutation)
		}
	}
	rollback, err := ReplayRollback(filepath.Join(result.OutputDirectory, "rollback-bundle.json"), result.Plan.RepositoryOutput)
	if err != nil {
		t.Fatal(err)
	}
	if rollback.State != DecisionClosed || len(rollback.Mismatches) != 0 {
		t.Fatalf("projection rollback = %+v", rollback)
	}
	if _, err := os.Stat(filepath.Join(result.Plan.RepositoryOutput, "evidence/report-v1.json")); err != nil {
		t.Fatalf("projection rollback deleted an existing report: %v", err)
	}
}

func TestProjectionManifestV3AuthorityFindings(t *testing.T) {
	tests := []struct {
		name       string
		mutate     func(string) string
		decision   string
		reason     string
		allUnknown bool
	}{
		{
			name: "before-digest-mismatch",
			mutate: func(value string) string {
				return strings.Replace(value, "projection-before-digest report \"sha256:94999f6037cfeb8875ed1bc4323a146e4a7b3bcac9208bfcd209020c1f5ee4df\"", "projection-before-digest report \"sha256:0000000000000000000000000000000000000000000000000000000000000000\"", 1)
			},
			decision: DecisionRefuted, reason: "PROJECTION_BEFORE_DIGEST_MISMATCH",
		},
		{
			name: "source-semantic-mismatch",
			mutate: func(value string) string {
				return strings.Replace(value, "projection-source-semantic-digest report \"sha256:2f0e0227b5e2cf86d223e06415c92c20f23000068a026f2a4988f85039e2c203\"", "projection-source-semantic-digest report \"sha256:0000000000000000000000000000000000000000000000000000000000000000\"", 1)
			},
			decision: DecisionRefuted, reason: "PROJECTION_SOURCE_SEMANTIC_DIGEST_MISMATCH",
		},
		{
			name: "missing-projection-authority",
			mutate: func(value string) string {
				for _, line := range []string{
					"target-file DERIVED_PROJECTION report \"evidence/report-v1.json\"\n",
					"projection-kind report \"assessment-report\"\n",
					"projection-before-digest report \"sha256:94999f6037cfeb8875ed1bc4323a146e4a7b3bcac9208bfcd209020c1f5ee4df\"\n",
					"projection-source-semantic-digest report \"sha256:2f0e0227b5e2cf86d223e06415c92c20f23000068a026f2a4988f85039e2c203\"\n",
					"projection-after-invariant report \"deterministic-regenerate-from-post-append-semantic-source\"\n",
				} {
					value = strings.Replace(value, line, "", 1)
				}
				return value
			},
			decision: DecisionUnknown, allUnknown: true,
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			manifest, err := os.ReadFile(filepath.Join(testRoot, ".gooo/append-transaction-manifest-v3.gooo"))
			if err != nil {
				t.Fatal(err)
			}
			manifestPath := filepath.Join(t.TempDir(), "manifest.gooo")
			if err := os.WriteFile(manifestPath, []byte(testCase.mutate(string(manifest))), 0o644); err != nil {
				t.Fatal(err)
			}
			result, err := Execute(filepath.Join(testRoot, "examples/transactions/valid-append-v3-v0.33.json"), Options{
				InputRepository: input, OutputDirectory: t.TempDir(), MetaCodePath: filepath.Join(testRoot, ".gooo/append-planner.gooo"),
				TransactionManifestPath: manifestPath, BaselineLockPath: filepath.Join(testRoot, "contracts/upstream-lock-v0.33.0.json"),
			})
			if err != nil {
				t.Fatal(err)
			}
			if result.Plan.OperationDecision != testCase.decision {
				t.Fatalf("decision=%s findings=%+v", result.Plan.OperationDecision, result.Plan.Findings)
			}
			if testCase.reason != "" && !hasFindingReason(result.Plan.Findings, testCase.reason) {
				t.Fatalf("findings=%+v, want reason %s", result.Plan.Findings, testCase.reason)
			}
			if testCase.allUnknown {
				if len(result.Plan.Findings) == 0 || result.Plan.Findings[0].State != DecisionUnknown || result.Plan.Findings[0].Stage == "" || result.Plan.Findings[0].Step == "" || result.Plan.Findings[0].Reason == "" || result.Plan.Findings[0].UnknownClass == "" || result.Plan.Findings[0].NextOperation == "" || len(result.Plan.Findings[0].BlockedBy) == 0 {
					t.Fatalf("unknown frontier is incomplete: %+v", result.Plan.Findings)
				}
			}
		})
	}
	deletedInput := filepath.Join(t.TempDir(), "ledger-v0.33-deleted-projection")
	if err := copyTree(filepath.Join(testRoot, "testdata/ledger-v0.33"), deletedInput); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(deletedInput, "evidence/report-v1.json")); err != nil {
		t.Fatal(err)
	}
	deletedResult, err := Execute(filepath.Join(testRoot, "examples/transactions/valid-append-v3-v0.33.json"), Options{
		InputRepository: deletedInput, OutputDirectory: t.TempDir(), MetaCodePath: filepath.Join(testRoot, ".gooo/append-planner.gooo"),
		TransactionManifestPath: filepath.Join(testRoot, ".gooo/append-transaction-manifest-v3.gooo"), BaselineLockPath: filepath.Join(testRoot, "contracts/upstream-lock-v0.33.0.json"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if deletedResult.Plan.OperationDecision != DecisionRefuted || !hasFindingReason(deletedResult.Plan.Findings, "UNEXPECTED_PROJECTION_DELETE") {
		t.Fatalf("deleted projection decision/findings = %s/%+v", deletedResult.Plan.OperationDecision, deletedResult.Plan.Findings)
	}
}

func hasFindingReason(findings []Finding, reason string) bool {
	for _, finding := range findings {
		if finding.Reason == reason {
			return true
		}
	}
	return false
}

func mustCanonical(t *testing.T, value any) []byte {
	t.Helper()
	data, err := canonicalJSON(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
