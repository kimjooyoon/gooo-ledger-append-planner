package planner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Options struct {
	InputRepository         string
	OutputDirectory         string
	MetaCodePath            string
	TransactionManifestPath string
	BaselineLockPath        string
	SourceArchivePath       string
	ReleaseAssetPath        string
}

type Result struct {
	Plan            PatchPlan
	Receipt         ReplayReceipt
	Rollback        RollbackBundle
	Dossier         string
	OutputDirectory string
}

type runState struct {
	Meta           MetaCode
	Manifest       TransactionManifest
	TargetBinding  *TargetBinding
	Transaction    Transaction
	TransactionRaw map[string]any
	BaselineLock   map[string]any
	Ledger         LedgerAST
	BeforeFiles    map[string][]byte
	AfterFiles     map[string][]byte
	BeforeDigest   string
	AfterDigest    string
	Findings       []Finding
	Plan           PatchPlan
	Receipt        ReplayReceipt
	Rollback       RollbackBundle
	Dossier        string
	Start          time.Time
}

func Execute(transactionPath string, options Options) (Result, error) {
	if options.InputRepository == "" {
		return Result{}, fmt.Errorf("input repository is required")
	}
	if options.MetaCodePath == "" || options.BaselineLockPath == "" {
		return Result{}, fmt.Errorf("metacode and baseline lock are required")
	}
	if filepath.Clean(options.InputRepository) == filepath.Clean(options.OutputDirectory) && options.OutputDirectory != "" {
		return Result{}, fmt.Errorf("output directory must be separate from input repository")
	}
	if isWithin(options.OutputDirectory, options.InputRepository) || isWithin(options.InputRepository, options.OutputDirectory) {
		return Result{}, fmt.Errorf("input repository and output directory must not contain one another")
	}
	meta, err := LoadMetaCode(options.MetaCodePath)
	if err != nil {
		return Result{}, err
	}
	transaction, transactionRaw, err := loadTransaction(transactionPath)
	if err != nil {
		return Result{}, err
	}
	manifestPath := options.TransactionManifestPath
	manifest := TransactionManifest{}
	if transaction.Schema == "gooo/ledger-append-transaction/v2" || transaction.Schema == "gooo/ledger-append-transaction/v3" {
		if manifestPath == "" {
			manifestPath = defaultManifestPath(transaction.Schema, meta.TransactionManifestPath)
		}
		if manifestPath != "" {
			manifest, err = LoadTransactionManifest(resolveAuthorityPath(options.MetaCodePath, manifestPath))
			if err != nil {
				return Result{}, fmt.Errorf("load transaction manifest: %w", err)
			}
		}
	}
	lock, err := loadASTFile(options.BaselineLockPath)
	if err != nil {
		return Result{}, err
	}
	lockObject, err := object(lock, "baseline lock")
	if err != nil {
		return Result{}, err
	}
	ledger, err := readLedger(options.InputRepository, meta)
	if err != nil {
		return Result{}, err
	}
	beforeFiles := cloneFiles(ledger.Files)
	beforeDigest, err := subjectDigest(ledger, beforeFiles, meta)
	if err != nil {
		return Result{}, err
	}
	state := &runState{
		Meta: meta, Manifest: manifest, Transaction: transaction, BaselineLock: lockObject, Ledger: ledger,
		TransactionRaw: transactionRaw,
		BeforeFiles:    beforeFiles, BeforeDigest: beforeDigest, Start: time.Now(),
	}
	if err := validateExecution(state, options); err != nil {
		return Result{}, err
	}
	state.Plan = basePlan(state)
	if len(state.Findings) == 0 {
		if err := buildPatch(state); err != nil {
			return Result{}, err
		}
		if err := renderAndFinalize(state, options); err != nil {
			return Result{}, err
		}
		if err := materialize(state, options.InputRepository, options.OutputDirectory); err != nil {
			return Result{}, err
		}
		if err := verifyMaterialized(state, options.OutputDirectory); err != nil {
			return Result{}, err
		}
		state.Plan.RepositoryOutput = filepath.Join(options.OutputDirectory, "repository")
		state.Plan.Metrics = collectMetrics(state.Plan.Metrics, state.Plan.RepositoryOutput, state.Meta, state.Plan.Files, 2)
		state.Plan.Metrics.WallMS = time.Since(state.Start).Milliseconds()
		state.Plan.Metrics.GeneratedBytes = projectionBytes(state.Plan.Files)
		state.Receipt = makeReceipt(state, options.OutputDirectory)
		if err := validateManifestReceipt(state); err != nil {
			return Result{}, err
		}
		state.Rollback = makeRollback(state)
		state.Dossier = makeDossier(state)
	} else {
		state.Plan.OperationDecision = reduceFindings(state.Meta, state.Findings)
		state.Plan.PortfolioDecision = state.Plan.OperationDecision
		state.Plan.Findings = state.Findings
		state.Plan.BeforeDigest = state.BeforeDigest
		state.Plan.AfterDigest = state.BeforeDigest
		state.Plan.Metrics = emptyMetrics(state.Meta)
		state.Receipt = ReplayReceipt{Schema: "gooo/ledger-append-planner/replay-receipt/v1", TransactionID: transaction.TransactionID, State: state.Plan.OperationDecision, BeforeDigest: state.BeforeDigest, AfterDigest: state.BeforeDigest, ObservedAfterDigest: state.BeforeDigest, Mismatches: findingReasons(state.Findings), RollbackReady: false, RepositoryWrites: 0}
		state.Dossier = makeDossier(state)
		if err := materializeFindings(state, options.OutputDirectory); err != nil {
			return Result{}, err
		}
	}
	if err := writeArtifacts(state, options.OutputDirectory); err != nil {
		return Result{}, err
	}
	return Result{Plan: state.Plan, Receipt: state.Receipt, Rollback: state.Rollback, Dossier: state.Dossier, OutputDirectory: options.OutputDirectory}, nil
}

func resolveAuthorityPath(metaCodePath, declaredPath string) string {
	if filepath.IsAbs(declaredPath) {
		return declaredPath
	}
	metaRoot := filepath.Dir(filepath.Dir(metaCodePath))
	return filepath.Join(metaRoot, declaredPath)
}

func defaultManifestPath(transactionSchema, declaredPath string) string {
	if transactionSchema == "gooo/ledger-append-transaction/v3" {
		if strings.HasSuffix(declaredPath, "-v3.gooo") {
			return declaredPath
		}
		return ".gooo/append-transaction-manifest-v3.gooo"
	}
	if strings.HasSuffix(declaredPath, "-v2.gooo") {
		return declaredPath
	}
	return ".gooo/append-transaction-manifest-v2.gooo"
}

func loadTransaction(path string) (Transaction, map[string]any, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Transaction{}, nil, err
	}
	var transaction Transaction
	if err := decodeJSON(raw, &transaction); err != nil {
		return Transaction{}, nil, fmt.Errorf("decode transaction: %w", err)
	}
	if err := transaction.ValidateShape(); err != nil {
		return Transaction{}, nil, err
	}
	var rawValue map[string]any
	if err := decodeJSON(raw, &rawValue); err != nil {
		return Transaction{}, nil, fmt.Errorf("decode transaction object: %w", err)
	}
	return transaction, rawValue, nil
}

