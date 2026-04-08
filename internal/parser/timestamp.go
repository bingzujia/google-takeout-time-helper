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
	re   *regexp.Regexp
	parse func(m []string) (time.Time, bool)
}

var loc = time.UTC

var patterns = []pattern{
	// 1. IMG_20230302_112040  (8-digit date _ 6-digit time)
	{
		re: regexp.MustCompile(`(?i)(?:IMG|VID|WP|P|PXL|DSC|MVIMG|PANO|BURST)_(\d{8})_(\d{6})`),
		parse: func(m []string) (time.Time, bool) {
			return parseDateTime8_6(m[1], m[2])
		},
	},
	// 2. IMG20230123102606 (8-digit date concat 6-digit time, no separator)
	{
		re: regexp.MustCompile(`(?i)(?:IMG|VID|MVIMG|PANO|BURST)(\d{8})(\d{6})`),
		parse: func(m []string) (time.Time, bool) {
			return parseDateTime8_6(m[1], m[2])
		},
	},
	// 3. WP_20131010_074  (8-digit date _ 3-6 digit time, zero-pad to 6)
	{
		re: regexp.MustCompile(`(?i)WP_(\d{8})_(\d{3,6})`),
		parse: func(m []string) (time.Time, bool) {
			timeStr := m[2]
			for len(timeStr) < 6 {
				timeStr += "0"
			}
			return parseDateTime8_6(m[1], timeStr)
		},
	},
	// 6. Screenshot_2016-02-28-13-06-34 (YYYY-MM-DD-HH-mm-ss)
	{
		re: regexp.MustCompile(`(?i)Screenshot_(\d{4})-(\d{2})-(\d{2})-(\d{2})-(\d{2})-(\d{2})`),
		parse: func(m []string) (time.Time, bool) {
			return parseComponents(m[1], m[2], m[3], m[4], m[5], m[6])
		},
	},
	// 7. Screenshot_20210803-084525 (YYYYMMDD-HHmmss)
	{
		re: regexp.MustCompile(`(?i)Screenshot_(\d{8})-(\d{6})`),
		parse: func(m []string) (time.Time, bool) {
			return parseDateTime8_6(m[1], m[2])
		},
	},
	// 8/9/10. mmexport<13-digit-unix-ms>[(-suffix)|((N))]
	{
		re: regexp.MustCompile(`(?i)mmexport(\d{10})\d{0,3}(?:[(-].*)?`),
		parse: func(m []string) (time.Time, bool) {
			sec, err := strconv.ParseInt(m[1], 10, 64)
			if err != nil {
				return time.Time{}, false
			}
			return time.Unix(sec, 0).UTC(), true
		},
	},
	// 5. 20151120_120004~2  (must come before pattern 4)
	{
		re: regexp.MustCompile(`^(\d{8})_(\d{6})~\d+`),
		parse: func(m []string) (time.Time, bool) {
			return parseDateTime8_6(m[1], m[2])
		},
	},
	// 4. 20151120_120004
	{
		re: regexp.MustCompile(`^(\d{8})_(\d{6})`),
		parse: func(m []string) (time.Time, bool) {
			return parseDateTime8_6(m[1], m[2])
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
	t := time.Date(vals[0], time.Month(vals[1]), vals[2], vals[3], vals[4], vals[5], 0, loc)
	return t, true
}
