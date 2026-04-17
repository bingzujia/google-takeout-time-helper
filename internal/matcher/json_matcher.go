package matcher

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/bingzujia/google-takeout-time-helper/internal/parser"
)

// maxTakeoutFilenameLength is Google Takeout's filename length limit.
// When filename + ".json" exceeds this, the name is truncated.
const maxTakeoutFilenameLength = 51

// extraFormats lists known "edited" suffixes in multiple languages that Google
// Takeout may append to filenames. Order matters: the first match wins.
var extraFormats = []string{
	// Chinese (Simplified)
	"-已修改",
	"-编辑",
	"-修改",
	// English/US
	"-edited",
	"-effects",
	"-smile",
	"-mix",
	// Polish
	"-edytowane",
	// German
	"-bearbeitet",
	// Dutch
	"-bewerkt",
	// Japanese
	"-編集済み",
	// Italian
	"-modificato",
	// French (with accent)
	"-modifié",
	// Spanish (with space)
	"-ha editado",
	// Catalan
	"-editat",
}

// bracketSwapRegex matches "(digits)." pattern, used to find the last occurrence.
var bracketSwapRegex = regexp.MustCompile(`\(\d+\)\.`)

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
	CameraMake           string `json:"cameraMake"`
	CameraModel          string `json:"cameraModel"`
	GooglePhotosOrigin   struct {
		MobileUpload struct {
			DeviceFolder struct {
				LocalFolderName string `json:"localFolderName"`
			} `json:"deviceFolder"`
			DeviceType string `json:"deviceType"`
		} `json:"mobileUpload"`
	} `json:"googlePhotosOrigin"`
}

// JSONLookupResult holds the result of looking up a JSON sidecar for a photo.
type JSONLookupResult struct {
	JSONFile      string    // path to the matched JSON file
	Timestamp     time.Time // extracted photo taken time (zero if parsing failed)
	Lat           float64   // latitude from geoData
	Lon           float64   // longitude from geoData
	Alt           float64   // altitude from geoData
	CameraMake    string    // device manufacturer
	CameraModel   string    // device model
	DeviceFolder  string    // device folder name from googlePhotosOrigin.mobileUpload.deviceFolder
	DeviceType    string    // device type from googlePhotosOrigin.mobileUpload
	GooglePhoto   *GooglePhoto // raw parsed JSON (for ResolveGPS caller access)
}

// supplementalSuffixes lists known supplemental-metadata suffixes that Google
// Takeout appends to JSON sidecar filenames. The JSON file is named as
// "photo.ext.<suffix>.json" while the photo is "photo.ext".
var supplementalSuffixes = []string{
	"supplemental-met",   // truncated form (51-char limit)
	"supplemental-metadata",
	"supplemen",          // further truncated form
	"supp",               // shorter truncation
	"s",                  // shortest truncation
}

// supplementalRegex matches any "photo.ext.<suffix>.json" pattern where
// <suffix> starts with "supplemental" or a truncated variant.
// This covers all possible truncations due to the 51-char filename limit.
var supplementalRegex = regexp.MustCompile(`^(.+)\.supp[a-z]*\.json$`)

