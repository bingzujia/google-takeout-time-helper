package matcher

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/bingzujia/g_photo_take_out_helper/internal/parser"
)

// GooglePhoto holds parsed data from a Google Takeout JSON sidecar.
type GooglePhoto struct {
	PhotoTakenTime struct {
		Timestamp string `json:"timestamp"`
	} `json:"photoTakenTime"`
	GeoData struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
		Altitude  float64 `json:"altitude"`
	} `json:"geoData"`
}

// MatchResult represents a matched (json, photo) pair.
type MatchResult struct {
	JSONFile  string
	PhotoFile string
	Timestamp time.Time
	Lat       float64
	Lon       float64
	Alt       float64
}

var chineseSuffixes = []string{"-已修改", "-编辑", "-修改", "-edited", "-modified"}

// MatchAll walks dir (non-recursively) and returns all matched pairs.
// Second return value is the list of unmatched JSON files.
func MatchAll(dir string) ([]MatchResult, []string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, err
	}

	// build a set of all non-json files
	nonJSON := map[string]string{} // lowercase basename -> actual path
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.EqualFold(filepath.Ext(name), ".json") {
			continue
		}
		nonJSON[strings.ToLower(name)] = filepath.Join(dir, name)
	}

	var results []MatchResult
	var unmatched []string

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.EqualFold(filepath.Ext(name), ".json") {
			continue
		}

		jsonPath := filepath.Join(dir, name)
		gp, err := parseJSON(jsonPath)
		if err != nil {
			unmatched = append(unmatched, jsonPath)
			continue
		}

		baseName := deriveBaseName(name)
		photoPath, found := findPhoto(baseName, nonJSON, dir)
		if !found {
			unmatched = append(unmatched, jsonPath)
			continue
		}

		ts := resolveTimestamp(photoPath, gp)
		results = append(results, MatchResult{
			JSONFile:  jsonPath,
			PhotoFile: photoPath,
			Timestamp: ts,
			Lat:       gp.GeoData.Latitude,
			Lon:       gp.GeoData.Longitude,
			Alt:       gp.GeoData.Altitude,
		})
	}

	return results, unmatched, nil
}

// deriveBaseName strips ".json" and handles supplemental-metadata naming.
func deriveBaseName(jsonName string) string {
	// Strip .json extension
	base := strings.TrimSuffix(jsonName, filepath.Ext(jsonName))

	// Handle supplemental-metadata: "photo.jpg(1).json" -> look for "photo(1).jpg"
	// Google Takeout truncates long names: "Screenshot_xxx8(1).json" can map to "Screenshot_xxx(1).jpg"
	return base
}

// findPhoto tries multiple strategies to match a photo file to a JSON basename.
func findPhoto(baseName string, nonJSON map[string]string, dir string) (string, bool) {
	// Strategy 1: exact match (baseName already has an extension)
	if path, ok := nonJSON[strings.ToLower(baseName)]; ok {
		return path, true
	}

	// Strategy 2: try variant suffixes before the extension
	ext := filepath.Ext(baseName)
	stem := strings.TrimSuffix(baseName, ext)
	if ext != "" {
		for _, suffix := range chineseSuffixes {
			candidate := strings.ToLower(stem + suffix + ext)
			if path, ok := nonJSON[candidate]; ok {
				return path, true
			}
		}
	}

	// Strategy 3: fuzzy glob - if exactly 1 non-json file matches baseName*
	matches := globPrefix(baseName, nonJSON)
	if len(matches) == 1 {
		return matches[0], true
	}

	// Strategy 4: numbered-prefix truncation match
	// Handle cases like "Screenshot_xxx8(1)" -> "Screenshot_xxx(1)"
	if path, ok := truncationMatch(stem, ext, nonJSON); ok {
		return path, true
	}

	return "", false
}

// globPrefix returns all files whose lowercase name starts with the lowercase prefix.
func globPrefix(prefix string, nonJSON map[string]string) []string {
	lower := strings.ToLower(prefix)
	var results []string
	for k, v := range nonJSON {
		if strings.HasPrefix(k, lower) {
			results = append(results, v)
		}
	}
	return results
}

// truncationMatch handles Google Takeout's filename truncation for numbered variants.
// e.g. "Screenshot_2016-02-28-13-06-348(1)" (json stem) might map to
// "Screenshot_2016-02-28-13-06-34(1).jpg" (actual file, truncated by 1+ chars before "(N)")
func truncationMatch(stem, ext string, nonJSON map[string]string) (string, bool) {
	// Look for patterns like "name(N)" where the number indicates a duplicate
	idx := strings.LastIndex(stem, "(")
	if idx < 0 {
		return "", false
	}
	closingOffset := strings.Index(stem[idx:], ")")
	if closingOffset < 0 {
		return "", false
	}
	numStr := stem[idx+1 : idx+closingOffset]
	if _, err := strconv.Atoi(numStr); err != nil {
		return "", false
	}
	suffix := stem[idx:] // "(N)"
	nameBase := stem[:idx]

	// Try progressively shorter stems combined with the "(N)" suffix
	for i := 1; i <= len(nameBase) && i <= 10; i++ {
		candidate := strings.ToLower(nameBase[:len(nameBase)-i] + suffix + ext)
		if path, ok := nonJSON[candidate]; ok {
			return path, true
		}
	}
	return "", false
}

func parseJSON(path string) (*GooglePhoto, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var gp GooglePhoto
	if err := json.Unmarshal(data, &gp); err != nil {
		return nil, err
	}
	return &gp, nil
}

func resolveTimestamp(photoPath string, gp *GooglePhoto) time.Time {
	if t, ok := parser.ParseFilenameTimestamp(filepath.Base(photoPath)); ok {
		return t
	}
	if gp.PhotoTakenTime.Timestamp != "" {
		sec, err := strconv.ParseInt(gp.PhotoTakenTime.Timestamp, 10, 64)
		if err == nil {
			return time.Unix(sec, 0).UTC()
		}
	}
	return time.Time{}
}
