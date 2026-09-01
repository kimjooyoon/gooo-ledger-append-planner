package planner

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	transactionManifestSchemaV2 = "gooo/transaction-manifest/v2"
	transactionManifestSchemaV3 = "gooo/transaction-manifest/v3"
	projectionAfterInvariant    = "deterministic-regenerate-from-post-append-semantic-source"
)

var requiredManifestAfterInvariants = []string{
	"append-one-cell",
	"append-one-activity",
	"append-one-release-lock",
	"append-one-assessment",
	"append-one-registry-entry",
	"advance-denominator-by-one",
	"increment-state-counts-by-outcome",
	"preserve-existing-canonical-objects",
	"planned-file-set-exact",
	"replay-mismatches-zero",
	"no-input-repository-writes",
}

var requiredManifestAfterInvariantsV3 = append(append([]string(nil), requiredManifestAfterInvariants...),
	"semantic-targets-declared",
	"derived-projection-targets-declared",
	"projection-kind-declared",
	"projection-before-digest-exact",
	"projection-source-semantic-digest-exact",
	"projection-regenerate-deterministic",
	"unexpected-deletes-refuted",
)

type TransactionManifest struct {
	Path              string
	Schema            string
	Operation         string
	TransactionSchema string
	PlannedFiles      map[string]string
	AfterInvariants   []string
	Targets           map[string]TargetBinding
	FileTargets       map[string]ManifestFileTarget
}

type TargetBinding struct {
	Key                 string
	Repository          string
	Tag                 string
	TargetCommitSHA     string
	BeforeDigest        string
	ExpectedDenominator int
	ExpectedStateCounts map[string]int
	Anchors             map[string]string
}

func hasTransactionManifest(schema string) bool {
	return schema == "gooo/ledger-append-transaction/v2" || schema == "gooo/ledger-append-transaction/v3"
}

