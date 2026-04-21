package screenshotter

import (
	"os"
	"regexp"
	"strconv"
	"time"
)

var (
	// Format 1: YYYY-MM-DD-HH-MM-SS-MS
	screenshotTimestampRe1 = regexp.MustCompile(`(\d{4})-(\d{2})-(\d{2})-(\d{2})-(\d{2})-(\d{2})-(\d{2})(?:\.|$)`)

	// Format 2: YYYYMMDD_HHMMSS
	screenshotTimestampRe2 = regexp.MustCompile(`(\d{8})_(\d{6})(?:\.|$)`)

	// Format 3: YYYY-MM-DD_HH-MM-SS
	screenshotTimestampRe3 = regexp.MustCompile(`(\d{4})-(\d{2})-(\d{2})_(\d{2})-(\d{2})-(\d{2})(?:\.|$)`)

	// Format 4: YYYY_M_D_H_M_S (auto zero-padding)
	screenshotTimestampRe4 = regexp.MustCompile(`(\d{4})_(\d{1,2})_(\d{1,2})_(\d{1,2})_(\d{1,2})_(\d{1,2})(?:\.|$)`)

	// Format 5: YYYY-MM-DD
	screenshotTimestampRe5 = regexp.MustCompile(`(\d{4})-(\d{2})-(\d{2})(?:\.|$)`)

	// Format 6b: {13-digit-milliseconds}
	screenshotTimestampRe6b = regexp.MustCompile(`(\d{13})(?:\.|$)`)

	// Format 6a: {10-digit-seconds}
	screenshotTimestampRe6a = regexp.MustCompile(`(\d{10})(?:\.|$)`)
)

// ParseScreenshotTimestamp extracts timestamp from screenshot filename
// Returns (time.Time, success bool, source string) where source is for reference only
// This is the public version that returns three values
func ParseScreenshotTimestamp(filename string) (time.Time, bool, string) {
	// Try Format 1: YYYY-MM-DD-HH-MM-SS-MS
	if m := screenshotTimestampRe1.FindStringSubmatch(filename); m != nil {
		year := atoi(m[1])
		month := atoi(m[2])
		day := atoi(m[3])
		hour := atoi(m[4])
		min := atoi(m[5])
		sec := atoi(m[6])
		ms := atoi(m[7])
		t := time.Date(year, time.Month(month), day, hour, min, sec, ms*10_000_000, time.UTC)
		return t, true, ""
	}

	// Try Format 2: YYYYMMDD_HHMMSS
	if m := screenshotTimestampRe2.FindStringSubmatch(filename); m != nil {
		dateStr := m[1]
		timeStr := m[2]
		year := atoi(dateStr[0:4])
		month := atoi(dateStr[4:6])
		day := atoi(dateStr[6:8])
		hour := atoi(timeStr[0:2])
		min := atoi(timeStr[2:4])
		sec := atoi(timeStr[4:6])
		t := time.Date(year, time.Month(month), day, hour, min, sec, 0, time.UTC)
		return t, true, ""
	}

	// Try Format 3: YYYY-MM-DD_HH-MM-SS
	if m := screenshotTimestampRe3.FindStringSubmatch(filename); m != nil {
		year := atoi(m[1])
		month := atoi(m[2])
		day := atoi(m[3])
		hour := atoi(m[4])
		min := atoi(m[5])
		sec := atoi(m[6])
		t := time.Date(year, time.Month(month), day, hour, min, sec, 0, time.UTC)
		return t, true, ""
	}

	// Try Format 4: YYYY_M_D_H_M_S (auto zero-padding)
	if m := screenshotTimestampRe4.FindStringSubmatch(filename); m != nil {
		year := atoi(m[1])
		month := atoi(m[2])
		day := atoi(m[3])
		hour := atoi(m[4])
		min := atoi(m[5])
		sec := atoi(m[6])
		t := time.Date(year, time.Month(month), day, hour, min, sec, 0, time.UTC)
		return t, true, ""
	}

	// Try Format 5: YYYY-MM-DD (date-only)
	if m := screenshotTimestampRe5.FindStringSubmatch(filename); m != nil {
		year := atoi(m[1])
		month := atoi(m[2])
		day := atoi(m[3])
		t := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
		return t, true, ""
	}

	// Try Format 6b: {13-digit-milliseconds}
	if m := screenshotTimestampRe6b.FindStringSubmatch(filename); m != nil {
		timestampMs := atoi64(m[1])
		unixSec := timestampMs / 1000
		nanoFrac := (timestampMs % 1000) * 1_000_000
		t := time.Unix(unixSec, nanoFrac).UTC()
		return t, true, ""
	}

	// Try Format 6a: {10-digit-seconds}
	if m := screenshotTimestampRe6a.FindStringSubmatch(filename); m != nil {
		timestampSec := atoi64(m[1])
		t := time.Unix(timestampSec, 0).UTC()
		return t, true, ""
	}

	return time.Time{}, false, ""
}

// parseScreenshotTimestamp is the legacy private version (kept for backward compatibility)
// Returns (time.Time, bool) if successful, (zero, false) otherwise
func parseScreenshotTimestamp(filename string) (time.Time, bool) {
	t, ok, _ := ParseScreenshotTimestamp(filename)
	return t, ok
}

// Helper functions for parsing
func atoi(s string) int {
	v, _ := strconv.Atoi(s)
	return v
}

func atoi64(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

// GetFileModTime retrieves the modification time of a file
// Returns (time.Time, true) if successful, (zero, false) otherwise
func GetFileModTime(filepath string) (time.Time, bool) {
	info, err := os.Stat(filepath)
	if err != nil {
		return time.Time{}, false
	}
	return info.ModTime().UTC(), true
}

// ResolveTimestamp attempts to get a timestamp from a file using priority logic
// Priority: filename parsing → modtime → fail
// Returns (time.Time, success bool, source string)
// source is "filename" or "modtime" or "" (if both failed)
func ResolveTimestamp(filepath, filename string) (time.Time, bool, string) {
	// Priority 1: Try parsing from filename
	if t, ok, _ := ParseScreenshotTimestamp(filename); ok {
		return t, true, "filename"
	}

	// Priority 2: Fall back to file modtime
	if t, ok := GetFileModTime(filepath); ok {
		return t, true, "modtime"
	}

	// Both failed
	return time.Time{}, false, ""
}
