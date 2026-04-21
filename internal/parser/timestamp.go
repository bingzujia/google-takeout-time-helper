package parser

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// pattern groups all timestamp-parsing rules in priority order.
type pattern struct {
	re           *regexp.Regexp
	parse        func(m []string) (time.Time, bool)
	isUnixFormat bool // true if pattern uses Unix timestamp (Pattern 9, 13), false for explicit datetime
}

var loc = time.UTC

// lastMatchedPattern tracks which pattern was matched for the current ParseFilenameTimestamp call.
// This is used by isUnixTimestampFormat to determine the timestamp type.
var lastMatchedPattern *pattern

var patterns = []pattern{
	// 1. IMG20250409084814 / VID20250409084814 (anchored, IMG/VID prefix)
	{
		re: regexp.MustCompile(`(?i)^(?:IMG|VID)(\d{8})(\d{6})`),
		parse: func(m []string) (time.Time, bool) {
			return parseDateTime8_6(m[1], m[2])
		},
		isUnixFormat: false,
	},
	// 2. IMG_20250727_141938 / VID_20250727_141938 (anchored, IMG/VID prefix)
	{
		re: regexp.MustCompile(`(?i)^(?:IMG|VID)_(\d{8})_(\d{6})`),
		parse: func(m []string) (time.Time, bool) {
			return parseDateTime8_6(m[1], m[2])
		},
		isUnixFormat: false,
	},
	// 3. (\d{8})_(\d{6}) — generic, anywhere in filename
	{
		re: regexp.MustCompile(`(\d{8})_(\d{6})`),
		parse: func(m []string) (time.Time, bool) {
			return parseDateTime8_6(m[1], m[2])
		},
		isUnixFormat: false,
	},
	// 4. (\d{8})(\d{6}) — 14 consecutive digits at filename start
	{
		re: regexp.MustCompile(`^(\d{8})(\d{6})`),
		parse: func(m []string) (time.Time, bool) {
			return parseDateTime8_6(m[1], m[2])
		},
		isUnixFormat: false,
	},
	// 5. (\d{8})_(\d{3,6}) — WP short time, default to 12:00:00 when < 6 digits
	{
		re: regexp.MustCompile(`(\d{8})_(\d{3,6})`),
		parse: func(m []string) (time.Time, bool) {
			timeStr := m[2]
			if len(timeStr) < 6 {
				// Time part is incomplete, default to 12:00:00
				return parseDateTime8_6(m[1], "120000")
			}
			return parseDateTime8_6(m[1], timeStr)
		},
		isUnixFormat: false,
	},
	// 6. (\d{8})_(\d{6})~\d+ — burst photos with ~N suffix
	{
		re: regexp.MustCompile(`(\d{8})_(\d{6})~\d+`),
		parse: func(m []string) (time.Time, bool) {
			return parseDateTime8_6(m[1], m[2])
		},
		isUnixFormat: false,
	},
	// 7. (\d{4})-(\d{2})-(\d{2})-(\d{2})-(\d{2})-(\d{2}) — YYYY-MM-DD-HH-mm-ss
	{
		re: regexp.MustCompile(`(\d{4})-(\d{2})-(\d{2})-(\d{2})-(\d{2})-(\d{2})`),
		parse: func(m []string) (time.Time, bool) {
			return parseComponents(m[1], m[2], m[3], m[4], m[5], m[6])
		},
		isUnixFormat: false,
	},
	// 8. (\d{8})-(\d{6}) — YYYYMMDD-HHmmss
	{
		re: regexp.MustCompile(`(\d{8})-(\d{6})`),
		parse: func(m []string) (time.Time, bool) {
			return parseDateTime8_6(m[1], m[2])
		},
		isUnixFormat: false,
	},
	// 9/10/11. mmexport<13-digit-unix-ms>[(-suffix)|((N))]
	{
		re: regexp.MustCompile(`(?i)mmexport(\d{13})(?:[(-].*)?`),
		parse: func(m []string) (time.Time, bool) {
			return parseUnixSeconds(m[1][:10])
		},
		isUnixFormat: true,
	},
	// 12. TIM图片YYYYMMDDHHMMSS
	{
		re: regexp.MustCompile(`(?i)^TIM图片(\d{8})(\d{6})$`),
		parse: func(m []string) (time.Time, bool) {
			return parseDateTime8_6(m[1], m[2])
		},
		isUnixFormat: false,
	},
	// 13. album_temp__..._<unix-seconds>
	{
		re: regexp.MustCompile(`(?i)^album_temp__.*_(\d{10})$`),
		parse: func(m []string) (time.Time, bool) {
			return parseUnixSeconds(m[1])
		},
		isUnixFormat: true,
	},
}