func LoadTransactionManifest(path string) (TransactionManifest, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return TransactionManifest{}, err
	}
	manifest := TransactionManifest{
		Path: path, PlannedFiles: map[string]string{}, Targets: map[string]TargetBinding{}, FileTargets: map[string]ManifestFileTarget{},
	}
	var current *TargetBinding
	scanner := bufio.NewScanner(strings.NewReader(string(raw)))
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := strings.TrimSpace(scanner.Text())
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
				return TransactionManifest{}, fmt.Errorf("%s:%d: malformed transaction manifest declaration", path, lineNumber)
			}
			if fields[2] == "v2" {
				manifest.Schema = transactionManifestSchemaV2
			} else {
				manifest.Schema = transactionManifestSchemaV3
			}
		case "operation":
			manifest.Operation = strings.TrimSpace(strings.TrimPrefix(line, "operation "))
		case "transaction-schema":
			value, ok := quotedValue(strings.TrimSpace(strings.TrimPrefix(line, "transaction-schema ")))
			if !ok {
				return TransactionManifest{}, fmt.Errorf("%s:%d: malformed transaction schema", path, lineNumber)
			}
			manifest.TransactionSchema = value
		case "planned-file":
			key, value, ok := parseQuotedPair(strings.TrimSpace(strings.TrimPrefix(line, "planned-file ")))
			if !ok || current != nil {
				return TransactionManifest{}, fmt.Errorf("%s:%d: malformed planned file", path, lineNumber)
			}
			if _, exists := manifest.PlannedFiles[key]; exists {
				return TransactionManifest{}, fmt.Errorf("%s:%d: duplicate planned file %q", path, lineNumber, key)
			}
			manifest.PlannedFiles[key] = value
		case "target-file":
			if current != nil {
				return TransactionManifest{}, fmt.Errorf("%s:%d: target file inside target", path, lineNumber)
			}
			kind, role, value, ok := parseTargetFile(strings.TrimSpace(strings.TrimPrefix(line, "target-file ")))
			if !ok || (kind != "SEMANTIC_APPEND_ONLY" && kind != "DERIVED_PROJECTION") || manifest.FileTargets[role].Role != "" {
				return TransactionManifest{}, fmt.Errorf("%s:%d: malformed target file", path, lineNumber)
			}
			manifest.FileTargets[role] = ManifestFileTarget{Role: role, Path: value, Kind: kind}
		case "projection-kind", "projection-before-digest", "projection-source-semantic-digest", "projection-after-invariant":
			if current != nil {
				return TransactionManifest{}, fmt.Errorf("%s:%d: projection declaration inside target", path, lineNumber)
			}
			role, value, ok := parseQuotedPair(strings.TrimSpace(strings.TrimPrefix(line, fields[0]+" ")))
			if !ok {
				return TransactionManifest{}, fmt.Errorf("%s:%d: malformed %s", path, lineNumber, fields[0])
			}
			target := manifest.FileTargets[role]
			target.Role = role
			switch fields[0] {
			case "projection-kind":
				target.ProjectionKind = value
			case "projection-before-digest":
				target.BeforeDigest = value
			case "projection-source-semantic-digest":
				target.SourceSemanticDigest = value
			case "projection-after-invariant":
				target.AfterInvariant = value
			}
			manifest.FileTargets[role] = target
		case "after-invariant":
			if current != nil || strings.TrimSpace(strings.TrimPrefix(line, "after-invariant ")) == "" {
				return TransactionManifest{}, fmt.Errorf("%s:%d: malformed after invariant", path, lineNumber)
			}
			manifest.AfterInvariants = append(manifest.AfterInvariants, strings.TrimSpace(strings.TrimPrefix(line, "after-invariant ")))
		case "target":
			if current != nil || len(fields) != 2 || fields[1] == "" {
				return TransactionManifest{}, fmt.Errorf("%s:%d: malformed target", path, lineNumber)
			}
			if _, exists := manifest.Targets[fields[1]]; exists {
				return TransactionManifest{}, fmt.Errorf("%s:%d: duplicate target %q", path, lineNumber, fields[1])
			}
			current = &TargetBinding{Key: fields[1], ExpectedStateCounts: map[string]int{}, Anchors: map[string]string{}}
		case "end-target":
			if current == nil {
				return TransactionManifest{}, fmt.Errorf("%s:%d: end-target without target", path, lineNumber)
			}
			manifest.Targets[current.Key] = *current
			current = nil
		case "target-repository":
			if current == nil {
				return TransactionManifest{}, fmt.Errorf("%s:%d: target-repository outside target", path, lineNumber)
			}
			current.Repository, err = manifestQuotedValue(path, lineNumber, line, "target-repository")
			if err != nil {
				return TransactionManifest{}, err
			}
		case "target-tag":
			if current == nil {
				return TransactionManifest{}, fmt.Errorf("%s:%d: target-tag outside target", path, lineNumber)
			}
			current.Tag, err = manifestQuotedValue(path, lineNumber, line, "target-tag")
			if err != nil {
				return TransactionManifest{}, err
			}
		case "target-commit-sha":
			if current == nil {
				return TransactionManifest{}, fmt.Errorf("%s:%d: target-commit-sha outside target", path, lineNumber)
			}
			current.TargetCommitSHA, err = manifestQuotedValue(path, lineNumber, line, "target-commit-sha")
			if err != nil {
				return TransactionManifest{}, err
			}
		case "target-before-digest":
			if current == nil {
				return TransactionManifest{}, fmt.Errorf("%s:%d: target-before-digest outside target", path, lineNumber)
			}
			current.BeforeDigest, err = manifestQuotedValue(path, lineNumber, line, "target-before-digest")
			if err != nil {
				return TransactionManifest{}, err
			}
		case "expected-denominator":
			if current == nil || len(fields) != 2 {
				return TransactionManifest{}, fmt.Errorf("%s:%d: malformed expected denominator", path, lineNumber)
			}
			current.ExpectedDenominator, err = strconv.Atoi(fields[1])
			if err != nil || current.ExpectedDenominator <= 0 {
				return TransactionManifest{}, fmt.Errorf("%s:%d: expected denominator is not positive", path, lineNumber)
			}
		case "expected-state-count":
			if current == nil {
				return TransactionManifest{}, fmt.Errorf("%s:%d: expected state count outside target", path, lineNumber)
			}
			counts, ok := parseManifestCounts(strings.TrimSpace(strings.TrimPrefix(line, "expected-state-count ")))
			if !ok {
				return TransactionManifest{}, fmt.Errorf("%s:%d: malformed expected state count", path, lineNumber)
			}
			for key, value := range counts {
				current.ExpectedStateCounts[key] = value
			}
		case "anchor":
			if current == nil {
				return TransactionManifest{}, fmt.Errorf("%s:%d: anchor outside target", path, lineNumber)
			}
			key, value, ok := parseQuotedPair(strings.TrimSpace(strings.TrimPrefix(line, "anchor ")))
			if !ok || current.Anchors[key] != "" {
				return TransactionManifest{}, fmt.Errorf("%s:%d: malformed or duplicate anchor", path, lineNumber)
			}
			current.Anchors[key] = value
		default:
			return TransactionManifest{}, fmt.Errorf("%s:%d: unknown transaction manifest directive %q", path, lineNumber, fields[0])
		}
	}
	if err := scanner.Err(); err != nil {
		return TransactionManifest{}, err
	}
	if current != nil {
		return TransactionManifest{}, fmt.Errorf("%s: target is not closed", path)
	}
	if err := validateTransactionManifest(manifest); err != nil {
		return TransactionManifest{}, err
	}
	return manifest, nil
}

