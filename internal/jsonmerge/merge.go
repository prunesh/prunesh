// Package jsonmerge deep-merges a JSON patch into a config file.
// Used by `prunesh json-merge` so the installer can patch hooks.json without jq.
package jsonmerge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// MergeFile reads filePath (or starts from {} if it does not exist),
// reads a JSON patch from stdin, deep-merges the patch into the file,
// and writes the result back atomically.
//
// Merge rules:
//   - objects are merged recursively: patch keys are added or overwrite target
//   - arrays are concatenated; items already present (by JSON equality) are not duplicated
//   - scalars: patch value wins
func MergeFile(filePath string) (changed bool, err error) {
	if filePath == "" {
		return false, fmt.Errorf("file path is empty")
	}
	var patch any
	if err := json.NewDecoder(os.Stdin).Decode(&patch); err != nil {
		return false, fmt.Errorf("read patch from stdin: %w", err)
	}
	return MergeValue(filePath, patch)
}

// MergeValue reads filePath (or starts from {} if it does not exist),
// deep-merges patch into it, and writes the result back atomically.
func MergeValue(filePath string, patch any) (changed bool, err error) {
	if filePath == "" {
		return false, fmt.Errorf("file path is empty")
	}
	if fi, statErr := os.Lstat(filePath); statErr == nil && fi.Mode()&os.ModeSymlink != 0 {
		resolved, evalErr := filepath.EvalSymlinks(filePath)
		if evalErr != nil {
			return false, fmt.Errorf("resolve symlink %s: %w", filePath, evalErr)
		}
		filePath = resolved
	} else if statErr != nil && !os.IsNotExist(statErr) {
		return false, fmt.Errorf("stat %s: %w", filePath, statErr)
	}

	var target any = map[string]any{}
	data, err := os.ReadFile(filePath)
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("read %s: %w", filePath, err)
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &target); err != nil {
			return false, fmt.Errorf("parse %s: %w", filePath, err)
		}
	}

	merged := deepMerge(target, patch)

	mergedBytes, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return false, fmt.Errorf("marshal result: %w", err)
	}
	mergedBytes = append(mergedBytes, '\n')

	if len(data) > 0 {
		var norm any
		if json.Unmarshal(data, &norm) == nil {
			normBytes, _ := json.MarshalIndent(norm, "", "  ")
			normBytes = append(normBytes, '\n')
			if string(mergedBytes) == string(normBytes) {
				return false, nil
			}
		}
	}

	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return false, fmt.Errorf("mkdir: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(filePath), ".prunesh-jsonmerge-*")
	if err != nil {
		return false, fmt.Errorf("tempfile: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		if err != nil {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err = tmp.Write(mergedBytes); err != nil {
		tmp.Close()
		return false, fmt.Errorf("write temp: %w", err)
	}
	if err = tmp.Close(); err != nil {
		return false, fmt.Errorf("close temp: %w", err)
	}
	if err = os.Rename(tmpName, filePath); err != nil {
		return false, fmt.Errorf("rename: %w", err)
	}
	return true, nil
}

func deepMerge(target, patch any) any {
	tMap, tIsMap := target.(map[string]any)
	pMap, pIsMap := patch.(map[string]any)
	if tIsMap && pIsMap {
		result := make(map[string]any, len(tMap))
		for k, v := range tMap {
			result[k] = v
		}
		for k, pv := range pMap {
			if tv, exists := result[k]; exists {
				result[k] = deepMerge(tv, pv)
			} else {
				result[k] = pv
			}
		}
		return result
	}

	tSlice, tIsSlice := target.([]any)
	pSlice, pIsSlice := patch.([]any)
	if tIsSlice && pIsSlice {
		return dedupConcat(tSlice, pSlice)
	}

	return patch
}

func dedupConcat(a, b []any) []any {
	result := make([]any, len(a))
	copy(result, a)
	for _, item := range b {
		if !containsJSON(result, item) {
			result = append(result, item)
		}
	}
	return result
}

func containsJSON(slice []any, item any) bool {
	itemBytes, err := json.Marshal(item)
	if err != nil {
		return false
	}
	for _, existing := range slice {
		existingBytes, err := json.Marshal(existing)
		if err != nil {
			continue
		}
		if string(existingBytes) == string(itemBytes) {
			return true
		}
	}
	return false
}