// ParseFilenameTimestamp returns the time embedded in the filename, or zero time if none found.
// It also updates lastMatchedPattern to track which pattern was matched (used by isUnixTimestampFormat).
func ParseFilenameTimestamp(filename string) (time.Time, bool) {
	base := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	for i := range patterns {
		p := &patterns[i]
		m := p.re.FindStringSubmatch(base)
		if m != nil {
			lastMatchedPattern = p
			return p.parse(m)
		}
	}
	lastMatchedPattern = nil
	return time.Time{}, false
}

// isUnixTimestampFormat returns true if the most recently parsed filename timestamp
// came from a Unix timestamp format (Pattern 9/13: mmexport or album_temp).
// Must be called immediately after ParseFilenameTimestamp to get accurate results.
func isUnixTimestampFormat(t time.Time) bool {
	if lastMatchedPattern == nil {
		return false
	}
	return lastMatchedPattern.isUnixFormat
}

func parseUnixSeconds(secStr string) (time.Time, bool) {
	sec, err := strconv.ParseInt(secStr, 10, 64)
	if err != nil {
		return time.Time{}, false
	}

	t := time.Unix(sec, 0).UTC()
	if t.Year() < 1980 || t.Year() > 2100 {
		return time.Time{}, false
	}
	return t, true
}

func parseDateTime8_6(date, timeStr string) (time.Time, bool) {
	if len(date) != 8 || len(timeStr) != 6 {
		return time.Time{}, false
	}
	return parseComponents(date[0:4], date[4:6], date[6:8], timeStr[0:2], timeStr[2:4], timeStr[4:6])
}

func parseComponents(year, month, day, hour, min, sec string) (time.Time, bool) {
	vals := make([]int, 6)
	for i, s := range []string{year, month, day, hour, min, sec} {
		v, err := strconv.Atoi(s)
		if err != nil {
			return time.Time{}, false
		}
		vals[i] = v
	}

	// Reject obviously invalid dates before Go normalizes them.
	// Go's time.Date(2025, 13, 45, ...) silently rolls over to a valid date,
	// which produces wrong results for false-positive regex matches.
	if vals[0] < 1980 || vals[0] > 2100 {
		return time.Time{}, false
	}
	if vals[1] < 1 || vals[1] > 12 {
		return time.Time{}, false
	}
	if vals[2] < 1 || vals[2] > 31 {
		return time.Time{}, false
	}
	if vals[3] > 23 || vals[4] > 59 || vals[5] > 59 {
		return time.Time{}, false
	}

	t := time.Date(vals[0], time.Month(vals[1]), vals[2], vals[3], vals[4], vals[5], 0, loc)
	// Double-check: Go may still normalize (e.g. Feb 30 → Mar 2).
	// If the components changed, the date was invalid.
	if t.Year() != vals[0] || int(t.Month()) != vals[1] || t.Day() != vals[2] ||
		t.Hour() != vals[3] || t.Minute() != vals[4] || t.Second() != vals[5] {
		return time.Time{}, false
	}
	return t, true
}
