package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

type verifierTransactionManifest struct {
	schema            string
	operation         string
	transactionSchema string
	plannedFiles      map[string]string
	afterInvariants   map[string]bool
	targets           map[string]verifierTargetBinding
	fileTargets       map[string]verifierFileTarget
}

type verifierFileTarget struct {
	role                 string
	path                 string
	kind                 string
	projectionKind       string
	beforeDigest         string
	sourceSemanticDigest string
	afterInvariant       string
}

type verifierTargetBinding struct {
	key                 string
	repository          string
	tag                 string
	targetCommitSHA     string
	beforeDigest        string
	expectedDenominator int64
	expectedStateCounts map[string]int64
	anchors             map[string]string
}

func parseTransactionManifest(path string) (verifierTransactionManifest, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return verifierTransactionManifest{}, err
	}
	manifest := verifierTransactionManifest{plannedFiles: map[string]string{}, afterInvariants: map[string]bool{}, targets: map[string]verifierTargetBinding{}, fileTargets: map[string]verifierFileTarget{}}
	var current *verifierTargetBinding
	for lineNumber, rawLine := range strings.Split(string(raw), "\n") {
		lineNumber++
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "gooo":
			if len(fields) != 3 || fields[1] != "transaction_manifest" || (fields[2] != "v2" && fields[2] != "v3") {
				return verifierTransactionManifest{}, fmt.Errorf("%s:%d: malformed transaction manifest", path, lineNumber)
			}
			manifest.schema = "gooo/transaction-manifest/" + fields[2]
		case "operation":
			manifest.operation = strings.TrimSpace(strings.TrimPrefix(line, "operation "))
		case "transaction-schema":
			manifest.transactionSchema, err = verifierQuoted(strings.TrimSpace(strings.TrimPrefix(line, "transaction-schema ")))
			if err != nil {
				return verifierTransactionManifest{}, fmt.Errorf("%s:%d: %w", path, lineNumber, err)
			}
		case "planned-file":
			if current != nil {
				return verifierTransactionManifest{}, fmt.Errorf("%s:%d: planned file inside target", path, lineNumber)
			}
			key, value, ok := verifierQuotedPair(strings.TrimSpace(strings.TrimPrefix(line, "planned-file ")))
			if !ok || manifest.plannedFiles[key] != "" {
				return verifierTransactionManifest{}, fmt.Errorf("%s:%d: malformed planned file", path, lineNumber)
			}
			manifest.plannedFiles[key] = value
		case "target-file":
			if current != nil {
				return verifierTransactionManifest{}, fmt.Errorf("%s:%d: target file inside target", path, lineNumber)
			}
			kind, role, value, ok := verifierTargetFile(strings.TrimSpace(strings.TrimPrefix(line, "target-file ")))
			if !ok || (kind != "SEMANTIC_APPEND_ONLY" && kind != "DERIVED_PROJECTION") || manifest.fileTargets[role].role != "" {
				return verifierTransactionManifest{}, fmt.Errorf("%s:%d: malformed target file", path, lineNumber)
			}
			manifest.fileTargets[role] = verifierFileTarget{role: role, path: value, kind: kind}
		case "projection-kind", "projection-before-digest", "projection-source-semantic-digest", "projection-after-invariant":
			if current != nil {
				return verifierTransactionManifest{}, fmt.Errorf("%s:%d: projection declaration inside target", path, lineNumber)
			}
			role, value, ok := verifierQuotedPair(strings.TrimSpace(strings.TrimPrefix(line, fields[0]+" ")))
			if !ok {
				return verifierTransactionManifest{}, fmt.Errorf("%s:%d: malformed %s", path, lineNumber, fields[0])
			}
			target := manifest.fileTargets[role]
			target.role = role
			switch fields[0] {
			case "projection-kind":
				target.projectionKind = value
			case "projection-before-digest":
				target.beforeDigest = value
			case "projection-source-semantic-digest":
				target.sourceSemanticDigest = value
			case "projection-after-invariant":
				target.afterInvariant = value
			}
			manifest.fileTargets[role] = target
		case "after-invariant":
			if current != nil {
				return verifierTransactionManifest{}, fmt.Errorf("%s:%d: after invariant inside target", path, lineNumber)
			}
			invariant := strings.TrimSpace(strings.TrimPrefix(line, "after-invariant "))
			if invariant == "" {
				return verifierTransactionManifest{}, fmt.Errorf("%s:%d: empty after invariant", path, lineNumber)
			}
			manifest.afterInvariants[invariant] = true
		case "target":
			if current != nil || len(fields) != 2 || fields[1] == "" {
				return verifierTransactionManifest{}, fmt.Errorf("%s:%d: malformed target", path, lineNumber)
			}
			current = &verifierTargetBinding{key: fields[1], expectedStateCounts: map[string]int64{}, anchors: map[string]string{}}
		case "end-target":
			if current == nil {
				return verifierTransactionManifest{}, fmt.Errorf("%s:%d: target close without target", path, lineNumber)
			}
			manifest.targets[current.key] = *current
			current = nil
		case "target-repository", "target-tag", "target-commit-sha", "target-before-digest":
			if current == nil {
				return verifierTransactionManifest{}, fmt.Errorf("%s:%d: target field outside target", path, lineNumber)
			}
			value, valueErr := verifierQuoted(strings.TrimSpace(strings.TrimPrefix(line, fields[0]+" ")))
			if valueErr != nil {
				return verifierTransactionManifest{}, fmt.Errorf("%s:%d: %w", path, lineNumber, valueErr)
			}
			switch fields[0] {
			case "target-repository":
				current.repository = value
			case "target-tag":
				current.tag = value
			case "target-commit-sha":
				current.targetCommitSHA = value
			case "target-before-digest":
				current.beforeDigest = value
			}
		case "expected-denominator":
			if current == nil || len(fields) != 2 {
				return verifierTransactionManifest{}, fmt.Errorf("%s:%d: malformed denominator", path, lineNumber)
			}
			current.expectedDenominator, err = strconv.ParseInt(fields[1], 10, 64)
			if err != nil || current.expectedDenominator <= 0 {
				return verifierTransactionManifest{}, fmt.Errorf("%s:%d: invalid denominator", path, lineNumber)
			}
		case "expected-state-count":
			if current == nil {
				return verifierTransactionManifest{}, fmt.Errorf("%s:%d: state count outside target", path, lineNumber)
			}
			for _, field := range strings.Fields(strings.TrimSpace(strings.TrimPrefix(line, "expected-state-count "))) {
				parts := strings.SplitN(field, "=", 2)
				if len(parts) != 2 {
					return verifierTransactionManifest{}, fmt.Errorf("%s:%d: malformed state count", path, lineNumber)
				}
				count, parseErr := strconv.ParseInt(parts[1], 10, 64)
				if parseErr != nil || count < 0 {
					return verifierTransactionManifest{}, fmt.Errorf("%s:%d: malformed state count", path, lineNumber)
				}
				current.expectedStateCounts[parts[0]] = count
			}
		case "anchor":
			if current == nil {
				return verifierTransactionManifest{}, fmt.Errorf("%s:%d: anchor outside target", path, lineNumber)
			}
			key, value, ok := verifierQuotedPair(strings.TrimSpace(strings.TrimPrefix(line, "anchor ")))
			if !ok || current.anchors[key] != "" {
				return verifierTransactionManifest{}, fmt.Errorf("%s:%d: malformed anchor", path, lineNumber)
			}
			current.anchors[key] = value
		default:
			return verifierTransactionManifest{}, fmt.Errorf("%s:%d: unknown directive %q", path, lineNumber, fields[0])
		}
	}
	if current != nil {
		return verifierTransactionManifest{}, fmt.Errorf("%s: target is not closed", path)
	}
	if manifest.operation == "" || len(manifest.targets) == 0 {
		return verifierTransactionManifest{}, fmt.Errorf("transaction manifest contract is incomplete")
	}
	if manifest.schema == "gooo/transaction-manifest/v2" && (manifest.transactionSchema != "gooo/ledger-append-transaction/v2" || len(manifest.plannedFiles) != 7) {
		return verifierTransactionManifest{}, fmt.Errorf("transaction manifest contract is incomplete")
	}
	if manifest.schema == "gooo/transaction-manifest/v3" && (manifest.transactionSchema != "gooo/ledger-append-transaction/v3" || len(manifest.fileTargets) == 0) {
		return verifierTransactionManifest{}, fmt.Errorf("transaction manifest contract is incomplete")
	}
	for _, invariant := range requiredVerifierManifestInvariants(manifest.schema) {
		if !manifest.afterInvariants[invariant] {
			return verifierTransactionManifest{}, fmt.Errorf("transaction manifest invariant is missing: %s", invariant)
		}
	}
	return manifest, nil
}

