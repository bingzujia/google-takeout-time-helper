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
	re    *regexp.Regexp
	parse func(m []string) (time.Time, bool)
}

var loc = time.UTC

var patterns = []pattern{
	// 1. IMG20250409084814 / VID20250409084814 (anchored, IMG/VID prefix)
	{
		re: regexp.MustCompile(`(?i)^(?:IMG|VID)(\d{8})(\d{6})`),
		parse: func(m []string) (time.Time, bool) {
			return parseDateTime8_6(m[1], m[2])
		},
	},
	// 2. IMG_20250727_141938 / VID_20250727_141938 (anchored, IMG/VID prefix)
	{
		re: regexp.MustCompile(`(?i)^(?:IMG|VID)_(\d{8})_(\d{6})`),
		parse: func(m []string) (time.Time, bool) {
			return parseDateTime8_6(m[1], m[2])
		},
	},
	// 3. (\d{8})_(\d{6}) — generic, anywhere in filename
	{
		re: regexp.MustCompile(`(\d{8})_(\d{6})`),
		parse: func(m []string) (time.Time, bool) {
			return parseDateTime8_6(m[1], m[2])
		},
	},
	// 4. (\d{8})(\d{6}) — 14 consecutive digits at filename start
	{
		re: regexp.MustCompile(`^(\d{8})(\d{6})`),
		parse: func(m []string) (time.Time, bool) {
			return parseDateTime8_6(m[1], m[2])
		},
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
	},
	// 6. (\d{8})_(\d{6})~\d+ — burst photos with ~N suffix
	{
		re: regexp.MustCompile(`(\d{8})_(\d{6})~\d+`),
		parse: func(m []string) (time.Time, bool) {
			return parseDateTime8_6(m[1], m[2])
		},
	},
	// 7. (\d{4})-(\d{2})-(\d{2})-(\d{2})-(\d{2})-(\d{2}) — YYYY-MM-DD-HH-mm-ss
	{
		re: regexp.MustCompile(`(\d{4})-(\d{2})-(\d{2})-(\d{2})-(\d{2})-(\d{2})`),
		parse: func(m []string) (time.Time, bool) {
			return parseComponents(m[1], m[2], m[3], m[4], m[5], m[6])
		},
	},
	// 8. (\d{8})-(\d{6}) — YYYYMMDD-HHmmss
	{
		re: regexp.MustCompile(`(\d{8})-(\d{6})`),
		parse: func(m []string) (time.Time, bool) {
			return parseDateTime8_6(m[1], m[2])
		},
	},
	// 9/10/11. mmexport<13-digit-unix-ms>[(-suffix)|((N))]
	{
		re: regexp.MustCompile(`(?i)mmexport(\d{13})(?:[(-].*)?`),
		parse: func(m []string) (time.Time, bool) {
			sec, err := strconv.ParseInt(m[1][:10], 10, 64)
			if err != nil {
				return time.Time{}, false
			}
			return time.Unix(sec, 0).UTC(), true
		},
	},
}

// ParseFilenameTimestamp returns the time embedded in the filename, or zero time if none found.
func ParseFilenameTimestamp(filename string) (time.Time, bool) {
	base := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	for _, p := range patterns {
		m := p.re.FindStringSubmatch(base)
		if m != nil {
			return p.parse(m)
		}
	}
	return time.Time{}, false
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
