package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kimjooyoon/gooo-ledger-append-planner/internal/planner"
)

func main() {
	transaction := flag.String("transaction", "", "transaction JSON")
	repository := flag.String("repository", "", "read-only input ledger repository")
	meta := flag.String("metacode", ".gooo/append-planner.gooo", "Gooo authority file")
	baselineLock := flag.String("baseline-lock", "contracts/upstream-lock-v0.31.0.json", "immutable upstream lock")
	output := flag.String("output-dir", "", "empty caller-owned output directory")
	sourceArchive := flag.String("source-archive", "", "optional exact GitHub API source archive")
	releaseAsset := flag.String("release-asset", "", "optional exact immutable release asset")
	flag.Parse()
	if *transaction == "" || *repository == "" {
		fatal("-transaction and -repository are required")
	}
	outputDir := *output
	if outputDir == "" {
		var err error
		outputDir, err = os.MkdirTemp("", "gooo-ledger-append-")
		if err != nil {
			fatal("create caller-owned temporary output: %v", err)
		}
	}
	result, err := planner.Execute(*transaction, planner.Options{
		InputRepository: *repository, OutputDirectory: outputDir, MetaCodePath: *meta,
		BaselineLockPath: *baselineLock, SourceArchivePath: *sourceArchive, ReleaseAssetPath: *releaseAsset,
	})
	if err != nil {
		fatal("append transaction: %v", err)
	}
	printJSON(map[string]any{
		"decision":           result.Plan.OperationDecision,
		"portfolio_decision": result.Plan.PortfolioDecision,
		"transaction_id":     result.Plan.TransactionID,
		"output_directory":   filepath.Clean(result.OutputDirectory),
		"repository_output":  result.Plan.RepositoryOutput,
		"patch_plan":         filepath.Join(result.OutputDirectory, "patch-plan.json"),
		"replay_receipt":     filepath.Join(result.OutputDirectory, "replay-receipt.json"),
		"rollback_bundle":    filepath.Join(result.OutputDirectory, "rollback-bundle.json"),
		"human_dossier":      filepath.Join(result.OutputDirectory, "human-dossier.md"),
	})
	if result.Plan.OperationDecision != planner.DecisionClosed {
		os.Exit(3)
	}
}

func printJSON(value any) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fatal("encode result: %v", err)
	}
	fmt.Println(string(data))
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(2)
}