func verifierQuoted(value string) (string, error) {
	parsed, err := strconv.Unquote(strings.TrimSpace(value))
	if err != nil || parsed == "" {
		return "", fmt.Errorf("malformed quoted value")
	}
	return parsed, nil
}

func verifierQuotedPair(value string) (string, string, bool) {
	parts := strings.SplitN(strings.TrimSpace(value), " ", 2)
	if len(parts) != 2 || parts[0] == "" {
		return "", "", false
	}
	parsed, err := verifierQuoted(parts[1])
	return parts[0], parsed, err == nil
}

func verifierTargetFile(value string) (string, string, string, bool) {
	parts := strings.SplitN(strings.TrimSpace(value), " ", 3)
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" {
		return "", "", "", false
	}
	path, err := strconv.Unquote(strings.TrimSpace(parts[2]))
	return parts[0], parts[1], path, err == nil && path != ""
}

func requiredVerifierManifestInvariants(schema string) []string {
	result := []string{"append-one-cell", "append-one-activity", "append-one-release-lock", "append-one-assessment", "append-one-registry-entry", "advance-denominator-by-one", "increment-state-counts-by-outcome", "preserve-existing-canonical-objects", "planned-file-set-exact", "replay-mismatches-zero", "no-input-repository-writes"}
	if schema == "gooo/transaction-manifest/v3" {
		result = append(result, "semantic-targets-declared", "derived-projection-targets-declared", "projection-kind-declared", "projection-before-digest-exact", "projection-source-semantic-digest-exact", "projection-regenerate-deterministic", "unexpected-deletes-refuted")
	}
	return result
}

