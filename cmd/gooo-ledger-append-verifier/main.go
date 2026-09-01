// Command gooo-ledger-append-verifier is an independent consumer of planner
// artifacts. It deliberately does not import internal/planner: its checks are
// a second implementation of the boundary between .gooo authority and the
// Go AST executor.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type authority struct {
	paths      map[string]string
	directives map[string][]string
	activities int
	edges      int
}

func main() {
	repository := flag.String("repository", "", "read-only input ledger repository")
	output := flag.String("output", "", "planner output directory")
	transaction := flag.String("transaction", "", "transaction JSON")
	metacode := flag.String("metacode", ".gooo/append-planner.gooo", "Gooo authority file")
	baselineLock := flag.String("baseline-lock", "contracts/upstream-lock-v0.31.0.json", "immutable upstream lock")
	flag.Parse()
	if *repository == "" || *output == "" || *transaction == "" {
		fatal("-repository, -output, and -transaction are required")
	}
	if err := verify(*repository, *output, *transaction, *metacode, *baselineLock); err != nil {
		fatal("independent verification failed: %v", err)
	}
	fmt.Println("independent verification: structural append boundary accepted")
}

func verify(repository, output, transactionPath, metacodePath, baselineLockPath string) error {
	meta, err := parseAuthority(metacodePath)
	if err != nil {
		return err
	}
	tx, err := readObject(transactionPath)
	if err != nil {
		return fmt.Errorf("transaction: %w", err)
	}
	lock, err := readObject(baselineLockPath)
	if err != nil {
		return fmt.Errorf("baseline lock: %w", err)
	}
	plan, err := readObject(filepath.Join(output, "patch-plan.json"))
	if err != nil {
		return fmt.Errorf("patch plan: %w", err)
	}
	receipt, err := readObject(filepath.Join(output, "replay-receipt.json"))
	if err != nil {
		return fmt.Errorf("replay receipt: %w", err)
	}
	if err := verifyAuthority(meta); err != nil {
		return err
	}
	if err := verifyDecisions(plan, receipt); err != nil {
		return err
	}
	if err := verifyBaseline(tx, lock, repository); err != nil {
		return err
	}
	return verifyMaterialized(meta, tx, repository, filepath.Join(output, "repository"), plan)
}

func parseAuthority(path string) (authority, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return authority{}, err
	}
	result := authority{paths: map[string]string{}, directives: map[string][]string{}}
	for lineNumber, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		switch {
		case strings.HasPrefix(line, "activity "):
			result.activities++
		case strings.HasPrefix(line, "edge "):
			result.edges++
		case strings.HasPrefix(line, "path "):
			key, value, ok := quotedPair(strings.TrimPrefix(line, "path "))
			if !ok {
				return authority{}, fmt.Errorf("%s:%d: malformed path", path, lineNumber+1)
			}
			result.paths[key] = value
		default:
			fields := strings.Fields(line)
			if len(fields) == 0 {
				continue
			}
			key := fields[0]
			value := strings.TrimSpace(strings.TrimPrefix(line, key))
			result.directives[key] = append(result.directives[key], value)
		}
	}
	return result, nil
}

