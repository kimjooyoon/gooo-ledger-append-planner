package planner

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type JSONFile struct {
	Path string
	Raw  []byte
	AST  any
}

type LedgerAST struct {
	ActivityFile string
	ActivityRaw  []byte
	Profile      JSONFile
	Locks        JSONFile
	Assessment   JSONFile
	Registry     JSONFile
	Files        map[string][]byte
}

type LedgerSnapshot struct {
	Files map[string]any `json:"files"`
}

func readLedger(root string, meta MetaCode) (LedgerAST, error) {
	paths := meta.Paths
	activityPath := filepath.Join(root, filepath.FromSlash(paths["activity_file"]))
	activityRaw, err := os.ReadFile(activityPath)
	if err != nil {
		return LedgerAST{}, fmt.Errorf("read activity file: %w", err)
	}
	readJSONFile := func(key string) (JSONFile, error) {
		rel := paths[key]
		if rel == "" {
			return JSONFile{}, fmt.Errorf("metacode path %q is missing", key)
		}
		path := filepath.Join(root, filepath.FromSlash(rel))
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return JSONFile{}, fmt.Errorf("read %s: %w", rel, readErr)
		}
		var value any
		if decodeErr := decodeJSON(raw, &value); decodeErr != nil {
			return JSONFile{}, fmt.Errorf("decode %s: %w", rel, decodeErr)
		}
		return JSONFile{Path: rel, Raw: raw, AST: value}, nil
	}
	profile, err := readJSONFile("profile")
	if err != nil {
		return LedgerAST{}, err
	}
	locks, err := readJSONFile("release_locks")
	if err != nil {
		return LedgerAST{}, err
	}
	assessment, err := readJSONFile("assessment")
	if err != nil {
		return LedgerAST{}, err
	}
	registry, err := readJSONFile("registry")
	if err != nil {
		return LedgerAST{}, err
	}
	files, err := snapshotFiles(root)
	if err != nil {
		return LedgerAST{}, err
	}
	return LedgerAST{ActivityFile: paths["activity_file"], ActivityRaw: activityRaw, Profile: profile, Locks: locks, Assessment: assessment, Registry: registry, Files: files}, nil
}

func snapshotFiles(root string) (map[string][]byte, error) {
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
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink is not a safe ledger input: %s", filepath.ToSlash(rel))
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("unsupported ledger input file type: %s", filepath.ToSlash(rel))
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

func canonicalJSON(value any) ([]byte, error) {
	normalized, err := normalizeJSON(value)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(normalized)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func normalizeJSON(value any) (any, error) {
	switch current := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(current))
		for key, child := range current {
			normalized, err := normalizeJSON(child)
			if err != nil {
				return nil, err
			}
			result[key] = normalized
		}
		return result, nil
	case []any:
		result := make([]any, len(current))
		for index, child := range current {
			normalized, err := normalizeJSON(child)
			if err != nil {
				return nil, err
			}
			result[index] = normalized
		}
		return result, nil
	case json.Number:
		if _, err := strconv.ParseFloat(string(current), 64); err != nil {
			return nil, fmt.Errorf("invalid JSON number %q: %w", current, err)
		}
		return current, nil
	default:
		return value, nil
	}
}

func canonicalDigest(value any) (string, error) {
	data, err := canonicalJSON(value)
	if err != nil {
		return "", err
	}
	return digestBytes(data), nil
}

func digestBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func fileDigest(data []byte) string {
	return digestBytes(data)
}

func ledgerDigest(ledger LedgerAST, includeFiles map[string][]byte) (string, error) {
	paths := make([]string, 0, len(includeFiles))
	for path := range includeFiles {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	entries := make(map[string]any, len(paths))
	for _, path := range paths {
		data := includeFiles[path]
		if isJSONPath(path) {
			var value any
			if err := decodeJSON(data, &value); err != nil {
				return "", fmt.Errorf("decode snapshot %s: %w", path, err)
			}
			entries[path] = value
		} else {
			entries[path] = string(data)
		}
	}
	entries[ledger.ActivityFile] = string(ledger.ActivityRaw)
	return canonicalDigest(LedgerSnapshot{Files: entries})
}

func isJSONPath(path string) bool {
	return strings.HasSuffix(path, ".json")
}

func object(value any, path string) (map[string]any, error) {
	result, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s is not a JSON object", path)
	}
	return result, nil
}

func array(value any, path string) ([]any, error) {
	result, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("%s is not a JSON array", path)
	}
	return result, nil
}

func stringField(value any, path string) (string, error) {
	result, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("%s is not a string", path)
	}
	return result, nil
}

func numberField(value any, path string) (int64, error) {
	number, ok := value.(json.Number)
	if !ok {
		return 0, fmt.Errorf("%s is not a JSON number", path)
	}
	result, err := strconv.ParseInt(string(number), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s is not an integer: %w", path, err)
	}
	return result, nil
}

func cloneJSON(value any) (any, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var cloned any
	if err := decodeJSON(data, &cloned); err != nil {
		return nil, err
	}
	return cloned, nil
}

func jsonBytes(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func encodeBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

func decodeBase64(value string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(value)
}

func sameCanonical(left, right any) bool {
	leftData, leftErr := canonicalJSON(left)
	rightData, rightErr := canonicalJSON(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftData, rightData)
}

func sourceTreeDigest(root string) (string, error) {
	files, err := snapshotFiles(root)
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