func verifyManifestBefore(manifest verifierTransactionManifest, tx map[string]any, meta authority, repository string, input map[string][]byte) error {
	if stringField(tx, "schema") != manifest.transactionSchema || manifest.operation != "append_exactly_one_adoption_transaction" {
		return fmt.Errorf("transaction does not match manifest identity")
	}
	if manifest.schema == "gooo/transaction-manifest/v3" {
		return verifyManifestBeforeV3(manifest, tx, meta, repository, input)
	}
	for role, path := range meta.paths {
		if manifest.plannedFiles[role] != path {
			return fmt.Errorf("manifest planned file differs from authority path: %s", role)
		}
	}
	key := stringField(tx, "manifest_key")
	binding, ok := manifest.targets[key]
	if !ok {
		return fmt.Errorf("manifest target binding is missing: %s", key)
	}
	baseline := nestedObject(tx, "baseline")
	if stringField(baseline, "repository") != binding.repository || stringField(baseline, "tag") != binding.tag || stringField(baseline, "target_commit_sha") != binding.targetCommitSHA || stringField(baseline, "source_tree_sha256") != binding.beforeDigest {
		return fmt.Errorf("transaction baseline differs from manifest target binding")
	}
	observed, err := sourceTreeDigest(repository)
	if err != nil {
		return err
	}
	if observed != binding.beforeDigest {
		return fmt.Errorf("manifest target before digest mismatch: observed=%s expected=%s", observed, binding.beforeDigest)
	}
	profile, err := readObjectBytes(input[meta.paths["profile"]])
	if err != nil {
		return err
	}
	cells, ok := profile["cells"].([]any)
	if !ok || int64(len(cells)) != binding.expectedDenominator || numberValue(profile["total_cells"]) != binding.expectedDenominator {
		return fmt.Errorf("manifest target denominator mismatch")
	}
	assessment, err := readObjectBytes(input[meta.paths["assessment"]])
	if err != nil {
		return err
	}
	assessmentCells, ok := assessment["cells"].([]any)
	if !ok {
		return fmt.Errorf("assessment cells are unavailable")
	}
	counts := map[string]int64{"CLOSED": 0, "UNKNOWN": 0, "REFUTED": 0}
	for _, raw := range assessmentCells {
		cell, ok := raw.(map[string]any)
		if !ok {
			return fmt.Errorf("assessment cell is not an object")
		}
		counts[stringField(cell, "state")]++
	}
	if !sameInt64Map(counts, binding.expectedStateCounts) {
		return fmt.Errorf("manifest target state counts mismatch")
	}
	if len(cells) == 0 {
		return fmt.Errorf("profile cells are empty")
	}
	first := nestedObjectAt(cells, 0)
	last := nestedObjectAt(cells, len(cells)-1)
	if err := verifyAnchor(binding, "profile.first_cell_id", stringField(first, "id")); err != nil {
		return err
	}
	if err := verifyAnchor(binding, "profile.last_cell_id", stringField(last, "id")); err != nil {
		return err
	}
	if err := verifyAnchor(binding, "release_map.last_key", stringField(last, "release_key")); err != nil {
		return err
	}
	if err := verifyAnchor(binding, "assessment.last_cell_id", stringField(nestedObjectAt(assessmentCells, len(assessmentCells)-1), "cell_id")); err != nil {
		return err
	}
	registry, err := readObjectBytes(input[meta.paths["registry"]])
	if err != nil {
		return err
	}
	entries, ok := registry["entries"].([]any)
	if !ok || len(entries) == 0 {
		return fmt.Errorf("registry entries are unavailable")
	}
	if err := verifyAnchor(binding, "registry.last_entry_id", stringField(nestedObjectAt(entries, len(entries)-1), "entry_id")); err != nil {
		return err
	}
	activities := activitySequence(input[meta.paths["activity_file"]])
	if len(activities) == 0 {
		return fmt.Errorf("activity sequence is unavailable")
	}
	return verifyAnchor(binding, "activity.last_activity", activities[len(activities)-1])
}