func quotedPair(value string) (string, string, bool) {
	parts := strings.SplitN(strings.TrimSpace(value), " ", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	parsed, err := strconv.Unquote(strings.TrimSpace(parts[1]))
	return parts[0], parsed, err == nil && parts[0] != "" && parsed != ""
}

func verifyAuthority(meta authority) error {
	if meta.activities != 9 || meta.edges != 8 {
		return fmt.Errorf(".gooo graph shape is activities=%d edges=%d", meta.activities, meta.edges)
	}
	if !hasDirective(meta, "gooo", "ledger_append_transaction v1") || !hasDirective(meta, "operation", "append_exactly_one_adoption_transaction") {
		return fmt.Errorf(".gooo identity does not declare the append transaction")
	}
	if !hasDirective(meta, "precedence", "REFUTED>UNKNOWN>CLOSED") || !hasDirective(meta, "states", "CLOSED UNKNOWN REFUTED") || !hasDirective(meta, "terminal-state", "CLOSED") {
		return fmt.Errorf(".gooo decision contract is incomplete")
	}
	for _, class := range []string{"FOUNDATION", "COHERENCE", "REGRESSION"} {
		if !hasDirective(meta, "proof-class", class) {
			return fmt.Errorf(".gooo proof class %q is missing", class)
		}
	}
	for _, class := range []string{"DRIVER", "OUTCOME", "GUARDRAIL"} {
		if !hasDirective(meta, "indicator-class", class) {
			return fmt.Errorf(".gooo indicator class %q is missing", class)
		}
	}
	for _, field := range []string{"repository", "tag", "release_id", "tag_object_sha", "target_commit_sha", "immutable"} {
		if !hasDirective(meta, "release-lock-field", field) {
			return fmt.Errorf(".gooo release-lock field %q is missing", field)
		}
	}
	for _, invariant := range []string{"append-only", "ast-addressed-json", "preserve-existing-canonical-objects", "no-input-repository-writes", "no-overwrite"} {
		if !hasDirective(meta, "invariant", invariant) {
			return fmt.Errorf(".gooo invariant %q is missing", invariant)
		}
	}
	for _, key := range []string{"activity_file", "profile", "release_locks", "assessment", "registry", "report", "history"} {
		if meta.paths[key] == "" {
			return fmt.Errorf(".gooo path %q is missing", key)
		}
	}
	return nil
}

func hasDirective(meta authority, key, wanted string) bool {
	for _, value := range meta.directives[key] {
		if value == wanted {
			return true
		}
	}
	return false
}

func verifyDecisions(plan, receipt map[string]any) error {
	if stringField(plan, "decision") != "CLOSED" || stringField(plan, "portfolio_decision") != "REFUTED" {
		return fmt.Errorf("unexpected planner decisions: %s/%s", stringField(plan, "decision"), stringField(plan, "portfolio_decision"))
	}
	if numberField(plan, "metrics", "exact_files_changed") != 7 || numberField(plan, "metrics", "exact_files_planned") != 7 {
		return fmt.Errorf("planner did not report exactly seven file mutations")
	}
	if numberField(plan, "metrics", "repository_writes") != 0 || boolField(plan, "input_repository_mutated") {
		return fmt.Errorf("planner reported a repository write")
	}
	if stringField(receipt, "state") != "CLOSED" {
		return fmt.Errorf("unexpected replay state: %s", stringField(receipt, "state"))
	}
	mismatches, ok := receipt["mismatches"].([]any)
	if !ok || len(mismatches) != 0 {
		return fmt.Errorf("replay receipt contains mismatches")
	}
	files, ok := plan["files"].([]any)
	if !ok || len(files) != 7 {
		return fmt.Errorf("plan contains %d file mutations, want 7", len(files))
	}
	return nil
}

func verifyBaseline(tx, lock map[string]any, repository string) error {
	baseline, ok := tx["baseline"].(map[string]any)
	if !ok {
		return fmt.Errorf("transaction baseline is not an object")
	}
	for _, field := range []string{"repository", "tag", "release_id", "tag_object_sha", "target_commit_sha", "immutable"} {
		if !jsonEqual(baseline[field], lock[field]) {
			return fmt.Errorf("baseline field %s differs from immutable lock", field)
		}
	}
	if !jsonEqual(baseline["source_archive_sha256"], nested(lock, "source_archive", "sha256")) || !jsonEqual(baseline["release_asset_sha256"], nested(lock, "release_asset", "sha256")) || !jsonEqual(baseline["release_asset_id"], nested(lock, "release_asset", "id")) || !jsonEqual(baseline["source_tree_sha256"], lock["source_tree_sha256"]) {
		return fmt.Errorf("baseline digest or asset identity differs from immutable lock")
	}
	if stringField(baseline, "repository") == "" || stringField(baseline, "repository") != "kimjooyoon/gooo-self-improvement-ledger" {
		return fmt.Errorf("unexpected baseline repository")
	}
	observed, err := sourceTreeDigest(repository)
	if err != nil {
		return err
	}
	if observed != stringField(lock, "source_tree_sha256") {
		return fmt.Errorf("input tree digest changed: observed=%s expected=%s", observed, stringField(lock, "source_tree_sha256"))
	}
	return nil
}

func verifyMaterialized(meta authority, tx map[string]any, input, output string, plan map[string]any) error {
	inputFiles, err := snapshot(input)
	if err != nil {
		return fmt.Errorf("snapshot input: %w", err)
	}
	outputFiles, err := snapshot(output)
	if err != nil {
		return fmt.Errorf("snapshot output: %w", err)
	}
	expectedGenerated := map[string]bool{meta.paths["report"]: true, meta.paths["history"]: true}
	expectedModified := map[string]bool{
		meta.paths["activity_file"]: true,
		meta.paths["profile"]:       true,
		meta.paths["release_locks"]: true,
		meta.paths["assessment"]:    true,
		meta.paths["registry"]:      true,
	}
	for path, data := range inputFiles {
		if expectedGenerated[path] || expectedModified[path] {
			continue
		}
		if !bytes.Equal(data, outputFiles[path]) {
			return fmt.Errorf("existing file changed outside the append contract: %s", path)
		}
	}
	for path := range outputFiles {
		if _, exists := inputFiles[path]; !exists && !expectedGenerated[path] {
			return fmt.Errorf("unexpected output file: %s", path)
		}
	}
	for path := range expectedGenerated {
		if _, ok := outputFiles[path]; !ok {
			return fmt.Errorf("projection was not generated: %s", path)
		}
		if !json.Valid(outputFiles[path]) {
			return fmt.Errorf("projection is not valid JSON: %s", path)
		}
	}
	if err := verifyPlanPaths(meta, plan); err != nil {
		return err
	}
	if err := verifyActivity(meta, tx, inputFiles, outputFiles); err != nil {
		return err
	}
	if err := verifyProfile(meta, tx, inputFiles, outputFiles); err != nil {
		return err
	}
	if err := verifyLocks(meta, tx, inputFiles, outputFiles); err != nil {
		return err
	}
	if err := verifyAssessment(meta, tx, inputFiles, outputFiles); err != nil {
		return err
	}
	return verifyRegistry(meta, tx, inputFiles, outputFiles)
}

func verifyPlanPaths(meta authority, plan map[string]any) error {
	files, ok := plan["files"].([]any)
	if !ok {
		return fmt.Errorf("plan files is not an array")
	}
	want := map[string]bool{}
	for _, key := range []string{"activity_file", "profile", "release_locks", "assessment", "registry", "report", "history"} {
		want[meta.paths[key]] = true
	}
	got := map[string]bool{}
	for _, raw := range files {
		file, ok := raw.(map[string]any)
		if !ok || stringField(file, "path") == "" || got[stringField(file, "path")] {
			return fmt.Errorf("plan contains an invalid or duplicate path")
		}
		got[stringField(file, "path")] = true
	}
	if len(got) != len(want) {
		return fmt.Errorf("plan paths are not the seven declared mutation paths")
	}
	for path := range want {
		if !got[path] {
			return fmt.Errorf("plan omitted mutation path %s", path)
		}
	}
	return nil
}

func verifyActivity(meta authority, tx map[string]any, input, output map[string][]byte) error {
	path := meta.paths["activity_file"]
	before, okBefore := input[path]
	after, okAfter := output[path]
	activity, ok := tx["activity"].(map[string]any)
	if !ok || !okBefore || !okAfter {
		return fmt.Errorf("activity binding is unavailable")
	}
	line := fmt.Sprintf("activity %s(%s) -> %s computes %q\n", stringField(activity, "name"), stringField(activity, "input_type"), stringField(activity, "output_type"), stringField(activity, "computes"))
	expected := append([]byte(nil), before...)
	if len(expected) > 0 && expected[len(expected)-1] != '\n' {
		expected = append(expected, '\n')
	}
	expected = append(expected, []byte(line)...)
	if !bytes.Equal(after, expected) {
		return fmt.Errorf("activity file is not an exact AST-addressed append")
	}
	return nil
}

func verifyProfile(meta authority, tx map[string]any, input, output map[string][]byte) error {
	before, err := readObjectBytes(input[meta.paths["profile"]])
	if err != nil {
		return err
	}
	after, err := readObjectBytes(output[meta.paths["profile"]])
	if err != nil {
		return err
	}
	oldCells, okOld := before["cells"].([]any)
	newCells, okNew := after["cells"].([]any)
	newCell := nestedObject(tx, "cell")
	if !okOld || !okNew || len(newCells) != len(oldCells)+1 || newCell == nil {
		return fmt.Errorf("profile does not append exactly one cell")
	}
	for i := range oldCells {
		if !jsonEqual(oldCells[i], newCells[i]) {
			return fmt.Errorf("profile existing cell %d changed", i)
		}
	}
	if !jsonEqual(newCells[len(newCells)-1], newCell) || numberValue(after["total_cells"]) != int64(len(newCells)) {
		return fmt.Errorf("profile appended cell or denominator is incorrect")
	}
	return nil
}

func verifyLocks(meta authority, tx map[string]any, input, output map[string][]byte) error {
	before, err := readObjectBytes(input[meta.paths["release_locks"]])
	if err != nil {
		return err
	}
	after, err := readObjectBytes(output[meta.paths["release_locks"]])
	if err != nil {
		return err
	}
	oldReleases, okOld := before["releases"].(map[string]any)
	newReleases, okNew := after["releases"].(map[string]any)
	key := stringField(tx, "release_key")
	lock, okLock := tx["release_lock"].(map[string]any)
	if !okOld || !okNew || !okLock || len(newReleases) != len(oldReleases)+1 || key == "" || !jsonEqual(newReleases[key], lock) {
		return fmt.Errorf("release lock map did not append exactly one key")
	}
	for name, old := range oldReleases {
		if !jsonEqual(old, newReleases[name]) {
			return fmt.Errorf("existing release lock %s changed", name)
		}
	}
	return nil
}

func verifyAssessment(meta authority, tx map[string]any, input, output map[string][]byte) error {
	before, err := readObjectBytes(input[meta.paths["assessment"]])
	if err != nil {
		return err
	}
	after, err := readObjectBytes(output[meta.paths["assessment"]])
	if err != nil {
		return err
	}
	oldCells, okOld := before["cells"].([]any)
	newCells, okNew := after["cells"].([]any)
	if !okOld || !okNew || len(newCells) != len(oldCells)+1 {
		return fmt.Errorf("assessment does not append exactly one cell")
	}
	for i := range oldCells {
		if !jsonEqual(oldCells[i], newCells[i]) {
			return fmt.Errorf("assessment existing cell %d changed", i)
		}
	}
	cell := nestedObject(tx, "cell")
	outcome := nestedObject(tx, "outcome")
	expected := map[string]any{"cell_id": stringField(cell, "id"), "state": stringField(outcome, "state"), "release_key": stringField(cell, "release_key"), "evidence": outcome["evidence"]}
	if expected["evidence"] == nil {
		expected["evidence"] = []any{}
	}
	if unknown, ok := outcome["unknown"]; ok && unknown != nil {
		expected["unknown"] = unknown
	}
	if refutation, ok := outcome["refutation"]; ok && refutation != nil {
		expected["refutation"] = refutation
	}
	if !jsonEqual(newCells[len(newCells)-1], expected) {
		return fmt.Errorf("assessment appended cell differs from transaction outcome")
	}
	return nil
}

func verifyRegistry(meta authority, tx map[string]any, input, output map[string][]byte) error {
	before, err := readObjectBytes(input[meta.paths["registry"]])
	if err != nil {
		return err
	}
	after, err := readObjectBytes(output[meta.paths["registry"]])
	if err != nil {
		return err
	}
	oldEntries, okOld := before["entries"].([]any)
	newEntries, okNew := after["entries"].([]any)
	entry, okEntry := tx["registry_entry"].(map[string]any)
	if !okOld || !okNew || !okEntry || len(newEntries) != len(oldEntries)+1 || !jsonEqual(newEntries[len(newEntries)-1], entry) {
		return fmt.Errorf("registry does not append exactly one entry")
	}
	for i := range oldEntries {
		if !jsonEqual(oldEntries[i], newEntries[i]) {
			return fmt.Errorf("registry existing entry %d changed", i)
		}
	}
	if numberValue(after["entry_count"]) != int64(len(newEntries)) {
		return fmt.Errorf("registry entry_count is inconsistent")
	}
	return nil
}

func readObject(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return readObjectBytes(data)
}

func readObjectBytes(data []byte) (map[string]any, error) {
	var value map[string]any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	return value, nil
}

func snapshot(root string) (map[string][]byte, error) {
	files := map[string][]byte{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return fs.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() {
			return fmt.Errorf("unsafe file in %s: %s", root, rel)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = data
		return nil
	})
	return files, err
}

func sourceTreeDigest(root string) (string, error) {
	files, err := snapshot(root)
	if err != nil {
		return "", err
	}
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	hash := sha256.New()
	for _, path := range paths {
		data := files[path]
		hash.Write([]byte("file\x00"))
		hash.Write([]byte(path))
		hash.Write([]byte("\x00"))
		hash.Write([]byte(strconv.FormatInt(int64(len(data)), 10)))
		hash.Write([]byte("\x00"))
		hash.Write(data)
		hash.Write([]byte("\x00"))
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}

func jsonEqual(left, right any) bool {
	leftBytes, leftErr := json.Marshal(left)
	rightBytes, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftBytes, rightBytes)
}

func nested(value map[string]any, parent, child string) any {
	object, _ := value[parent].(map[string]any)
	return object[child]
}

func nestedObject(value map[string]any, key string) map[string]any {
	object, _ := value[key].(map[string]any)
	return object
}

func stringField(value map[string]any, key string) string {
	result, _ := value[key].(string)
	return result
}

func numberField(value map[string]any, parent, child string) int64 {
	object, _ := value[parent].(map[string]any)
	return numberValue(object[child])
}

func numberValue(value any) int64 {
	switch current := value.(type) {
	case json.Number:
		result, _ := current.Int64()
		return result
	case float64:
		return int64(current)
	case int:
		return int64(current)
	case int64:
		return current
	default:
		return 0
	}
}

func boolField(value map[string]any, key string) bool {
	result, _ := value[key].(bool)
	return result
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