// JSONForFile looks up the JSON sidecar file for a given photo file using
// a 6-step degradation strategy (mirroring the Dart implementation, without
// the tryhard mode).
//
// Strategy order (safety decreasing):
//
//	1. Identity — try the original filename as-is
//	2. ShortenName — truncate to 46 chars if filename+.json exceeds 51 chars
//	3. BracketSwap — move "(N)" from before extension to after it
//	4. RemoveExtra — remove known "edited" suffixes (15 languages)
//	5. Supplemental — try supplemental-metadata suffixes
//	6. NoExtension — strip the file extension entirely
//
// Returns nil if no JSON sidecar is found after all 6 strategies.
func JSONForFile(photoPath string, cache *DirCache) *JSONLookupResult {
	dir := filepath.Dir(photoPath)
	name := filepath.Base(photoPath)

	// Build the transformation methods (no tryhard mode)
	methods := []func(string) string{
		methodIdentity,
		methodShortenName,
		methodBracketSwap,
		methodRemoveExtra,
		methodNoExtension,
	}

	for _, method := range methods {
		transformedName := method(name)
		jsonPath := filepath.Join(dir, transformedName+".json")

		if _, err := os.Stat(jsonPath); err == nil {
			return parseJSONLookup(jsonPath)
		}
	}

	// Strategy 4b: double-dot JSON naming (filename.ext..json)
	// Google Takeout sometimes names JSON sidecars as "photo.ext..json"
	// (double dot) when the original filename already contains an extension.
	doubleDotPath := filepath.Join(dir, name+"..json")
	if _, err := os.Stat(doubleDotPath); err == nil {
		return parseJSONLookup(doubleDotPath)
	}

	// Strategy 5: Supplemental-metadata suffixes
	// The photo is "photo.ext" but JSON is "photo.ext.<suffix>.json"
	// Step 5a: try known suffix variants on the original name
	for _, suffix := range supplementalSuffixes {
		jsonPath := filepath.Join(dir, name+"."+suffix+".json")
		if _, err := os.Stat(jsonPath); err == nil {
			return parseJSONLookup(jsonPath)
		}
	}

	// Step 5a2: try known suffix variants on the RemoveExtra-transformed name
	// e.g. "IMG_20210629_114736-已修改.jpg" → RemoveExtra → "IMG_20210629_114736.jpg"
	//   → try "IMG_20210629_114736.jpg.supplemental-metadata.json"
	cleanedName := methodRemoveExtra(name)
	if cleanedName != name {
		for _, suffix := range supplementalSuffixes {
			jsonPath := filepath.Join(dir, cleanedName+"."+suffix+".json")
			if _, err := os.Stat(jsonPath); err == nil {
				return parseJSONLookup(jsonPath)
			}
		}
	}

	// Step 5b: regex fallback — scan directory for any JSON matching
	// "photo.ext.supp*.json" or "base.ext.supp*(N).json" (numbered duplicates)
	//
	// Handles two cases:
	//   a) photo.ext → photo.ext.supp*.json
	//   b) photo(N).ext → photo.ext.supp*(N).json
	escapedName := regexp.QuoteMeta(name)
	pattern := regexp.MustCompile(`^` + escapedName + `\.su[a-z-]*(\(\d+\))?\.json$`)
	entries, err := cache.ReadDir(dir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if pattern.MatchString(e.Name()) {
				jsonPath := filepath.Join(dir, e.Name())
				return parseJSONLookup(jsonPath)
			}
		}
	}

	// Step 5b2: regex fallback on the RemoveExtra-transformed name
	// e.g. "IMG_20210629_114736-已修改.jpg" → cleaned → "IMG_20210629_114736.jpg"
	//   → scan for "IMG_20210629_114736.jpg.supp*.json"
	if cleanedName != name {
		escapedCleaned := regexp.QuoteMeta(cleanedName)
		cleanedPattern := regexp.MustCompile(`^` + escapedCleaned + `\.su[a-z-]*(\(\d+\))?\.json$`)
		if err == nil {
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				if cleanedPattern.MatchString(e.Name()) {
					jsonPath := filepath.Join(dir, e.Name())
					return parseJSONLookup(jsonPath)
				}
			}
		}
	}

	// Step 5c: handle numbered duplicates where (N) moves from photo to JSON suffix
	// photo: "IMG20240405102259(1).heic" → JSON: "IMG20240405102259.heic.supplemental-metadata(1).json"
	bracketNumRegex := regexp.MustCompile(`^(.+)\((\d+)\)\.(\w+)$`)
	if m := bracketNumRegex.FindStringSubmatch(name); m != nil {
		baseName := m[1] // "IMG20240405102259"
		num := m[2]      // "1"
		ext := m[3]      // "heic"
		// Match: baseName.ext.supp*(num).json
		escapedBase := regexp.QuoteMeta(baseName)
		escapedExt := regexp.QuoteMeta(ext)
		numPattern := regexp.MustCompile(`^` + escapedBase + `\.` + escapedExt + `\.su[a-z-]*\(` + num + `\)\.json$`)
		if err == nil {
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				if numPattern.MatchString(e.Name()) {
					jsonPath := filepath.Join(dir, e.Name())
					return parseJSONLookup(jsonPath)
				}
			}
		}
	}

	return nil
}

// methodIdentity returns the filename unchanged.
// Corresponds to Dart: (String s) => s
func methodIdentity(filename string) string {
	return filename
}

// methodShortenName truncates the filename if filename+".json" exceeds 51 chars.
// Google Takeout has a 51-character limit on sidecar filenames.
//
// Logic branches:
//   - len(filename+".json") > 51 → truncate filename to first 46 chars (51 - 5)
//   - len(filename+".json") <= 51 → return filename unchanged
func methodShortenName(filename string) string {
	if len(filename)+len(".json") > maxTakeoutFilenameLength {
		return filename[:maxTakeoutFilenameLength-len(".json")]
	}
	return filename
}