func verifyManifestBeforeV3(manifest verifierTransactionManifest, tx map[string]any, meta authority, repository string, input map[string][]byte) error {
	roles := map[string]bool{}
	for role, path := range meta.paths {
		roles[role] = true
		target, ok := manifest.fileTargets[role]
		if !ok {
			return fmt.Errorf("derived or semantic manifest target is missing: %s", role)
		}
		if target.path != path || target.kind != meta.pathKinds[role] {
			return fmt.Errorf("manifest target differs from authority: %s", role)
		}
		if target.kind == "DERIVED_PROJECTION" && (target.projectionKind == "" || target.beforeDigest == "" || target.sourceSemanticDigest == "" || target.afterInvariant == "") {
			return fmt.Errorf("derived projection authority is incomplete: %s", role)
		}
	}
	for role := range manifest.fileTargets {
		if !roles[role] {
			return fmt.Errorf("manifest contains undeclared target: %s", role)
		}
	}
	key := stringField(tx, "manifest_key")
	binding, ok := manifest.targets[key]
	if !ok {
		return fmt.Errorf("manifest target binding is missing: %s", key)
	}
	baseline := nestedObject(tx, "baseline")
	if stringField(baseline, "repository") != binding.repository || stringField(baseline, "tag") != binding.tag || stringField(baseline, "target_commit_sha") != binding.targetCommitSHA || stringField(baseline, "source_tree_sha256") != binding.beforeDigest {
		return fmt.Errorf("transaction baseline differs from manifest target binding")
	}
	observed, err := sourceTreeDigest(repository)
	if err != nil {
		return err
	}
	if observed != binding.beforeDigest {
		return fmt.Errorf("manifest target before digest mismatch: observed=%s expected=%s", observed, binding.beforeDigest)
	}
	semantic, err := semanticDigest(meta, input)
	if err != nil {
		return err
	}
	for role, target := range manifest.fileTargets {
		if target.kind != "DERIVED_PROJECTION" {
			continue
		}
		data, exists := input[target.path]
		if !exists || digestBytes(data) != target.beforeDigest {
			return fmt.Errorf("projection before digest mismatch: %s", role)
		}
		if semantic != target.sourceSemanticDigest {
			return fmt.Errorf("projection source semantic digest mismatch: %s", role)
		}
	}
	profile, err := readObjectBytes(input[meta.paths["profile"]])
	if err != nil {
		return err
	}
	cells, ok := profile["cells"].([]any)
	if !ok || int64(len(cells)) != binding.expectedDenominator || numberValue(profile["total_cells"]) != binding.expectedDenominator {
		return fmt.Errorf("manifest target denominator mismatch")
	}
	assessment, err := readObjectBytes(input[meta.paths["assessment"]])
	if err != nil {
		return err
	}
	assessmentCells, ok := assessment["cells"].([]any)
	if !ok || len(assessmentCells) == 0 {
		return fmt.Errorf("assessment cells are unavailable")
	}
	counts := map[string]int64{}
	for _, stateName := range []string{"CLOSED", "UNKNOWN", "REFUTED"} {
		counts[stateName] = 0
	}
	for _, raw := range assessmentCells {
		cell, ok := raw.(map[string]any)
		if !ok {
			return fmt.Errorf("assessment cell is not an object")
		}
		counts[stringField(cell, "state")]++
	}
	if !sameInt64Map(counts, binding.expectedStateCounts) {
		return fmt.Errorf("manifest target state counts mismatch")
	}
	if len(cells) == 0 {
		return fmt.Errorf("profile cells are empty")
	}
	first := nestedObjectAt(cells, 0)
	last := nestedObjectAt(cells, len(cells)-1)
	for key, observed := range map[string]string{
		"profile.first_cell_id":   stringField(first, "id"),
		"profile.last_cell_id":    stringField(last, "id"),
		"release_map.last_key":    stringField(last, "release_key"),
		"assessment.last_cell_id": stringField(nestedObjectAt(assessmentCells, len(assessmentCells)-1), "cell_id"),
	} {
		if err := verifyAnchor(binding, key, observed); err != nil {
			return err
		}
	}
	registry, err := readObjectBytes(input[meta.paths["registry"]])
	if err != nil {
		return err
	}
	entries, ok := registry["entries"].([]any)
	if !ok || len(entries) == 0 {
		return fmt.Errorf("registry entries are unavailable")
	}
	if err := verifyAnchor(binding, "registry.last_entry_id", stringField(nestedObjectAt(entries, len(entries)-1), "entry_id")); err != nil {
		return err
	}
	activities := activitySequence(input[meta.paths["activity_file"]])
	if len(activities) == 0 {
		return fmt.Errorf("activity sequence is unavailable")
	}
	return verifyAnchor(binding, "activity.last_activity", activities[len(activities)-1])
}