func validateTransactionManifest(manifest TransactionManifest) error {
	if manifest.Operation == "" || (manifest.Schema != transactionManifestSchemaV2 && manifest.Schema != transactionManifestSchemaV3) {
		return fmt.Errorf("transaction manifest identity is incomplete")
	}
	if manifest.Schema == transactionManifestSchemaV2 && manifest.TransactionSchema != "gooo/ledger-append-transaction/v2" {
		return fmt.Errorf("transaction manifest identity is incomplete")
	}
	if manifest.Schema == transactionManifestSchemaV3 && manifest.TransactionSchema != "gooo/ledger-append-transaction/v3" {
		return fmt.Errorf("transaction manifest identity is incomplete")
	}
	if len(manifest.AfterInvariants) == 0 || len(manifest.Targets) == 0 {
		return fmt.Errorf("transaction manifest contract is incomplete")
	}
	if manifest.Schema == transactionManifestSchemaV2 && len(manifest.PlannedFiles) != 7 {
		return fmt.Errorf("transaction manifest contract is incomplete")
	}
	if manifest.Schema == transactionManifestSchemaV3 && len(manifest.FileTargets) == 0 {
		return fmt.Errorf("transaction manifest contract is incomplete")
	}
	required := requiredManifestAfterInvariants
	if manifest.Schema == transactionManifestSchemaV3 {
		required = requiredManifestAfterInvariantsV3
	}
	for _, invariant := range required {
		if !contains(manifest.AfterInvariants, invariant) {
			return fmt.Errorf("transaction manifest invariant %q is missing", invariant)
		}
	}
	for key, target := range manifest.Targets {
		if target.Key != key || target.Repository == "" || target.Tag == "" || target.TargetCommitSHA == "" || target.BeforeDigest == "" || target.ExpectedDenominator <= 0 || len(target.ExpectedStateCounts) == 0 || len(target.Anchors) == 0 {
			return fmt.Errorf("transaction manifest target %q is incomplete", key)
		}
	}
	return nil
}

func parseTargetFile(value string) (string, string, string, bool) {
	parts := strings.SplitN(strings.TrimSpace(value), " ", 3)
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" {
		return "", "", "", false
	}
	path, ok := quotedValue(parts[2])
	return parts[0], parts[1], path, ok
}

func parseManifestCounts(value string) (map[string]int, bool) {
	result := map[string]int{}
	for _, field := range strings.Fields(value) {
		parts := strings.SplitN(field, "=", 2)
		if len(parts) != 2 || parts[0] == "" {
			return nil, false
		}
		count, err := strconv.Atoi(parts[1])
		if err != nil || count < 0 {
			return nil, false
		}
		result[parts[0]] = count
	}
	return result, len(result) > 0
}

func quotedValue(value string) (string, bool) {
	parsed, err := strconv.Unquote(strings.TrimSpace(value))
	return parsed, err == nil && parsed != ""
}

func manifestQuotedValue(path string, lineNumber int, line, directive string) (string, error) {
	value, ok := quotedValue(strings.TrimSpace(strings.TrimPrefix(line, directive+" ")))
	if !ok {
		return "", fmt.Errorf("%s:%d: malformed %s", path, lineNumber, directive)
	}
	return value, nil
}