// methodBracketSwap moves the last "(digits)." pattern from before the
// extension to after it.
//
// Logic branches:
//   - No "(digits)." match → return filename unchanged
//   - Match found → extract "(N)", remove it from original position, append to end
//
// Example: "image(11).jpg" → "image.jpg(11)"
//
// Uses lastOrNull to handle cases like "image(3).(2)(3).jpg" correctly —
// the last match "(3)." is the one before the extension.
func methodBracketSwap(filename string) string {
	// Find all matches and take the last one
	matches := bracketSwapRegex.FindAllStringIndex(filename, -1)
	if len(matches) == 0 {
		return filename
	}

	// Get the last match
	lastMatch := matches[len(matches)-1]
	bracketWithDot := filename[lastMatch[0]:lastMatch[1]] // e.g. "(11)."
	bracket := strings.TrimSuffix(bracketWithDot, ".")    // e.g. "(11)"

	// Remove the bracket (without dot) from filename, keeping the dot
	// e.g. "image(11).jpg" → "image.jpg"
	withoutBracket := filename[:lastMatch[0]] + filename[lastMatch[0]+len(bracket):]

	// Append bracket to the end
	return withoutBracket + bracket
}

// methodRemoveExtra removes known "edited" suffixes from the filename.
//
// Logic branches:
//   - NFC normalize the filename (handle macOS NFD encoding differences)
//   - Iterate extraFormats (12 language suffixes) in order
//   - For each suffix, check if filename contains it
//     - Contains → remove last occurrence, return immediately
//     - Not contains → continue to next suffix
//   - No suffix matches → return filename unchanged
//
// Uses replaceLast (not replaceAll) to avoid removing strings from the
// middle of the filename. E.g. "my-edited-photo-edited.jpg" only removes
// the trailing "-edited".
func methodRemoveExtra(filename string) string {
	filename = nfcNormalize(filename)
	for _, extra := range extraFormats {
		if strings.Contains(filename, extra) {
			return replaceLast(filename, extra, "")
		}
	}
	return filename
}

// methodNoExtension strips the file extension from the filename.
//
// Logic:
//   - Extract basename without the last extension
//   - "archive.tar.gz" → "archive.tar" (only removes the last extension)
//
// Design reason: original files uploaded without extensions (e.g. "20030616")
// get extensions added by Google (becomes "20030616.jpg"), but the JSON
// sidecar still uses the extensionless name ("20030616.json").
func methodNoExtension(filename string) string {
	ext := filepath.Ext(filename)
	return strings.TrimSuffix(filename, ext)
}

// parseJSONLookup reads and parses a JSON sidecar file, returning a
// JSONLookupResult. Returns nil if the file cannot be read or parsed.
func parseJSONLookup(jsonPath string) *JSONLookupResult {
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil
	}

	var gp GooglePhoto
	if err := json.Unmarshal(data, &gp); err != nil {
		return nil
	}

	result := &JSONLookupResult{
		JSONFile:     jsonPath,
		Lat:          gp.GeoData.Latitude,
		Lon:          gp.GeoData.Longitude,
		Alt:          gp.GeoData.Altitude,
		CameraMake:   gp.CameraMake,
		CameraModel:  gp.CameraModel,
		DeviceFolder: gp.GooglePhotosOrigin.MobileUpload.DeviceFolder.LocalFolderName,
		DeviceType:   gp.GooglePhotosOrigin.MobileUpload.DeviceType,
		GooglePhoto:  &gp,
	}

	// Try to extract timestamp — prefer filename, fallback to JSON
	// First, we need the photo filename. Since we don't have it here,
	// we extract from the JSON's title field if available, or just use JSON.
	// The caller (JSONForFile) will have the photo path; timestamp resolution
	// is deferred to the caller who can pass the photo filename.
	if gp.PhotoTakenTime.Timestamp != "" {
		sec, err := strconv.ParseInt(gp.PhotoTakenTime.Timestamp, 10, 64)
		if err == nil {
			result.Timestamp = time.Unix(sec, 0).UTC()
		}
	}

	return result
}

// ResolveTimestamp extracts the photo taken time with a 3-tier priority:
//  1. EXIF DateTimeOriginal tag (via exiftool)
//  2. Parse timestamp from photo filename (via parser.ParseFilenameTimestamp)
//  3. Parse timestamp from JSON photoTakenTime.timestamp field
//  4. Return zero time if all fail
func ResolveTimestamp(photoPath string, gp *GooglePhoto) time.Time {
	// Priority 1: EXIF DateTimeOriginal
	if t, ok := parser.ParseEXIFTimestamp(photoPath); ok {
		return t
	}

	// Priority 2: filename-based parsing
	if t, ok := parser.ParseFilenameTimestamp(filepath.Base(photoPath)); ok {
		return t
	}

	// Priority 3: JSON timestamp
	if gp.PhotoTakenTime.Timestamp != "" {
		sec, err := strconv.ParseInt(gp.PhotoTakenTime.Timestamp, 10, 64)
		if err == nil {
			return time.Unix(sec, 0).UTC()
		}
	}

	// Priority 4: zero time
	return time.Time{}
}

