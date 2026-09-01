package planner

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

// ReplayRollback applies only the inverse recorded in a generated rollback bundle.
// It refuses to touch a repository whose changed files no longer match the bundle.
func ReplayRollback(bundlePath, repositoryRoot string) (ReplayReceipt, error) {
	raw, err := os.ReadFile(bundlePath)
	if err != nil {
		return ReplayReceipt{}, err
	}
	var bundle RollbackBundle
	if err := decodeJSON(raw, &bundle); err != nil {
		return ReplayReceipt{}, err
	}
	if bundle.Schema != "gooo/ledger-append-planner/rollback-bundle/v1" {
		return ReplayReceipt{}, fmt.Errorf("unsupported rollback bundle schema %q", bundle.Schema)
	}
	mismatches := []string{}
	for _, file := range bundle.Files {
		path := filepath.Join(repositoryRoot, filepath.FromSlash(file.Path))
		data, readErr := os.ReadFile(path)
		if !file.BeforeExists {
			if readErr == nil && file.AfterDigest != fileDigest(data) {
				mismatches = append(mismatches, file.Path+":after-digest")
			}
			if readErr != nil && !os.IsNotExist(readErr) {
				mismatches = append(mismatches, file.Path+":read")
			}
			continue
		}
		if readErr != nil || file.AfterDigest != fileDigest(data) {
			mismatches = append(mismatches, file.Path+":after-digest")
		}
	}
	if len(mismatches) == 0 {
		for _, file := range bundle.Files {
			path := filepath.Join(repositoryRoot, filepath.FromSlash(file.Path))
			if file.BeforeExists {
				data, err := decodeBase64(file.BeforeBase64)
				if err != nil {
					return ReplayReceipt{}, err
				}
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					return ReplayReceipt{}, err
				}
				if err := os.WriteFile(path, data, 0o644); err != nil {
					return ReplayReceipt{}, err
				}
			} else if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return ReplayReceipt{}, err
			}
		}
	}
	state := bundle.Replay.State
	if state == "" {
		return ReplayReceipt{}, fmt.Errorf("rollback bundle replay state is missing")
	}
	receipt := ReplayReceipt{Schema: "gooo/ledger-append-planner/replay-receipt/v1", TransactionID: bundle.TransactionID, State: state, BeforeDigest: bundle.BeforeDigest, AfterDigest: bundle.AfterDigest, ObservedAfterDigest: bundle.BeforeDigest, Mismatches: mismatches, RollbackReady: false, RepositoryWrites: 0}
	if len(mismatches) > 0 {
		receipt.State = DecisionRefuted
		return receipt, nil
	}
	if err := verifyRollbackFiles(repositoryRoot, bundle); err != nil {
		receipt.State = DecisionRefuted
		receipt.Mismatches = append(receipt.Mismatches, err.Error())
	}
	return receipt, nil
}

func verifyRollbackFiles(root string, bundle RollbackBundle) error {
	for _, file := range bundle.Files {
		path := filepath.Join(root, filepath.FromSlash(file.Path))
		data, err := os.ReadFile(path)
		if !file.BeforeExists {
			if err == nil {
				return fmt.Errorf("rollback file still exists: %s", file.Path)
			}
			if !os.IsNotExist(err) {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		before, err := decodeBase64(file.BeforeBase64)
		if err != nil {
			return err
		}
		if !bytes.Equal(data, before) || file.BeforeDigest != fileDigest(data) {
			return fmt.Errorf("rollback digest mismatch: %s", file.Path)
		}
	}
	return nil
}