func verifyManifestAfter(manifest verifierTransactionManifest, tx map[string]any, meta authority, input, output map[string][]byte, plan map[string]any) error {
	key := stringField(tx, "manifest_key")
	binding, ok := manifest.targets[key]
	if !ok {
		return fmt.Errorf("manifest target binding is missing after apply: %s", key)
	}
	want := map[string]bool{}
	if manifest.schema == "gooo/transaction-manifest/v3" {
		for _, target := range manifest.fileTargets {
			want[target.path] = true
		}
	} else {
		for _, path := range manifest.plannedFiles {
			want[path] = true
		}
	}
	rawFiles, ok := plan["files"].([]any)
	if !ok {
		return fmt.Errorf("plan files are unavailable")
	}
	got := map[string]bool{}
	for _, raw := range rawFiles {
		file, ok := raw.(map[string]any)
		if !ok {
			return fmt.Errorf("plan file is not an object")
		}
		got[stringField(file, "path")] = true
	}
	if len(got) != len(want) {
		return fmt.Errorf("manifest planned file set was not applied exactly")
	}
	for path := range want {
		if !got[path] {
			return fmt.Errorf("manifest planned file is missing: %s", path)
		}
	}
	profile, err := readObjectBytes(output[meta.paths["profile"]])
	if err != nil {
		return err
	}
	cells, ok := profile["cells"].([]any)
	if !ok || len(cells) != int(binding.expectedDenominator+1) || numberValue(profile["total_cells"]) != binding.expectedDenominator+1 {
		return fmt.Errorf("after profile violates manifest denominator invariant")
	}
	assessment, err := readObjectBytes(output[meta.paths["assessment"]])
	if err != nil {
		return err
	}
	assessmentCells, ok := assessment["cells"].([]any)
	if !ok || len(assessmentCells) != int(binding.expectedDenominator+1) {
		return fmt.Errorf("after assessment violates append invariant")
	}
	counts := map[string]int64{"CLOSED": 0, "UNKNOWN": 0, "REFUTED": 0}
	for _, raw := range assessmentCells {
		cell, ok := raw.(map[string]any)
		if !ok {
			return fmt.Errorf("after assessment cell is not an object")
		}
		counts[stringField(cell, "state")]++
	}
	outcome := nestedObject(tx, "outcome")
	wantCounts := cloneVerifierInt64Map(binding.expectedStateCounts)
	wantCounts[stringField(outcome, "state")]++
	if !sameInt64Map(counts, wantCounts) {
		return fmt.Errorf("after state counts violate manifest invariant")
	}
	if manifest.schema == "gooo/transaction-manifest/v3" {
		return verifyProjectionAfterV3(manifest, meta, input, output, plan)
	}
	return nil
}