// ResolveGPS extracts GPS coordinates with a 2-tier priority:
//  1. EXIF GPS tags (via exiftool)
//  2. JSON geoData.latitude/longitude/altitude fields
//  3. Return zero GPSInfo if both fail
func ResolveGPS(photoPath string, gp *GooglePhoto) parser.GPSInfo {
	// Priority 1: EXIF GPS
	if info := parser.ParseEXIFGPS(photoPath); info.Has {
		return info
	}

	// Priority 2: JSON geoData
	if gp.GeoData.Latitude != 0 || gp.GeoData.Longitude != 0 {
		return parser.GPSInfo{
			Lat:  gp.GeoData.Latitude,
			Lon:  gp.GeoData.Longitude,
			Alt:  gp.GeoData.Altitude,
			Has:  true,
		}
	}

	// Priority 3: no GPS
	return parser.GPSInfo{}
}
// If old is not found, returns s unchanged.
func replaceLast(s, old, new string) string {
	i := strings.LastIndex(s, old)
	if i == -1 {
		return s
	}
	return s[:i] + new + s[i+len(old):]
}

// nfcNormalize performs NFC Unicode normalization on a string.
// Go's standard library doesn't include Unicode normalization, so we use
// a simple approach: for most practical cases with Google Takeout filenames,
// the NFD/NFC difference is primarily in accented characters (like é).
// We handle the common cases inline to avoid an external dependency.
//
// For full NFC normalization, the golang.org/x/text/unicode/norm package
// would be needed. This simplified version handles the most common cases
// seen in Google Takeout filenames.
func nfcNormalize(s string) string {
	// If the string is pure ASCII, no normalization needed.
	isASCII := true
	for i := 0; i < len(s); i++ {
		if s[i] > 127 {
			isASCII = false
			break
		}
	}
	if isASCII {
		return s
	}

	// For non-ASCII strings, we need proper NFC normalization.
	// Common NFD→NFC compositions in Google Takeout filenames:
	// é = e + combining acute accent (U+0301) → é (U+00E9)
	// We handle the most common accented characters seen in extraFormats.

	// Convert to runes for easier manipulation
	runes := []rune(s)
	var result []rune

	i := 0
	for i < len(runes) {
		if i+1 < len(runes) && unicode.Is(unicode.Mn, runes[i+1]) {
			// Current rune + combining mark — try to compose
			composed := composePair(runes[i], runes[i+1])
			if composed != 0 {
				result = append(result, composed)
				i += 2
				continue
			}
		}
		result = append(result, runes[i])
		i++
	}

	return string(result)
}

// composePair attempts to compose a base character + combining mark into
// a single precomposed character. Returns 0 if no composition exists.
// This handles the common accented characters found in Google Takeout
// filenames from various languages.
func composePair(base, combining rune) rune {
	if combining != 0x0301 { // combining acute accent
		return 0
	}
	// Common compositions used in extraFormats suffixes
	switch base {
	case 'e':
		return '\u00E9' // é
	case 'E':
		return '\u00C9' // É
	case 'a':
		return '\u00E1' // á
	case 'A':
		return '\u00C1' // Á
	case 'i':
		return '\u00ED' // í
	case 'I':
		return '\u00CD' // Í
	case 'o':
		return '\u00F3' // ó
	case 'O':
		return '\u00D3' // Ó
	case 'u':
		return '\u00FA' // ú
	case 'U':
		return '\u00DA' // Ú
	case 'c':
		return '\u0107' // ć
	case 'C':
		return '\u0106' // Ć
	case 'n':
		return '\u0144' // ń
	case 'N':
		return '\u0143' // Ń
	case 's':
		return '\u015B' // ś
	case 'S':
		return '\u015A' // Ś
	case 'z':
		return '\u017A' // ź
	case 'Z':
		return '\u0179' // Ź
	case 'l':
		return '\u013A' // ĺ
	case 'L':
		return '\u0139' // Ĺ
	case 'r':
		return '\u0155' // ŕ
	case 'R':
		return '\u0154' // Ŕ
	}
	return 0
}