func loadASTFile(path string) (any, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var value any
	if err := decodeJSON(raw, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func validateExecution(state *runState, options Options) error {
	if _, err := executionOrder(state.Meta); err != nil {
		state.Findings = append(state.Findings, refutation("METACODE", "VALIDATE_OPERATION_GRAPH", "METACODE_GRAPH_INVALID", err.Error()))
	}
	validateManifestBinding(state)
	validateBaseline(state, options)
	validateTransaction(state)
	validateManifestTarget(state)
	validateManifestDeclaration(state)
	validateLedgerShape(state)
	if len(state.Findings) > 0 {
		state.Plan = basePlan(state)
		return nil
	}
	if err := validateExpectedCounts(state); err != nil {
		state.Findings = append(state.Findings, refutation("VALIDATE", "CHECK_EXPECTED_COUNTS", "SUPPLIED_COUNTS_DO_NOT_MATCH_AST", err.Error()))
	}
	if len(state.Findings) == 0 {
		state.Plan.PortfolioDecision = expectedPortfolioDecision(state)
	}
	return nil
}

func validateManifestBinding(state *runState) {
	if !hasTransactionManifest(state.Transaction.Schema) {
		return
	}
	if state.Manifest.Schema == "" {
		state.Findings = append(state.Findings, unknown("VERIFY", "LOAD_TRANSACTION_MANIFEST", "transaction manifest is unavailable", "TRANSACTION_MANIFEST_UNOBSERVED", "supply the declared .gooo transaction manifest", []string{"transaction-manifest"}))
		return
	}
	if state.Manifest.Operation != state.Meta.Operation || state.Manifest.TransactionSchema != state.Transaction.Schema {
		state.Findings = append(state.Findings, refutation("VALIDATE", "CHECK_TRANSACTION_MANIFEST_IDENTITY", "TRANSACTION_MANIFEST_IDENTITY_MISMATCH", "manifest operation or transaction schema differs from the executor contract"))
	}
	if state.Transaction.ManifestKey == "" {
		state.Findings = append(state.Findings, unknown("VERIFY", "BIND_TRANSACTION_MANIFEST_TARGET", "transaction manifest key is absent", "TRANSACTION_MANIFEST_BINDING_UNOBSERVED", "supply an exact immutable target binding key", []string{"manifest_key"}))
		return
	}
	binding, ok := state.Manifest.Targets[state.Transaction.ManifestKey]
	if !ok {
		state.Findings = append(state.Findings, unknown("VERIFY", "BIND_TRANSACTION_MANIFEST_TARGET", "transaction manifest target binding is absent", "TRANSACTION_MANIFEST_BINDING_UNKNOWN", "supply an exact immutable target binding for "+state.Transaction.ManifestKey, []string{"manifest_key", state.Transaction.ManifestKey}))
		return
	}
	state.TargetBinding = &binding
	tx := state.Transaction
	checks := []struct{ name, got, want string }{
		{"repository", tx.Baseline.Repository, binding.Repository},
		{"tag", tx.Baseline.Tag, binding.Tag},
		{"target_commit_sha", tx.Baseline.TargetCommitSHA, binding.TargetCommitSHA},
		{"target_before_digest", tx.Baseline.SourceTreeSHA256, binding.BeforeDigest},
	}
	for _, check := range checks {
		if check.got != check.want {
			state.Findings = append(state.Findings, refutation("VERIFY", "COMPARE_TRANSACTION_MANIFEST_BINDING", "TRANSACTION_MANIFEST_TARGET_MISMATCH", check.name+" differs from manifest target "+binding.Key))
		}
	}
}

func validateManifestDeclaration(state *runState) {
	if !hasTransactionManifest(state.Transaction.Schema) || state.Manifest.Schema == "" {
		return
	}
	if state.Transaction.Schema == "gooo/ledger-append-transaction/v3" {
		validateManifestDeclarationV3(state)
		return
	}
	for role, path := range state.Meta.Paths {
		if got, ok := state.Manifest.PlannedFiles[role]; !ok || got != path {
			state.Findings = append(state.Findings, refutation("VALIDATE", "CHECK_MANIFEST_PLANNED_FILE_SET", "MANIFEST_PLANNED_FILE_SET_MISMATCH", role))
		}
	}
	if len(state.Manifest.PlannedFiles) != len(state.Meta.Paths) {
		state.Findings = append(state.Findings, refutation("VALIDATE", "CHECK_MANIFEST_PLANNED_FILE_SET", "MANIFEST_PLANNED_FILE_SET_MISMATCH", fmt.Sprintf("manifest=%d declared=%d", len(state.Manifest.PlannedFiles), len(state.Meta.Paths))))
	}
	for _, invariant := range requiredManifestAfterInvariants {
		if !contains(state.Manifest.AfterInvariants, invariant) {
			state.Findings = append(state.Findings, refutation("VALIDATE", "CHECK_MANIFEST_AFTER_INVARIANTS", "MANIFEST_AFTER_INVARIANT_MISSING", invariant))
		}
	}
}

func validateManifestDeclarationV3(state *runState) {
	if state.Manifest.Schema != transactionManifestSchemaV3 {
		state.Findings = append(state.Findings, refutation("VALIDATE", "CHECK_TRANSACTION_MANIFEST_VERSION", "TRANSACTION_MANIFEST_VERSION_MISMATCH", state.Manifest.Schema))
		return
	}
	wantRoles := map[string]bool{}
	missingSemantic := false
	extraTarget := false
	for role, path := range state.Meta.Paths {
		wantRoles[role] = true
		target, ok := state.Manifest.FileTargets[role]
		if !ok {
			if state.Meta.PathKinds[role] == "DERIVED_PROJECTION" {
				appendProjectionAuthorityUnknown(state, role, path, []string{"target-file", "projection-kind", "projection-before-digest", "projection-source-semantic-digest", "projection-after-invariant"})
			} else {
				missingSemantic = true
				state.Findings = append(state.Findings, refutation("VALIDATE", "CHECK_MANIFEST_TARGET_SET", "MANIFEST_SEMANTIC_TARGET_MISSING", role))
			}
			continue
		}
		if target.Path != path {
			state.Findings = append(state.Findings, refutation("VALIDATE", "CHECK_MANIFEST_TARGET_SET", "MANIFEST_TARGET_PATH_MISMATCH", role+"="+target.Path))
		}
		if target.Kind != state.Meta.PathKinds[role] {
			state.Findings = append(state.Findings, refutation("VALIDATE", "CHECK_MANIFEST_TARGET_KIND", "MANIFEST_TARGET_KIND_MISMATCH", role+"="+target.Kind))
		}
		if target.Kind == "DERIVED_PROJECTION" {
			missing := []string{}
			if target.ProjectionKind == "" {
				missing = append(missing, "projection-kind")
			}
			if target.BeforeDigest == "" {
				missing = append(missing, "projection-before-digest")
			}
			if target.SourceSemanticDigest == "" {
				missing = append(missing, "projection-source-semantic-digest")
			}
			if target.AfterInvariant == "" {
				missing = append(missing, "projection-after-invariant")
			}
			if len(missing) > 0 {
				appendProjectionAuthorityUnknown(state, role, path, missing)
			} else if target.AfterInvariant != projectionAfterInvariant {
				state.Findings = append(state.Findings, refutation("VALIDATE", "CHECK_PROJECTION_AFTER_INVARIANT", "PROJECTION_AFTER_INVARIANT_UNSUPPORTED", role+"="+target.AfterInvariant))
			}
		}
	}
	for role := range state.Manifest.FileTargets {
		if !wantRoles[role] {
			extraTarget = true
			state.Findings = append(state.Findings, refutation("VALIDATE", "CHECK_MANIFEST_TARGET_SET", "MANIFEST_UNDECLARED_TARGET", role))
		}
	}
	if (missingSemantic || extraTarget) && len(state.Manifest.FileTargets) != len(state.Meta.Paths) {
		state.Findings = append(state.Findings, refutation("VALIDATE", "CHECK_MANIFEST_TARGET_SET", "MANIFEST_TARGET_SET_MISMATCH", fmt.Sprintf("manifest=%d declared=%d", len(state.Manifest.FileTargets), len(state.Meta.Paths))))
	}
	for _, invariant := range requiredManifestAfterInvariantsV3 {
		if !contains(state.Manifest.AfterInvariants, invariant) {
			state.Findings = append(state.Findings, refutation("VALIDATE", "CHECK_MANIFEST_AFTER_INVARIANTS", "MANIFEST_AFTER_INVARIANT_MISSING", invariant))
		}
	}
	for role, target := range state.Manifest.FileTargets {
		if target.Kind != "DERIVED_PROJECTION" || target.ProjectionKind == "" || target.BeforeDigest == "" || target.SourceSemanticDigest == "" || target.AfterInvariant == "" {
			continue
		}
		before, exists := state.BeforeFiles[target.Path]
		if !exists {
			state.Findings = append(state.Findings, refutation("VERIFY", "CHECK_PROJECTION_BEFORE_DIGEST", "UNEXPECTED_PROJECTION_DELETE", role+" at "+target.Path))
		} else if fileDigest(before) != target.BeforeDigest {
			state.Findings = append(state.Findings, refutation("VERIFY", "CHECK_PROJECTION_BEFORE_DIGEST", "PROJECTION_BEFORE_DIGEST_MISMATCH", role+" observed="+fileDigest(before)+" expected="+target.BeforeDigest))
		}
		if target.SourceSemanticDigest != state.BeforeDigest {
			state.Findings = append(state.Findings, refutation("VERIFY", "CHECK_PROJECTION_SOURCE_SEMANTIC_DIGEST", "PROJECTION_SOURCE_SEMANTIC_DIGEST_MISMATCH", role+" observed="+state.BeforeDigest+" expected="+target.SourceSemanticDigest))
		}
	}
}

func appendProjectionAuthorityUnknown(state *runState, role, path string, blocked []string) {
	state.Findings = append(state.Findings, unknown("VERIFY", "BIND_DERIVED_PROJECTION", "derived projection authority is unavailable for "+role+" at "+path, "DERIVED_PROJECTION_AUTHORITY_UNOBSERVED", "supply the exact before digest, projection kind, source semantic digest, and after invariant", blocked))
}

func validateManifestTarget(state *runState) {
	if !hasTransactionManifest(state.Transaction.Schema) || state.TargetBinding == nil {
		return
	}
	binding := state.TargetBinding
	profile, err := object(state.Ledger.Profile.AST, "profile")
	if err != nil {
		return
	}
	profileCells, err := array(profile["cells"], "profile.cells")
	if err != nil {
		return
	}
	if len(profileCells) != binding.ExpectedDenominator || intValue(profile["total_cells"]) != int64(binding.ExpectedDenominator) {
		state.Findings = append(state.Findings, refutation("VERIFY", "CHECK_TARGET_BEFORE_DENOMINATOR", "TARGET_BEFORE_DENOMINATOR_MISMATCH", fmt.Sprintf("observed=%d expected=%d", len(profileCells), binding.ExpectedDenominator)))
	}
	assessment, err := object(state.Ledger.Assessment.AST, "assessment")
	if err != nil {
		return
	}
	assessmentCells, err := array(assessment["cells"], "assessment.cells")
	if err != nil {
		return
	}
	status := map[string]int{}
	for _, stateName := range state.Meta.States {
		status[stateName] = 0
	}
	for _, raw := range assessmentCells {
		cell, ok := raw.(map[string]any)
		if ok {
			status[stringValue(cell["state"])]++
		}
	}
	if !sameIntMap(status, binding.ExpectedStateCounts) {
		state.Findings = append(state.Findings, refutation("VERIFY", "CHECK_TARGET_BEFORE_STATE_COUNTS", "TARGET_BEFORE_STATE_COUNTS_MISMATCH", fmt.Sprintf("observed=%v expected=%v", status, binding.ExpectedStateCounts)))
	}
	anchors := binding.Anchors
	if len(profileCells) > 0 {
		first, _ := profileCells[0].(map[string]any)
		last, _ := profileCells[len(profileCells)-1].(map[string]any)
		checkManifestAnchor(state, anchors, "profile.first_cell_id", stringValue(first["id"]))
		checkManifestAnchor(state, anchors, "profile.last_cell_id", stringValue(last["id"]))
		checkManifestAnchor(state, anchors, "release_map.last_key", stringValue(last["release_key"]))
	}
	if len(assessmentCells) > 0 {
		last, _ := assessmentCells[len(assessmentCells)-1].(map[string]any)
		checkManifestAnchor(state, anchors, "assessment.last_cell_id", stringValue(last["cell_id"]))
	}
	registry, err := object(state.Ledger.Registry.AST, "registry")
	if err == nil {
		entries, entryErr := array(registry["entries"], "registry.entries")
		if entryErr == nil && len(entries) > 0 {
			last, _ := entries[len(entries)-1].(map[string]any)
			checkManifestAnchor(state, anchors, "registry.last_entry_id", stringValue(last["entry_id"]))
		}
	}
	activities := parseActivitySequence(state.Ledger.ActivityRaw)
	if len(activities) > 0 {
		checkManifestAnchor(state, anchors, "activity.last_activity", activities[len(activities)-1])
	}
}

func checkManifestAnchor(state *runState, anchors map[string]string, key, observed string) {
	want, ok := anchors[key]
	if !ok || want == "" {
		state.Findings = append(state.Findings, refutation("VERIFY", "CHECK_TARGET_STRUCTURAL_ANCHORS", "MANIFEST_STRUCTURAL_ANCHOR_MISSING", key))
		return
	}
	if observed != want {
		state.Findings = append(state.Findings, refutation("VERIFY", "CHECK_TARGET_STRUCTURAL_ANCHORS", "TARGET_STRUCTURAL_ANCHOR_MISMATCH", key+" observed="+observed+" expected="+want))
	}
}

func validateBaseline(state *runState, options Options) {
	tx := state.Transaction
	lock := state.BaselineLock
	missing := false
	if tx.Baseline.Repository == "" || tx.Baseline.Tag == "" || tx.Baseline.ReleaseID == 0 || tx.Baseline.TagObjectSHA == "" || tx.Baseline.TargetCommitSHA == "" || tx.Baseline.SourceArchiveSHA256 == "" || tx.Baseline.SourceTreeSHA256 == "" || tx.Baseline.ReleaseAssetID == 0 || tx.Baseline.ReleaseAssetSHA256 == "" || tx.Baseline.Immutable == nil {
		missing = true
	}
	if missing {
		state.Findings = append(state.Findings, unknown("VERIFY", "BIND_IMMUTABLE_BASELINE", "immutable baseline lock is missing required fields", "MISSING_IMMUTABLE_LOCK", "supply the v0.31.0 REST release/tag/source lock and retry", []string{"baseline-lock", "source-digest"}))
		return
	}
	if !*tx.Baseline.Immutable {
		state.Findings = append(state.Findings, refutation("VERIFY", "CHECK_IMMUTABLE_BASELINE", "BASELINE_NOT_IMMUTABLE", "baseline.immutable=false"))
		return
	}
	checks := []struct{ name, got, want string }{
		{"repository", tx.Baseline.Repository, stringValue(lock["repository"])},
		{"tag", tx.Baseline.Tag, stringValue(lock["tag"])},
		{"tag_object_sha", tx.Baseline.TagObjectSHA, stringValue(lock["tag_object_sha"])},
		{"target_commit_sha", tx.Baseline.TargetCommitSHA, stringValue(lock["target_commit_sha"])},
		{"source_tree_sha256", tx.Baseline.SourceTreeSHA256, stringValue(lock["source_tree_sha256"])},
	}
	if tx.Baseline.ReleaseID != intValue(lock["release_id"]) || tx.Baseline.ReleaseAssetID != nestedInt(lock, "release_asset", "id") || tx.Baseline.SourceArchiveSHA256 != nestedString(lock, "source_archive", "sha256") || tx.Baseline.ReleaseAssetSHA256 != nestedString(lock, "release_asset", "sha256") {
		state.Findings = append(state.Findings, refutation("VERIFY", "COMPARE_BASELINE_LOCK", "BASELINE_LOCK_MISMATCH", "release/tag/source digest identity differs from the immutable lock"))
		return
	}
	for _, check := range checks {
		if check.got != check.want {
			state.Findings = append(state.Findings, refutation("VERIFY", "COMPARE_BASELINE_LOCK", "BASELINE_LOCK_MISMATCH", check.name+" differs from immutable lock"))
		}
	}
	if lock["immutable"] != true {
		state.Findings = append(state.Findings, refutation("VERIFY", "CHECK_RELEASE_API_IMMUTABILITY", "BASELINE_NOT_IMMUTABLE", "baseline lock does not record immutable=true"))
	}
	if options.SourceArchivePath != "" {
		data, err := os.ReadFile(options.SourceArchivePath)
		if err != nil {
			state.Findings = append(state.Findings, unknown("VERIFY", "READ_SOURCE_ARCHIVE", "source archive could not be read", "SOURCE_ARCHIVE_UNAVAILABLE", "provide the exact GitHub API zipball", []string{"source-archive"}))
		} else if fileDigest(data) != tx.Baseline.SourceArchiveSHA256 {
			state.Findings = append(state.Findings, refutation("VERIFY", "CHECK_SOURCE_ARCHIVE_DIGEST", "SOURCE_ARCHIVE_DIGEST_MISMATCH", fileDigest(data)))
		}
	}
	if options.ReleaseAssetPath != "" {
		data, err := os.ReadFile(options.ReleaseAssetPath)
		if err != nil {
			state.Findings = append(state.Findings, unknown("VERIFY", "READ_RELEASE_ASSET", "release evidence asset could not be read", "RELEASE_ASSET_UNAVAILABLE", "provide asset 538664422", []string{"release-asset"}))
		} else if fileDigest(data) != tx.Baseline.ReleaseAssetSHA256 {
			state.Findings = append(state.Findings, refutation("VERIFY", "CHECK_RELEASE_ASSET_DIGEST", "RELEASE_ASSET_DIGEST_MISMATCH", fileDigest(data)))
		}
	}
	gotTree, err := sourceTreeDigest(options.InputRepository)
	if err != nil {
		state.Findings = append(state.Findings, unknown("VERIFY", "HASH_SOURCE_TREE", "source tree digest could not be observed", "SOURCE_TREE_UNOBSERVED", "retry source-tree hashing", []string{"source-tree"}))
	} else if gotTree != tx.Baseline.SourceTreeSHA256 {
		state.Findings = append(state.Findings, refutation("VERIFY", "COMPARE_SOURCE_TREE_DIGEST", "SOURCE_TREE_DIGEST_MISMATCH", "observed="+gotTree+" expected="+tx.Baseline.SourceTreeSHA256))
	}
}

func validateTransaction(state *runState) {
	tx := state.Transaction
	for _, required := range state.Meta.Required {
		if !rawPathPresent(state.TransactionRaw, required) {
			if strings.HasPrefix(required, "baseline.") || required == "release_lock" {
				state.Findings = append(state.Findings, unknown("VALIDATE", "CHECK_REQUIRED_TRANSACTION_FIELD", "required immutable evidence is missing: "+required, "REQUIRED_EVIDENCE_UNOBSERVED", "supply the missing transaction evidence and retry", []string{required}))
			} else {
				state.Findings = append(state.Findings, refutation("VALIDATE", "CHECK_REQUIRED_TRANSACTION_FIELD", "REQUIRED_TRANSACTION_FIELD_MISSING", required))
			}
		}
	}
	if tx.InsertionStrategy != "" && tx.InsertionStrategy != "ast" {
		state.Findings = append(state.Findings, refutation("VALIDATE", "REJECT_TEXTUAL_INSERTION", "AMBIGUOUS_TEXTUAL_INSERTION", "insertion_strategy="+tx.InsertionStrategy))
	}
	if tx.Cell.ID == "" || tx.Cell.Axis == "" || tx.Cell.Proof == "" || tx.Cell.Indicator == "" || tx.Cell.Activity == "" || tx.Cell.Source == "" || tx.Cell.IR == "" || tx.Cell.GeneratedArtifact == "" || tx.Cell.Evaluator == "" || tx.Cell.MetricID == "" || tx.Cell.ReleaseKey == "" {
		state.Findings = append(state.Findings, refutation("VALIDATE", "CHECK_NEW_CELL_FIELDS", "CELL_FIELDS_INCOMPLETE", "new cell is not a complete binding"))
	}
	if !contains(state.Meta.ProofClasses, tx.Cell.Proof) {
		state.Findings = append(state.Findings, refutation("VALIDATE", "CHECK_NEW_CELL_CLASSIFICATION", "INVALID_PROOF_CLASS", tx.Cell.Proof))
	}
	if !contains(state.Meta.IndicatorClasses, tx.Cell.Indicator) {
		state.Findings = append(state.Findings, refutation("VALIDATE", "CHECK_NEW_CELL_CLASSIFICATION", "INVALID_INDICATOR_CLASS", tx.Cell.Indicator))
	}
	if tx.Cell.MetricDenominator != 1 {
		state.Findings = append(state.Findings, refutation("VALIDATE", "CHECK_METRIC_DENOMINATOR", "METRIC_DENOMINATOR_NOT_ONE", fmt.Sprint(tx.Cell.MetricDenominator)))
	}
	if tx.Activity.Name == "" || tx.Activity.InputType == "" || tx.Activity.OutputType == "" || tx.Activity.Computes == "" {
		state.Findings = append(state.Findings, refutation("VALIDATE", "CHECK_NEW_ACTIVITY", "ACTIVITY_FIELDS_INCOMPLETE", "activity declaration is incomplete"))
	}
	if tx.Activity.Name != tx.Cell.Activity {
		state.Findings = append(state.Findings, refutation("VALIDATE", "CHECK_CELL_ACTIVITY_BINDING", "CELL_ACTIVITY_MISMATCH", tx.Cell.Activity+" != "+tx.Activity.Name))
	}
	if tx.ReleaseKey != "" && tx.ReleaseKey != tx.Cell.ReleaseKey {
		state.Findings = append(state.Findings, refutation("VALIDATE", "CHECK_RELEASE_KEY_BINDING", "LOCK_MISMATCH", tx.ReleaseKey+" != "+tx.Cell.ReleaseKey))
	}
	if tx.ReleaseLock == nil {
		state.Findings = append(state.Findings, unknown("VERIFY", "BIND_NEW_RELEASE_LOCK", "new release lock is absent", "RELEASE_LOCK_UNOBSERVED", "supply an immutable release lock at the requested map key", []string{"release-lock", tx.Cell.ReleaseKey}))
	} else {
		validateReleaseLock(state, tx.ReleaseLock, tx.Cell.ReleaseKey)
	}
	if tx.Outcome.State == "" || !contains(state.Meta.States, tx.Outcome.State) {
		state.Findings = append(state.Findings, refutation("VALIDATE", "CHECK_OUTCOME_STATE", "INVALID_OUTCOME_STATE", tx.Outcome.State))
	}
	if tx.Outcome.State == DecisionUnknown {
		if tx.Outcome.Unknown == nil || !validUnknown(state.Meta, *tx.Outcome.Unknown) {
			state.Findings = append(state.Findings, refutation("VALIDATE", "CHECK_UNKNOWN_FRONTIER", "UNKNOWN_FRONTIER_INCOMPLETE", "UNKNOWN requires all declared metacode frontier fields"))
		}
	}
	if tx.Outcome.State == DecisionRefuted && tx.Outcome.Refutation == nil {
		state.Findings = append(state.Findings, refutation("VALIDATE", "CHECK_REFUTATION", "REFUTATION_MISSING", "REFUTED outcome requires a reason"))
	}
	if tx.RegistryEntry == nil {
		state.Findings = append(state.Findings, refutation("VALIDATE", "CHECK_REGISTRY_ENTRY", "REGISTRY_ENTRY_MISSING", "the append must project one registry entry"))
	}
	if tx.Migration.Operation != "" && (tx.Migration.Operation != state.Meta.Migration.Operation || tx.Migration.Add != state.Meta.Migration.Add || tx.Migration.Retire != state.Meta.Migration.Retire || tx.Migration.Split != state.Meta.Migration.Split) {
		state.Findings = append(state.Findings, refutation("VALIDATE", "CHECK_MIGRATION", "MIGRATION_CONTRACT_MISMATCH", "transaction migration differs from metacode"))
	}
}

func validateReleaseLock(state *runState, lock map[string]any, key string) {
	for _, field := range state.Meta.ReleaseLockFields {
		if _, ok := lock[field]; !ok {
			state.Findings = append(state.Findings, unknown("VERIFY", "BIND_NEW_RELEASE_LOCK", "new release lock is missing "+field, "RELEASE_LOCK_FIELD_UNOBSERVED", "supply the exact REST release/tag identity", []string{"release-lock", key, field}))
		}
	}
	if lock["immutable"] == false {
		state.Findings = append(state.Findings, refutation("VERIFY", "CHECK_NEW_RELEASE_IMMUTABILITY", "RELEASE_NOT_IMMUTABLE", key))
	}
	if releaseID := intValue(lock["release_id"]); releaseID <= 0 {
		state.Findings = append(state.Findings, refutation("VERIFY", "CHECK_NEW_RELEASE_ID", "RELEASE_ID_INVALID", fmt.Sprint(lock["release_id"])))
	}
}

func validateLedgerShape(state *runState) {
	profile, err := object(state.Ledger.Profile.AST, "profile")
	if err != nil {
		state.Findings = append(state.Findings, refutation("PARSE", "READ_PROFILE_AST", "PROFILE_NOT_OBJECT", err.Error()))
		return
	}
	locks, err := object(state.Ledger.Locks.AST, "release locks")
	if err != nil {
		state.Findings = append(state.Findings, refutation("PARSE", "READ_RELEASE_LOCK_AST", "RELEASE_LOCKS_NOT_OBJECT", err.Error()))
		return
	}
	assessment, err := object(state.Ledger.Assessment.AST, "assessment")
	if err != nil {
		state.Findings = append(state.Findings, refutation("PARSE", "READ_ASSESSMENT_AST", "ASSESSMENT_NOT_OBJECT", err.Error()))
		return
	}
	registry, err := object(state.Ledger.Registry.AST, "registry")
	if err != nil {
		state.Findings = append(state.Findings, refutation("PARSE", "READ_REGISTRY_AST", "REGISTRY_NOT_OBJECT", err.Error()))
		return
	}
	profileCells, err := array(profile["cells"], "profile.cells")
	if err != nil {
		state.Findings = append(state.Findings, refutation("PARSE", "READ_PROFILE_CELLS", "PROFILE_CELLS_NOT_ARRAY", err.Error()))
		return
	}
	if intValue(profile["total_cells"]) != int64(len(profileCells)) {
		state.Findings = append(state.Findings, refutation("VALIDATE", "CHECK_PROFILE_DENOMINATOR", "PROFILE_DENOMINATOR_INCONSISTENT", fmt.Sprintf("total_cells=%d cells=%d", intValue(profile["total_cells"]), len(profileCells))))
	}
	if state.Transaction.Migration.From != 0 && (state.Transaction.Migration.From != int(len(profileCells)) || state.Transaction.Migration.To != int(len(profileCells))+1) {
		state.Findings = append(state.Findings, refutation("VALIDATE", "CHECK_MIGRATION", "DENOMINATOR_DECREASE_OR_WRONG_BASE", fmt.Sprintf("from=%d to=%d current=%d", state.Transaction.Migration.From, state.Transaction.Migration.To, len(profileCells))))
	}
	if migration, ok := profile["denominator_migration"].(map[string]any); !ok || intValue(migration["to"]) != int64(len(profileCells)) {
		state.Findings = append(state.Findings, refutation("VALIDATE", "CHECK_PROFILE_MIGRATION", "PROFILE_MIGRATION_INCONSISTENT", "profile denominator migration does not end at current total"))
	}
	validateUniqueCellFields(state, profileCells)
	releaseMap, err := object(locks["releases"], "release locks.releases")
	if err != nil {
		state.Findings = append(state.Findings, refutation("PARSE", "READ_RELEASE_MAP", "RELEASE_MAP_NOT_OBJECT", err.Error()))
	} else if _, exists := releaseMap[state.Transaction.Cell.ReleaseKey]; exists {
		state.Findings = append(state.Findings, refutation("VALIDATE", "CHECK_RELEASE_KEY_UNIQUENESS", "DUPLICATE_OR_OVERWRITE", state.Transaction.Cell.ReleaseKey))
	}
	assessmentCells, err := array(assessment["cells"], "assessment.cells")
	if err != nil {
		state.Findings = append(state.Findings, refutation("PARSE", "READ_ASSESSMENT_CELLS", "ASSESSMENT_CELLS_NOT_ARRAY", err.Error()))
	} else {
		validateUniqueAssessmentCells(state, assessmentCells)
	}
	registryEntries, err := array(registry["entries"], "registry.entries")
	if err != nil {
		state.Findings = append(state.Findings, refutation("PARSE", "READ_REGISTRY_ENTRIES", "REGISTRY_ENTRIES_NOT_ARRAY", err.Error()))
	} else {
		validateUniqueRegistryEntries(state, registryEntries)
		if _, exists := registryEntryByID(registryEntries, stringValue(state.Transaction.RegistryEntry["entry_id"])); exists {
			state.Findings = append(state.Findings, refutation("VALIDATE", "CHECK_REGISTRY_ID_UNIQUENESS", "DUPLICATE_OR_OVERWRITE", stringValue(state.Transaction.RegistryEntry["entry_id"])))
		}
	}
	activityNames := parseActivityNames(state.Ledger.ActivityRaw)
	if _, exists := activityNames[state.Transaction.Activity.Name]; exists {
		state.Findings = append(state.Findings, refutation("VALIDATE", "CHECK_ACTIVITY_ID_UNIQUENESS", "DUPLICATE_OR_OVERWRITE", state.Transaction.Activity.Name))
	}
	if len(activityNames) != len(profileCells) {
		state.Findings = append(state.Findings, refutation("VALIDATE", "CHECK_ACTIVITY_CELL_CARDINALITY", "ACTIVITY_CELL_CARDINALITY_MISMATCH", fmt.Sprintf("activities=%d cells=%d", len(activityNames), len(profileCells))))
	}
	for role, kind := range state.Meta.PathKinds {
		if kind != "DERIVED_PROJECTION" {
			continue
		}
		path := state.Meta.Paths[role]
		if _, exists := state.BeforeFiles[path]; !exists {
			continue
		}
		if state.Transaction.Schema == "gooo/ledger-append-transaction/v3" {
			target, declared := state.Manifest.FileTargets[role]
			if declared && target.Kind == "DERIVED_PROJECTION" && target.ProjectionKind != "" && target.BeforeDigest != "" && target.SourceSemanticDigest != "" && target.AfterInvariant != "" {
				continue
			}
			// validateManifestDeclarationV3 supplies the six-field UNKNOWN frontier
			// for an absent or incomplete projection authority.
			continue
		}
		state.Findings = append(state.Findings, refutation("VALIDATE", "CHECK_PROJECTION_TARGET", "NO_OVERWRITE", path))
	}
}

func validateUniqueCellFields(state *runState, cells []any) {
	ids, axes, activities, metrics := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, raw := range cells {
		cell, ok := raw.(map[string]any)
		if !ok {
			state.Findings = append(state.Findings, refutation("VALIDATE", "CHECK_EXISTING_CELL", "CELL_NOT_OBJECT", "profile cell is not an object"))
			continue
		}
		for key, seen := range map[string]map[string]bool{"id": ids, "axis": axes, "activity": activities, "metric_id": metrics} {
			value := stringValue(cell[key])
			if value == "" || seen[value] {
				state.Findings = append(state.Findings, refutation("VALIDATE", "CHECK_EXISTING_CELL_UNIQUENESS", "DUPLICATE_OR_MISSING_CELL_IDENTITY", key+"="+value))
			}
			seen[value] = true
		}
	}
	if ids[state.Transaction.Cell.ID] || axes[state.Transaction.Cell.Axis] || activities[state.Transaction.Cell.Activity] || metrics[state.Transaction.Cell.MetricID] {
		state.Findings = append(state.Findings, refutation("VALIDATE", "CHECK_NEW_CELL_UNIQUENESS", "DUPLICATE_OR_OVERWRITE", state.Transaction.Cell.ID))
	}
}

func validateUniqueAssessmentCells(state *runState, cells []any) {
	seen := map[string]bool{}
	for _, raw := range cells {
		cell, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		id := stringValue(cell["cell_id"])
		if id == "" || seen[id] {
			state.Findings = append(state.Findings, refutation("VALIDATE", "CHECK_ASSESSMENT_UNIQUENESS", "DUPLICATE_ASSESSMENT_CELL", id))
		}
		seen[id] = true
	}
}

func validateUniqueRegistryEntries(state *runState, entries []any) {
	seen := map[string]bool{}
	for _, raw := range entries {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		id := stringValue(entry["entry_id"])
		if id == "" || seen[id] {
			state.Findings = append(state.Findings, refutation("VALIDATE", "CHECK_REGISTRY_UNIQUENESS", "DUPLICATE_REGISTRY_ENTRY", id))
		}
		seen[id] = true
	}
}

func rawPathPresent(root map[string]any, path string) bool {
	parts := strings.Split(path, ".")
	var current any = root
	for _, part := range parts {
		object, ok := current.(map[string]any)
		if !ok {
			return false
		}
		value, ok := object[part]
		if !ok || value == nil {
			return false
		}
		current = value
	}
	return true
}

func validUnknown(meta MetaCode, value UnknownFrontier) bool {
	fields := map[string]any{
		"stage":          value.Stage,
		"step":           value.Step,
		"reason":         value.Reason,
		"unknown_class":  value.UnknownClass,
		"next_operation": value.NextOperation,
		"blocked_by":     value.BlockedBy,
	}
	for _, field := range meta.UnknownFields {
		current, ok := fields[field]
		if !ok {
			return false
		}
		switch typed := current.(type) {
		case string:
			if typed == "" {
				return false
			}
		case []string:
			if len(typed) == 0 {
				return false
			}
		}
	}
	return true
}

func validateExpectedCounts(state *runState) error {
	profile, _ := object(state.Ledger.Profile.AST, "profile")
	cells, _ := array(profile["cells"], "profile.cells")
	proof, indicator := map[string]int{}, map[string]int{}
	for _, raw := range cells {
		cell, _ := raw.(map[string]any)
		proof[stringValue(cell["proof"])]++
		indicator[stringValue(cell["indicator"])]++
	}
	proof[state.Transaction.Cell.Proof]++
	indicator[state.Transaction.Cell.Indicator]++
	if !sameIntMap(proof, state.Transaction.Expected.ProofTotals) {
		return fmt.Errorf("proof totals observed=%v supplied=%v", proof, state.Transaction.Expected.ProofTotals)
	}
	if !sameIntMap(indicator, state.Transaction.Expected.IndicatorTotals) {
		return fmt.Errorf("indicator totals observed=%v supplied=%v", indicator, state.Transaction.Expected.IndicatorTotals)
	}
	assessment, _ := object(state.Ledger.Assessment.AST, "assessment")
	assessmentCells, _ := array(assessment["cells"], "assessment.cells")
	status := map[string]int{}
	for _, stateName := range state.Meta.States {
		status[stateName] = 0
	}
	for _, raw := range assessmentCells {
		cell, _ := raw.(map[string]any)
		status[stringValue(cell["state"])]++
	}
	status[state.Transaction.Outcome.State]++
	if !sameIntMap(status, state.Transaction.Expected.StatusCounts) {
		return fmt.Errorf("status counts observed=%v supplied=%v", status, state.Transaction.Expected.StatusCounts)
	}
	if expected := expectedPortfolioDecision(state); expected != state.Transaction.Expected.PortfolioDecision {
		return fmt.Errorf("portfolio decision observed=%s supplied=%s", expected, state.Transaction.Expected.PortfolioDecision)
	}
	return nil
}

func expectedPortfolioDecision(state *runState) string {
	counts := state.Transaction.Expected.StatusCounts
	for _, status := range state.Meta.Precedence {
		if counts[status] > 0 {
			return status
		}
	}
	return state.Meta.TerminalState
}

func basePlan(state *runState) PatchPlan {
	tx := state.Transaction
	decision := reduceFindings(state.Meta, state.Findings)
	plan := PatchPlan{
		Schema: "gooo/ledger-append-planner/patch-plan/v1", TransactionID: tx.TransactionID,
		Operation: state.Meta.Operation, OperationDecision: decision, PortfolioDecision: decision,
		Findings: state.Findings, Baseline: tx.Baseline, Migration: tx.Migration,
		NewCell: tx.Cell, NewActivity: tx.Activity, ReleaseKey: tx.Cell.ReleaseKey,
		ProofTotals: tx.Expected.ProofTotals, IndicatorTotals: tx.Expected.IndicatorTotals,
		StatusCounts: tx.Expected.StatusCounts, Authority: authorityValue(state.Meta),
		Process: state.Meta.Process, Claims: state.Meta.Claims, InputRepositoryMutated: false,
		BeforeDigest: state.BeforeDigest,
	}
	if hasTransactionManifest(tx.Schema) {
		plan.ManifestKey = tx.ManifestKey
		plan.ManifestAfterInvariants = append([]string(nil), state.Manifest.AfterInvariants...)
		if tx.Schema == "gooo/ledger-append-transaction/v2" {
			plan.ManifestPlannedFiles = manifestPlannedPaths(state.Manifest.PlannedFiles)
		} else {
			plan.ManifestPlannedFiles = manifestTargetPaths(state.Manifest.FileTargets)
			plan.ManifestFileTargets = manifestFileTargets(state.Manifest.FileTargets)
		}
		if state.TargetBinding != nil {
			plan.TargetBeforeDigest = state.TargetBinding.BeforeDigest
		}
	}
	return plan
}

func manifestPlannedPaths(planned map[string]string) []string {
	paths := make([]string, 0, len(planned))
	for _, path := range planned {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func manifestTargetPaths(targets map[string]ManifestFileTarget) []string {
	paths := make([]string, 0, len(targets))
	for _, target := range targets {
		paths = append(paths, target.Path)
	}
	sort.Strings(paths)
	return paths
}

func manifestFileTargets(targets map[string]ManifestFileTarget) []ManifestFileTarget {
	roles := make([]string, 0, len(targets))
	for role := range targets {
		roles = append(roles, role)
	}
	sort.Strings(roles)
	result := make([]ManifestFileTarget, 0, len(roles))
	for _, role := range roles {
		result = append(result, targets[role])
	}
	return result
}

func buildPatch(state *runState) error {
	tx := state.Transaction
	profile, _ := object(state.Ledger.Profile.AST, "profile")
	locks, _ := object(state.Ledger.Locks.AST, "release locks")
	assessment, _ := object(state.Ledger.Assessment.AST, "assessment")
	registry, _ := object(state.Ledger.Registry.AST, "registry")
	profileAfter, err := cloneJSON(profile)
	if err != nil {
		return err
	}
	locksAfter, err := cloneJSON(locks)
	if err != nil {
		return err
	}
	assessmentAfter, err := cloneJSON(assessment)
	if err != nil {
		return err
	}
	registryAfter, err := cloneJSON(registry)
	if err != nil {
		return err
	}
	profileMap := profileAfter.(map[string]any)
	profileCells, _ := array(profileMap["cells"], "profile.cells")
	cellValue, _ := toJSONValue(tx.Cell)
	profileCells = append(profileCells, cellValue)
	profileMap["cells"] = profileCells
	oldTotal := intValue(profile["total_cells"])
	if state.TargetBinding != nil && oldTotal != int64(state.TargetBinding.ExpectedDenominator) {
		return fmt.Errorf("manifest target denominator changed during planning: observed=%d expected=%d", oldTotal, state.TargetBinding.ExpectedDenominator)
	}
	if tx.Cell.Ordinal == 0 {
		tx.Cell.Ordinal = int(oldTotal + 1)
		state.Transaction.Cell.Ordinal = tx.Cell.Ordinal
		state.Plan.NewCell.Ordinal = tx.Cell.Ordinal
		profileCells[len(profileCells)-1].(map[string]any)["ordinal"] = json.Number(fmt.Sprint(tx.Cell.Ordinal))
	}
	profileMap["total_cells"] = json.Number(fmt.Sprint(oldTotal + 1))
	profileMap["proof_totals"] = intMapValue(tx.Expected.ProofTotals)
	profileMap["indicator_totals"] = intMapValue(tx.Expected.IndicatorTotals)
	profileMap["denominator_migration"] = migrationValue(oldTotal, oldTotal+1, state.Meta.Migration)

	locksMap := locksAfter.(map[string]any)
	releaseMap, _ := object(locksMap["releases"], "release locks.releases")
	releaseMap[tx.Cell.ReleaseKey] = tx.ReleaseLock
	locksMap["releases"] = releaseMap

	assessmentMap := assessmentAfter.(map[string]any)
	assessmentCells, _ := array(assessmentMap["cells"], "assessment.cells")
	assessmentCell, _ := assessmentJSONValue(tx)
	assessmentMap["cells"] = append(assessmentCells, assessmentCell)
	assessmentMap["denominator_migration"] = migrationValue(intValue(assessment["denominator_migration"].(map[string]any)["to"]), oldTotal+1, state.Meta.Migration)

	registryMap := registryAfter.(map[string]any)
	registryEntries, _ := array(registryMap["entries"], "registry.entries")
	registryMap["entries"] = append(registryEntries, tx.RegistryEntry)
	registryMap["entry_count"] = json.Number(fmt.Sprint(len(registryEntries) + 1))
	frontier, _ := array(registryMap["frontier_additions"], "registry.frontier_additions")
	frontier = append(frontier, stringValue(tx.RegistryEntry["entry_id"]))
	registryMap["frontier_additions"] = frontier

	activityAfter := appendActivity(state.Ledger.ActivityRaw, tx.Activity)
	afterCoreFiles := cloneFiles(state.BeforeFiles)
	profileBytes, err := jsonBytes(profileAfter)
	if err != nil {
		return err
	}
	locksBytes, err := jsonBytes(locksAfter)
	if err != nil {
		return err
	}
	assessmentBytes, err := jsonBytes(assessmentAfter)
	if err != nil {
		return err
	}
	registryBytes, err := jsonBytes(registryAfter)
	if err != nil {
		return err
	}
	afterCoreFiles[state.Ledger.Profile.Path] = profileBytes
	afterCoreFiles[state.Ledger.Locks.Path] = locksBytes
	afterCoreFiles[state.Ledger.Assessment.Path] = assessmentBytes
	afterCoreFiles[state.Ledger.Registry.Path] = registryBytes
	afterCoreFiles[state.Ledger.ActivityFile] = activityAfter
	state.AfterFiles = afterCoreFiles
	state.AfterDigest, err = subjectDigest(state.Ledger, afterCoreFiles, state.Meta)
	if err != nil {
		return err
	}
	if err := validateCanonicalPreservation(state, profile, locks, assessment, registry, profileAfter.(map[string]any), locksAfter.(map[string]any), assessmentAfter.(map[string]any), registryAfter.(map[string]any), activityAfter); err != nil {
		return err
	}
	state.Plan = basePlan(state)
	state.Plan.OperationDecision = state.Meta.TerminalState
	state.Plan.PortfolioDecision = expectedPortfolioDecision(state)
	state.Plan.BeforeDigest = state.BeforeDigest
	state.Plan.AfterDigest = state.AfterDigest
	state.Plan.NewCell = tx.Cell
	state.Plan.Migration = migrationValueStruct(oldTotal, oldTotal+1, state.Meta.Migration)
	state.Plan.Files = []FileMutation{
		mutation(state.Ledger.ActivityFile, state.BeforeFiles[state.Ledger.ActivityFile], activityAfter, "append", []string{"activity:" + tx.Activity.Name}, "append one Gooo activity at end of the source AST"),
		mutation(state.Ledger.Profile.Path, state.BeforeFiles[state.Ledger.Profile.Path], profileBytes, "append", []string{"/cells/-1", "/total_cells", "/denominator_migration", "/proof_totals", "/indicator_totals"}, "append one profile cell and advance the fixed denominator"),
		mutation(state.Ledger.Locks.Path, state.BeforeFiles[state.Ledger.Locks.Path], locksBytes, "append", []string{"/releases/" + tx.Cell.ReleaseKey}, "insert one release lock by map key"),
		mutation(state.Ledger.Assessment.Path, state.BeforeFiles[state.Ledger.Assessment.Path], assessmentBytes, "append", []string{"/cells/-1", "/denominator_migration"}, "append one assessment outcome"),
		mutation(state.Ledger.Registry.Path, state.BeforeFiles[state.Ledger.Registry.Path], registryBytes, "append", []string{"/entries/-1", "/entry_count", "/frontier_additions/-1"}, "append one capability-evidence registry entry"),
	}
	state.Plan.Metrics = initialMetrics(state.Meta, state.Plan.Files, state.BeforeFiles, state.AfterFiles)
	return nil
}

func renderAndFinalize(state *runState, options Options) error {
	baseFiles := state.Plan.Files
	if state.Transaction.Schema == "gooo/ledger-append-transaction/v3" {
		return renderAndFinalizeV3(state, baseFiles)
	}
	context := templateContext(state, baseFiles)
	var report, history []byte
	for iteration := 0; iteration < 8; iteration++ {
		var err error
		report, err = renderTemplate(state.Meta, "report", context)
		if err != nil {
			return err
		}
		history, err = renderTemplate(state.Meta, "history", context)
		if err != nil {
			return err
		}
		newBytes := int64(len(report) + len(history))
		if state.Plan.Metrics.GeneratedBytes == newBytes {
			break
		}
		state.Plan.Metrics.GeneratedBytes = newBytes
		context = templateContext(state, baseFiles)
	}
	if !json.Valid(report) || !json.Valid(history) {
		return fmt.Errorf("projection template emitted invalid JSON")
	}
	state.AfterFiles[state.Meta.Paths["report"]] = report
	state.AfterFiles[state.Meta.Paths["history"]] = history
	state.Plan.Files = append(baseFiles,
		mutation(state.Meta.Paths["report"], nil, report, "create", []string{"/"}, "render assessment report from the metacode template"),
		mutation(state.Meta.Paths["history"], nil, history, "create", []string{"/events/-1"}, "render append-only history projection from the metacode template"),
	)
	state.Plan.Metrics.GeneratedFiles = 2
	state.Plan.Metrics.GeneratedBytes = int64(len(report) + len(history))
	state.Plan.Metrics.ExactFilesChanged = int64(len(state.Plan.Files))
	state.Plan.Metrics.ExactFilesPlanned = int64(len(state.Plan.Files))
	if err := validateManifestAfter(state); err != nil {
		return err
	}
	return nil
}

func renderAndFinalizeV3(state *runState, baseFiles []FileMutation) error {
	targets := derivedProjectionTargets(state)
	if len(targets) == 0 {
		return fmt.Errorf("v3 manifest declares no derived projections")
	}
	generated := map[string][]byte{}
	var generatedBytes int64
	previousBytes := int64(-1)
	for iteration := 0; iteration < 8; iteration++ {
		context := templateContext(state, baseFiles)
		generated = map[string][]byte{}
		generatedBytes = 0
		for _, target := range targets {
			data, err := renderTemplate(state.Meta, target.Role, context)
			if err != nil {
				return err
			}
			if !json.Valid(data) {
				return fmt.Errorf("projection template emitted invalid JSON: %s", target.Role)
			}
			generated[target.Path] = data
			generatedBytes += int64(len(data))
		}
		state.Plan.Metrics.GeneratedBytes = generatedBytes
		if generatedBytes == previousBytes {
			break
		}
		previousBytes = generatedBytes
	}
	for _, target := range targets {
		data := generated[target.Path]
		context := templateContext(state, baseFiles)
		replayed, err := renderTemplate(state.Meta, target.Role, context)
		if err != nil {
			return err
		}
		if !bytes.Equal(data, replayed) {
			return fmt.Errorf("derived projection regeneration is not deterministic: %s", target.Path)
		}
		state.AfterFiles[target.Path] = data
		before := state.BeforeFiles[target.Path]
		action := "create"
		if before != nil {
			action = "replace"
		}
		state.Plan.Files = append(state.Plan.Files, mutation(target.Path, before, data, action, []string{"/"}, "regenerate derived projection from post-append semantic source"))
	}
	state.Plan.Metrics.GeneratedFiles = int64(len(targets))
	state.Plan.Metrics.GeneratedBytes = generatedBytes
	state.Plan.Metrics.ExactFilesChanged = int64(len(state.Plan.Files))
	state.Plan.Metrics.ExactFilesPlanned = int64(len(state.Plan.Files))
	return validateManifestAfter(state)
}

func derivedProjectionTargets(state *runState) []ManifestFileTarget {
	roles := []string{}
	for role, target := range state.Manifest.FileTargets {
		if target.Kind == "DERIVED_PROJECTION" && state.Meta.PathKinds[role] == "DERIVED_PROJECTION" {
			roles = append(roles, role)
		}
	}
	sort.Strings(roles)
	result := make([]ManifestFileTarget, 0, len(roles))
	for _, role := range roles {
		result = append(result, state.Manifest.FileTargets[role])
	}
	return result
}

func validateManifestAfter(state *runState) error {
	if !hasTransactionManifest(state.Transaction.Schema) || state.TargetBinding == nil {
		return nil
	}
	binding := state.TargetBinding
	wantPaths := map[string]bool{}
	if state.Transaction.Schema == "gooo/ledger-append-transaction/v2" {
		for _, path := range state.Manifest.PlannedFiles {
			wantPaths[path] = true
		}
	} else {
		for _, target := range state.Manifest.FileTargets {
			wantPaths[target.Path] = true
		}
	}
	gotPaths := map[string]bool{}
	for _, file := range state.Plan.Files {
		gotPaths[file.Path] = true
	}
	if len(gotPaths) != len(wantPaths) {
		return fmt.Errorf("manifest planned file set was not applied exactly")
	}
	for path := range wantPaths {
		if !gotPaths[path] {
			return fmt.Errorf("manifest planned file was not applied: %s", path)
		}
	}
	profile, err := afterObject(state, state.Meta.Paths["profile"])
	if err != nil {
		return err
	}
	profileCells, err := array(profile["cells"], "after profile.cells")
	if err != nil || len(profileCells) != binding.ExpectedDenominator+1 || intValue(profile["total_cells"]) != int64(binding.ExpectedDenominator+1) {
		return fmt.Errorf("after profile does not advance the manifest denominator by one")
	}
	if !sameCanonical(profileCells[len(profileCells)-1], mustJSONValue(state.Transaction.Cell)) {
		return fmt.Errorf("after profile does not append the manifest transaction cell")
	}
	assessment, err := afterObject(state, state.Meta.Paths["assessment"])
	if err != nil {
		return err
	}
	assessmentCells, err := array(assessment["cells"], "after assessment.cells")
	if err != nil || len(assessmentCells) != binding.ExpectedDenominator+1 {
		return fmt.Errorf("after assessment does not append exactly one cell")
	}
	expectedAssessment, err := assessmentJSONValue(state.Transaction)
	if err != nil || !sameCanonical(assessmentCells[len(assessmentCells)-1], expectedAssessment) {
		return fmt.Errorf("after assessment does not append the manifest transaction outcome")
	}
	statusAfter := cloneIntMap(binding.ExpectedStateCounts)
	statusAfter[state.Transaction.Outcome.State]++
	if !sameIntMap(statusAfter, state.Transaction.Expected.StatusCounts) {
		return fmt.Errorf("transaction state counts do not equal manifest before counts plus outcome")
	}
	actualStatus, err := statusCounts(state.Meta, assessmentCells)
	if err != nil || !sameIntMap(actualStatus, statusAfter) {
		return fmt.Errorf("after assessment state counts do not equal the manifest binding")
	}
	activity := state.AfterFiles[state.Meta.Paths["activity_file"]]
	activities := parseActivitySequence(activity)
	if len(activities) != binding.ExpectedDenominator+1 || activities[len(activities)-1] != state.Transaction.Activity.Name || !bytes.HasPrefix(activity, state.Ledger.ActivityRaw) {
		return fmt.Errorf("after activity is not an append-only manifest activity")
	}
	locks, err := afterObject(state, state.Meta.Paths["release_locks"])
	if err != nil {
		return err
	}
	beforeLocks, err := object(state.Ledger.Locks.AST, "before release locks")
	if err != nil {
		return err
	}
	beforeReleaseMap, err := object(beforeLocks["releases"], "before release locks.releases")
	if err != nil {
		return err
	}
	afterReleaseMap, err := object(locks["releases"], "after release locks.releases")
	if err != nil || len(afterReleaseMap) != len(beforeReleaseMap)+1 || !sameCanonical(afterReleaseMap[state.Transaction.ReleaseKey], state.Transaction.ReleaseLock) {
		return fmt.Errorf("after release locks do not append exactly one manifest lock")
	}
	registry, err := afterObject(state, state.Meta.Paths["registry"])
	if err != nil {
		return err
	}
	entries, err := array(registry["entries"], "after registry.entries")
	if err != nil {
		return err
	}
	beforeRegistry, err := object(state.Ledger.Registry.AST, "before registry")
	if err != nil {
		return err
	}
	beforeEntries, err := array(beforeRegistry["entries"], "before registry.entries")
	if err != nil || len(entries) != len(beforeEntries)+1 || !sameCanonical(entries[len(entries)-1], state.Transaction.RegistryEntry) || intValue(registry["entry_count"]) != int64(len(entries)) {
		return fmt.Errorf("after registry does not append exactly one manifest entry")
	}
	if state.Transaction.Schema == "gooo/ledger-append-transaction/v3" {
		for path := range state.BeforeFiles {
			if _, ok := state.AfterFiles[path]; !ok {
				return fmt.Errorf("unexpected delete outside the v3 manifest: %s", path)
			}
		}
		for _, target := range derivedProjectionTargets(state) {
			data, ok := state.AfterFiles[target.Path]
			if !ok || !json.Valid(data) {
				return fmt.Errorf("after invariant missing regenerated projection: %s", target.Path)
			}
			mutation, ok := mutationByPath(state.Plan.Files, target.Path)
			if !ok || mutation.BeforeDigest != target.BeforeDigest || mutation.Action != "replace" || mutation.AfterExists == false {
				return fmt.Errorf("projection mutation does not bind the exact before state: %s", target.Path)
			}
		}
	} else {
		for _, role := range []string{"report", "history"} {
			if _, ok := state.AfterFiles[state.Meta.Paths[role]]; !ok {
				return fmt.Errorf("after invariant missing generated file: %s", state.Meta.Paths[role])
			}
		}
	}
	return nil
}

func mutationByPath(files []FileMutation, path string) (FileMutation, bool) {
	for _, file := range files {
		if file.Path == path {
			return file, true
		}
	}
	return FileMutation{}, false
}

func validateManifestReceipt(state *runState) error {
	if !hasTransactionManifest(state.Transaction.Schema) {
		return nil
	}
	if !contains(state.Manifest.AfterInvariants, "replay-mismatches-zero") || len(state.Receipt.Mismatches) != 0 || state.Receipt.RepositoryWrites != 0 || state.Plan.Metrics.RepositoryWrites != 0 || state.Plan.InputRepositoryMutated {
		return fmt.Errorf("manifest replay or repository-write invariant failed")
	}
	return nil
}

func afterObject(state *runState, path string) (map[string]any, error) {
	data, ok := state.AfterFiles[path]
	if !ok {
		return nil, fmt.Errorf("after file is missing: %s", path)
	}
	var value any
	if err := decodeJSON(data, &value); err != nil {
		return nil, fmt.Errorf("decode after %s: %w", path, err)
	}
	return object(value, path)
}

func mustJSONValue(value any) any {
	converted, err := toJSONValue(value)
	if err != nil {
		return nil
	}
	return converted
}

func cloneIntMap(value map[string]int) map[string]int {
	result := map[string]int{}
	for key, count := range value {
		result[key] = count
	}
	return result
}

func statusCounts(meta MetaCode, cells []any) (map[string]int, error) {
	result := map[string]int{}
	for _, stateName := range meta.States {
		result[stateName] = 0
	}
	for _, raw := range cells {
		cell, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("assessment cell is not an object")
		}
		stateName := stringValue(cell["state"])
		if !contains(meta.States, stateName) {
			return nil, fmt.Errorf("assessment has unknown state %q", stateName)
		}
		result[stateName]++
	}
	return result, nil
}

func materialize(state *runState, inputRepository, outputDirectory string) error {
	if outputDirectory == "" {
		return fmt.Errorf("output directory is required")
	}
	if err := ensureEmptyDirectory(outputDirectory); err != nil {
		return err
	}
	repositoryOutput := filepath.Join(outputDirectory, "repository")
	if err := materializeRepository(inputRepository, repositoryOutput); err != nil {
		return err
	}
	for _, file := range state.Plan.Files {
		data := state.AfterFiles[file.Path]
		target := filepath.Join(repositoryOutput, filepath.FromSlash(file.Path))
		if !file.AfterExists {
			if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func materializeFindings(state *runState, outputDirectory string) error {
	if err := ensureEmptyDirectory(outputDirectory); err != nil {
		return err
	}
	return nil
}

func verifyMaterialized(state *runState, outputDirectory string) error {
	root := filepath.Join(outputDirectory, "repository")
	// The repository is copied by materializeRepository, kept separate so the source path is never writable.
	return verifyFiles(root, state)
}

func writeArtifacts(state *runState, outputDirectory string) error {
	if err := os.MkdirAll(outputDirectory, 0o755); err != nil {
		return err
	}
	planBytes, err := jsonBytes(state.Plan)
	if err != nil {
		return err
	}
	receiptBytes, err := jsonBytes(state.Receipt)
	if err != nil {
		return err
	}
	rollbackBytes, err := jsonBytes(state.Rollback)
	if err != nil {
		return err
	}
	for path, data := range map[string][]byte{
		"patch-plan.json": planBytes, "replay-receipt.json": receiptBytes,
		"rollback-bundle.json": rollbackBytes, "human-dossier.md": []byte(state.Dossier),
	} {
		if err := os.WriteFile(filepath.Join(outputDirectory, path), data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func copyTree(source, destination string) error {
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return err
	}
	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(destination, rel), info.Mode().Perm())
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported source file type: %s", path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode().Perm())
	})
}

func materializeRepository(source, destination string) error {
	return copyTree(source, destination)
}

func verifyFiles(root string, state *runState) error {
	files, err := snapshotFiles(root)
	if err != nil {
		return err
	}
	for path, before := range state.BeforeFiles {
		expected := before
		if after, ok := state.AfterFiles[path]; ok {
			expected = after
		}
		observed, ok := files[path]
		if !ok || !bytes.Equal(observed, expected) {
			return fmt.Errorf("materialized file mismatch: %s", path)
		}
	}
	for path, expected := range state.AfterFiles {
		observed, ok := files[path]
		if !ok || !bytes.Equal(observed, expected) {
			return fmt.Errorf("materialized generated file mismatch: %s", path)
		}
	}
	for path, before := range state.BeforeFiles {
		if isAllowedMutation(path, state.Plan.Files) {
			continue
		}
		observed, ok := files[path]
		if !ok || !bytes.Equal(observed, before) {
			return fmt.Errorf("undocumented file mutation: %s", path)
		}
	}
	return nil
}

func validateCanonicalPreservation(state *runState, beforeProfile, beforeLocks, beforeAssessment, beforeRegistry map[string]any, afterProfile, afterLocks, afterAssessment, afterRegistry map[string]any, afterActivity []byte) error {
	profileBeforeCells, _ := array(beforeProfile["cells"], "profile.cells")
	profileAfterCells, _ := array(afterProfile["cells"], "profile.cells")
	if len(profileAfterCells) != len(profileBeforeCells)+1 {
		return fmt.Errorf("profile cell append cardinality changed")
	}
	for index := range profileBeforeCells {
		if !sameCanonical(profileBeforeCells[index], profileAfterCells[index]) {
			return fmt.Errorf("pre-existing profile cell changed at ordinal %d", index+1)
		}
	}
	for key, value := range beforeLocks {
		if key == "releases" {
			continue
		}
		if !sameCanonical(value, afterLocks[key]) {
			return fmt.Errorf("pre-existing release-lock root field changed: %s", key)
		}
	}
	beforeReleaseMap, _ := object(beforeLocks["releases"], "release locks.releases")
	afterReleaseMap, _ := object(afterLocks["releases"], "release locks.releases")
	for key, value := range beforeReleaseMap {
		if !sameCanonical(value, afterReleaseMap[key]) {
			return fmt.Errorf("pre-existing release lock changed: %s", key)
		}
	}
	for key, value := range beforeAssessment {
		if key == "cells" || key == "denominator_migration" {
			continue
		}
		if !sameCanonical(value, afterAssessment[key]) {
			return fmt.Errorf("pre-existing assessment field changed: %s", key)
		}
	}
	beforeAssessmentCells, _ := array(beforeAssessment["cells"], "assessment.cells")
	afterAssessmentCells, _ := array(afterAssessment["cells"], "assessment.cells")
	if len(afterAssessmentCells) != len(beforeAssessmentCells)+1 {
		return fmt.Errorf("assessment cell append cardinality changed")
	}
	for index := range beforeAssessmentCells {
		if !sameCanonical(beforeAssessmentCells[index], afterAssessmentCells[index]) {
			return fmt.Errorf("pre-existing assessment cell changed at index %d", index)
		}
	}
	for key, value := range beforeRegistry {
		if key == "entries" || key == "entry_count" || key == "frontier_additions" {
			continue
		}
		if !sameCanonical(value, afterRegistry[key]) {
			return fmt.Errorf("pre-existing registry field changed: %s", key)
		}
	}
	beforeEntries, _ := array(beforeRegistry["entries"], "registry.entries")
	afterEntries, _ := array(afterRegistry["entries"], "registry.entries")
	if len(afterEntries) != len(beforeEntries)+1 {
		return fmt.Errorf("registry entry append cardinality changed")
	}
	for index := range beforeEntries {
		if !sameCanonical(beforeEntries[index], afterEntries[index]) {
			return fmt.Errorf("pre-existing registry entry changed at index %d", index)
		}
	}
	if !bytes.HasPrefix(afterActivity, state.Ledger.ActivityRaw) {
		return fmt.Errorf("pre-existing Gooo activity bytes changed")
	}
	return nil
}

func isAllowedMutation(path string, files []FileMutation) bool {
	for _, file := range files {
		if file.Path == path {
			return true
		}
	}
	return false
}

func ensureEmptyDirectory(path string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return fmt.Errorf("output directory must be empty: %s", path)
	}
	return nil
}

func isWithin(path, root string) bool {
	if path == "" || root == "" {
		return false
	}
	absPath, err1 := filepath.Abs(path)
	absRoot, err2 := filepath.Abs(root)
	if err1 != nil || err2 != nil {
		return false
	}
	rel, err := filepath.Rel(absRoot, absPath)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

func cloneFiles(files map[string][]byte) map[string][]byte {
	result := make(map[string][]byte, len(files))
	for path, data := range files {
		result[path] = append([]byte(nil), data...)
	}
	return result
}

func subjectDigest(ledger LedgerAST, files map[string][]byte, meta MetaCode) (string, error) {
	selected := map[string][]byte{}
	for _, key := range semanticTargetRoles(meta) {
		path := meta.Paths[key]
		if data, ok := files[path]; ok {
			selected[path] = data
		}
	}
	digestLedger := ledger
	if activity, ok := files[ledger.ActivityFile]; ok {
		digestLedger.ActivityRaw = activity
	}
	return ledgerDigest(digestLedger, selected)
}

func semanticTargetRoles(meta MetaCode) []string {
	roles := []string{}
	for role, kind := range meta.PathKinds {
		if kind == "SEMANTIC_APPEND_ONLY" {
			roles = append(roles, role)
		}
	}
	if len(roles) == 0 {
		roles = []string{"activity_file", "profile", "release_locks", "assessment", "registry"}
	}
	sort.Strings(roles)
	return roles
}

func reduceFindings(meta MetaCode, findings []Finding) string {
	for _, status := range meta.Precedence {
		for _, finding := range findings {
			if finding.State == status {
				return status
			}
		}
	}
	return meta.TerminalState
}

func refutation(stage, step, reason, counterexample string) Finding {
	return Finding{State: DecisionRefuted, Stage: stage, Step: step, Reason: reason, Counterexample: counterexample}
}

func unknown(stage, step, reason, class, next string, blocked []string) Finding {
	return Finding{State: DecisionUnknown, Stage: stage, Step: step, Reason: reason, UnknownClass: class, NextOperation: next, BlockedBy: blocked}
}

func findingReasons(findings []Finding) []string {
	result := make([]string, 0, len(findings))
	for _, finding := range findings {
		result = append(result, finding.Reason+":"+finding.Counterexample)
	}
	return result
}

func stringValue(value any) string {
	result, _ := value.(string)
	return result
}

func intValue(value any) int64 {
	switch current := value.(type) {
	case json.Number:
		result, _ := current.Int64()
		return result
	case int:
		return int64(current)
	case int64:
		return current
	case float64:
		return int64(current)
	default:
		return 0
	}
}

func nestedString(object map[string]any, parent, child string) string {
	value, _ := object[parent].(map[string]any)
	return stringValue(value[child])
}

func nestedInt(object map[string]any, parent, child string) int64 {
	value, _ := object[parent].(map[string]any)
	return intValue(value[child])
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func sameIntMap(left, right map[string]int) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}

func intMapValue(values map[string]int) map[string]any {
	result := map[string]any{}
	for key, value := range values {
		result[key] = json.Number(fmt.Sprint(value))
	}
	return result
}

func migrationValue(from, to int64, contract Migration) map[string]any {
	return map[string]any{"from": json.Number(fmt.Sprint(from)), "to": json.Number(fmt.Sprint(to)), "add": json.Number(fmt.Sprint(contract.Add)), "retire": json.Number(fmt.Sprint(contract.Retire)), "split": json.Number(fmt.Sprint(contract.Split)), "append_only": true}
}

func migrationValueStruct(from, to int64, contract Migration) Migration {
	return Migration{Operation: contract.Operation, From: int(from), To: int(to), Add: contract.Add, Retire: contract.Retire, Split: contract.Split}
}

func toJSONValue(value any) (any, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var result any
	if err := decodeJSON(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func assessmentJSONValue(tx Transaction) (map[string]any, error) {
	result := map[string]any{
		"cell_id": tx.Cell.ID, "state": tx.Outcome.State, "release_key": tx.Cell.ReleaseKey, "evidence": tx.Outcome.Evidence,
	}
	if result["evidence"] == nil {
		result["evidence"] = []string{}
	}
	if tx.Outcome.Unknown != nil {
		result["unknown"] = tx.Outcome.Unknown
	}
	if tx.Outcome.Refutation != nil {
		result["refutation"] = tx.Outcome.Refutation
	}
	return result, nil
}

func appendActivity(before []byte, activity Activity) []byte {
	line := fmt.Sprintf("activity %s(%s) -> %s computes %q", activity.Name, activity.InputType, activity.OutputType, activity.Computes)
	result := append([]byte(nil), before...)
	if len(result) > 0 && result[len(result)-1] != '\n' {
		result = append(result, '\n')
	}
	result = append(result, []byte(line+"\n")...)
	return result
}

func parseActivityNames(data []byte) map[string]bool {
	result := map[string]bool{}
	for _, name := range parseActivitySequence(data) {
		result[name] = true
	}
	return result
}

func parseActivitySequence(data []byte) []string {
	result := []string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "activity ") {
			continue
		}
		rest := strings.TrimPrefix(line, "activity ")
		open := strings.IndexByte(rest, '(')
		if open > 0 {
			result = append(result, strings.TrimSpace(rest[:open]))
		}
	}
	return result
}

func registryEntryByID(entries []any, id string) (map[string]any, bool) {
	for _, raw := range entries {
		entry, ok := raw.(map[string]any)
		if ok && stringValue(entry["entry_id"]) == id {
			return entry, true
		}
	}
	return nil, false
}

func mutation(path string, before, after []byte, action string, changed []string, description string) FileMutation {
	result := FileMutation{Path: path, Action: action, BeforeExists: before != nil, AfterExists: after != nil, BeforeBytes: int64(len(before)), AfterBytes: int64(len(after)), ChangedPaths: changed, Description: description}
	if before != nil {
		result.BeforeDigest = fileDigest(before)
	}
	if after != nil {
		result.AfterDigest = fileDigest(after)
	}
	return result
}

func authorityValue(meta MetaCode) map[string]any {
	result := map[string]any{}
	for key, value := range meta.Authority {
		result[key] = value
	}
	result["input_repository_mutated"] = false
	return result
}

func initialMetrics(meta MetaCode, files []FileMutation, before, after map[string][]byte) Metrics {
	result := emptyMetrics(meta)
	result.ExactFilesChanged = int64(len(files))
	result.ExactFilesPlanned = int64(len(files))
	for _, file := range files {
		result.BytesBefore += file.BeforeBytes
		result.BytesAfter += file.AfterBytes
	}
	result.ASTNodesAdded = 5
	result.RepositoryWrites = 0
	result.RootReadmeExcluded = true
	return result
}

func emptyMetrics(meta MetaCode) Metrics {
	return Metrics{RepositoryWrites: int64(meta.Authority["repository_writes"]), RootReadmeExcluded: true}
}

func projectionBytes(files []FileMutation) int64 {
	var result int64
	for _, file := range files {
		if file.Action == "create" || file.Action == "replace" {
			result += file.AfterBytes
		}
	}
	return result
}

func collectMetrics(current Metrics, root string, meta MetaCode, files []FileMutation, generatedFiles int) Metrics {
	current.GeneratedFiles = int64(generatedFiles)
	if root == "" {
		return current
	}
	entries, err := snapshotFiles(root)
	if err != nil {
		return current
	}
	for path, data := range entries {
		if path == "README.md" {
			continue
		}
		current.RegularFiles++
		if strings.HasSuffix(path, ".go") {
			current.GoFiles++
			current.GoLines += int64(strings.Count(string(data), "\n"))
			if len(data) > 0 && data[len(data)-1] != '\n' {
				current.GoLines++
			}
		}
		if strings.HasSuffix(path, ".gooo") {
			current.GoooFiles++
			current.GoooLines += int64(strings.Count(string(data), "\n"))
			if len(data) > 0 && data[len(data)-1] != '\n' {
				current.GoooLines++
			}
		}
	}
	directories := map[string]bool{}
	for path := range entries {
		for parent := filepath.ToSlash(filepath.Dir(path)); parent != "." && parent != ""; parent = filepath.ToSlash(filepath.Dir(parent)) {
			directories[parent] = true
		}
	}
	current.Directories = int64(len(directories))
	return current
}

func makeReceipt(state *runState, _ string) ReplayReceipt {
	observed := state.AfterDigest
	return ReplayReceipt{Schema: "gooo/ledger-append-planner/replay-receipt/v1", TransactionID: state.Transaction.TransactionID, State: state.Plan.OperationDecision, BeforeDigest: state.BeforeDigest, AfterDigest: state.AfterDigest, ObservedAfterDigest: observed, Mismatches: []string{}, RollbackReady: true, RepositoryWrites: 0}
}

func makeRollback(state *runState) RollbackBundle {
	paths := make([]string, 0, len(state.Plan.Files))
	for _, file := range state.Plan.Files {
		paths = append(paths, file.Path)
	}
	sort.Strings(paths)
	files := make([]RollbackFile, 0, len(paths))
	for _, path := range paths {
		before, exists := state.BeforeFiles[path]
		after := state.AfterFiles[path]
		entry := RollbackFile{Path: path, BeforeExists: exists, AfterDigest: fileDigest(after)}
		if exists {
			entry.BeforeDigest = fileDigest(before)
			entry.BeforeBase64 = encodeBase64(before)
		}
		files = append(files, entry)
	}
	return RollbackBundle{Schema: "gooo/ledger-append-planner/rollback-bundle/v1", TransactionID: state.Transaction.TransactionID, BeforeDigest: state.BeforeDigest, AfterDigest: state.AfterDigest, Files: files, Replay: state.Receipt}
}