func verifyProjectionAfterV3(manifest verifierTransactionManifest, meta authority, input, output map[string][]byte, plan map[string]any) error {
	beforeSemantic, err := semanticDigest(meta, input)
	if err != nil {
		return err
	}
	afterSemantic, err := semanticDigest(meta, output)
	if err != nil {
		return err
	}
	if beforeSemantic == afterSemantic {
		return fmt.Errorf("semantic append did not change the source digest")
	}
	if stringField(plan, "canonical_after_digest") != afterSemantic {
		return fmt.Errorf("plan after digest is not the post-append semantic digest")
	}
	planFiles, ok := plan["files"].([]any)
	if !ok {
		return fmt.Errorf("plan files are unavailable")
	}
	byPath := map[string]map[string]any{}
	for _, raw := range planFiles {
		file, ok := raw.(map[string]any)
		if !ok {
			return fmt.Errorf("plan file is not an object")
		}
		byPath[stringField(file, "path")] = file
	}
	for role, target := range manifest.fileTargets {
		if target.kind != "DERIVED_PROJECTION" {
			continue
		}
		data, exists := output[target.path]
		if !exists || !json.Valid(data) {
			return fmt.Errorf("projection is missing or invalid: %s", role)
		}
		if target.projectionKind == "" || target.afterInvariant != "deterministic-regenerate-from-post-append-semantic-source" {
			return fmt.Errorf("projection authority is incomplete: %s", role)
		}
		file, ok := byPath[target.path]
		if !ok || stringField(file, "action") != "replace" || stringField(file, "before_digest") != target.beforeDigest || stringField(file, "after_digest") != digestBytes(data) || !boolField(file, "before_exists") || !boolField(file, "after_exists") {
			return fmt.Errorf("projection mutation is not bound to the exact before and after state: %s", role)
		}
	}
	return nil
}

func semanticDigest(meta authority, files map[string][]byte) (string, error) {
	entries := map[string]any{}
	roles := []string{}
	for role, kind := range meta.pathKinds {
		if kind == "SEMANTIC_APPEND_ONLY" {
			roles = append(roles, role)
		}
	}
	if len(roles) == 0 {
		roles = []string{"activity_file", "profile", "release_locks", "assessment", "registry"}
	}
	sort.Strings(roles)
	for _, role := range roles {
		path := meta.paths[role]
		data, ok := files[path]
		if !ok {
			return "", fmt.Errorf("semantic source is missing: %s", path)
		}
		if role == "activity_file" {
			entries[path] = string(data)
			continue
		}
		value, err := readObjectBytes(data)
		if err != nil {
			return "", err
		}
		entries[path] = value
	}
	canonical, err := json.Marshal(map[string]any{"files": entries})
	if err != nil {
		return "", err
	}
	return digestBytes(canonical), nil
}

func digestBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func verifyAnchor(binding verifierTargetBinding, key, observed string) error {
	want, ok := binding.anchors[key]
	if !ok || want == "" || observed != want {
		return fmt.Errorf("manifest structural anchor mismatch: %s", key)
	}
	return nil
}

func nestedObjectAt(values []any, index int) map[string]any {
	if index < 0 || index >= len(values) {
		return nil
	}
	result, _ := values[index].(map[string]any)
	return result
}

func sameInt64Map(left, right map[string]int64) bool {
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

func cloneVerifierInt64Map(value map[string]int64) map[string]int64 {
	result := map[string]int64{}
	for key, count := range value {
		result[key] = count
	}
	return result
}

func activitySequence(data []byte) []string {
	result := []string{}
	for _, rawLine := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(rawLine)
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
