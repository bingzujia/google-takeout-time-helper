// Package exifrunner provides batched exiftool queries to avoid per-file subprocess spawning.
package exifrunner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"sync"
)

const batchSize = 1000

var (
	pathOnce      sync.Once
	exiftoolPath  string
	exiftoolFound bool
)

// LookupPath returns the exiftool binary path and whether it is available.
// The result is cached after the first call.
func LookupPath() (string, bool) {
	pathOnce.Do(func() {
		p, err := exec.LookPath("exiftool")
		if err == nil {
			exiftoolPath = p
			exiftoolFound = true
		}
	})
	return exiftoolPath, exiftoolFound
}

// BatchQuery runs a single exiftool -j query for the requested tags over all paths,
// processing up to 1000 files per subprocess call. Returns one map per path in the
// same order. Missing tags are absent from the map. Returns an error only when
// exiftool is unavailable or JSON parsing fails; per-file EXIF errors are silently
// omitted (empty map for that entry).
func BatchQuery(paths []string, tags []string) ([]map[string]string, error) {
	if len(paths) == 0 {
		return nil, nil
	}

	toolPath, ok := LookupPath()
	if !ok {
		// Return empty maps so callers degrade gracefully.
		results := make([]map[string]string, len(paths))
		for i := range results {
			results[i] = map[string]string{}
		}
		return results, nil
	}

	results := make([]map[string]string, len(paths))

	// Process in batches to avoid argument-list-too-long errors.
	for start := 0; start < len(paths); start += batchSize {
		end := start + batchSize
		if end > len(paths) {
			end = len(paths)
		}
		batch := paths[start:end]

		batchResults, err := queryBatch(toolPath, batch, tags)
		if err != nil {
			return nil, err
		}

		// Index results by SourceFile so we can match them back to original order.
		indexed := make(map[string]map[string]string, len(batchResults))
		for _, m := range batchResults {
			if src, ok := m["SourceFile"]; ok {
				indexed[src] = m
			}
		}

		for i, path := range batch {
			if m, ok := indexed[path]; ok {
				results[start+i] = m
			} else {
				results[start+i] = map[string]string{}
			}
		}
	}

	return results, nil
}

// queryBatch runs exiftool -j over a single batch of paths and returns raw parsed maps.
func queryBatch(toolPath string, paths []string, tags []string) ([]map[string]string, error) {
	args := make([]string, 0, len(tags)+len(paths)+1)
	for _, tag := range tags {
		args = append(args, "-"+tag)
	}
	args = append(args, "-j")
	args = append(args, paths...)

	var stdout bytes.Buffer
	cmd := exec.Command(toolPath, args...)
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		// exiftool exits non-zero for some files; if there is any JSON output, try parsing.
		if stdout.Len() == 0 {
			return nil, fmt.Errorf("exiftool batch query: %w", err)
		}
	}

	var raw []map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &raw); err != nil {
		return nil, fmt.Errorf("parse exiftool JSON: %w", err)
	}

	results := make([]map[string]string, len(raw))
	for i, record := range raw {
		m := make(map[string]string, len(record))
		for k, v := range record {
			switch sv := v.(type) {
			case string:
				m[k] = sv
			default:
				m[k] = fmt.Sprint(v)
			}
		}
		results[i] = m
	}
	return results, nil
}
