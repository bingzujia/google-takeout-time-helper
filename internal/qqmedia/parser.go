package qqmedia

import (
	"os"
	"regexp"
	"strconv"
	"time"
)

// ParseTimestamp extracts Unix timestamp (milliseconds) from QQ media filenames
// Returns 0 if no pattern matches (triggers mtime fallback)
func ParseTimestamp(filename string) (int64, error) {
	// Pattern 1: _YYYYMMDD_HHMMSS (most specific, try first)
	if ts, ok := parsePattern1(filename); ok {
		return ts, nil
	}

	// Pattern 3: QQ视频YYYYMMDDHHMMSS or qq_video_YYYYMMDDHHMMSS
	if ts, ok := parsePattern3(filename); ok {
		return ts, nil
	}

	// Pattern 4: Record_YYYY-MM-DD-HH-MM-SS
	if ts, ok := parsePattern4(filename); ok {
		return ts, nil
	}

	// Pattern 5: Snipaste_YYYY-MM-DD_HH-MM-SS
	if ts, ok := parsePattern5(filename); ok {
		return ts, nil
	}

	// Pattern 7: TIM图片YYYYMMDDHHMMSS or TIM.*YYYYMMDDHHMMSS
	if ts, ok := parsePattern7(filename); ok {
		return ts, nil
	}

	// Pattern 6: tb_image_share_YYYYMMDDHHMMSS (13 digits)
	if ts, ok := parsePattern6(filename); ok {
		return ts, nil
	}

	// Pattern 2: 13-digit Unix milliseconds (least specific, try last)
	if ts, ok := parsePattern2(filename); ok {
		return ts, nil
	}

	// No pattern matched, return 0 (trigger mtime fallback)
	return 0, nil
}

// GetFileModTime returns file modification time as Unix milliseconds
func GetFileModTime(filepath string) (int64, error) {
	stat, err := os.Stat(filepath)
	if err != nil {
		return 0, err
	}
	return stat.ModTime().UnixMilli(), nil
}

// parsePattern1: _YYYYMMDD_HHMMSS
func parsePattern1(filename string) (int64, bool) {
	re := regexp.MustCompile(`_(\d{8})_(\d{6})`)
	matches := re.FindStringSubmatch(filename)
	if len(matches) != 3 {
		return 0, false
	}

	dateStr := matches[1] // YYYYMMDD
	timeStr := matches[2] // HHMMSS

	return parseYYYYMMDDhhmmss(dateStr, timeStr)
}

// parsePattern2: 13-digit Unix milliseconds
func parsePattern2(filename string) (int64, bool) {
	re := regexp.MustCompile(`(\d{13})`)
	matches := re.FindStringSubmatch(filename)
	if len(matches) != 2 {
		return 0, false
	}

	ts, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil || ts < 0 || ts > 9999999999999 {
		return 0, false
	}
	return ts, true
}

// parsePattern3: QQ视频YYYYMMDDHHMMSS or qq_video_YYYYMMDDHHMMSS
func parsePattern3(filename string) (int64, bool) {
	re := regexp.MustCompile(`(?:QQ视频|qq_video_?)(\d{8})(\d{6})`)
	matches := re.FindStringSubmatch(filename)
	if len(matches) != 3 {
		return 0, false
	}

	dateStr := matches[1]
	timeStr := matches[2]

	return parseYYYYMMDDhhmmss(dateStr, timeStr)
}

// parsePattern4: Record_YYYY-MM-DD-HH-MM-SS
func parsePattern4(filename string) (int64, bool) {
	re := regexp.MustCompile(`Record_(\d{4})-(\d{2})-(\d{2})-(\d{2})-(\d{2})-(\d{2})`)
	matches := re.FindStringSubmatch(filename)
	if len(matches) != 7 {
		return 0, false
	}

	year, _ := strconv.Atoi(matches[1])
	month, _ := strconv.Atoi(matches[2])
	day, _ := strconv.Atoi(matches[3])
	hour, _ := strconv.Atoi(matches[4])
	min, _ := strconv.Atoi(matches[5])
	sec, _ := strconv.Atoi(matches[6])

	t := time.Date(year, time.Month(month), day, hour, min, sec, 0, time.UTC)
	return t.UnixMilli(), true
}

// parsePattern5: Snipaste_YYYY-MM-DD_HH-MM-SS
func parsePattern5(filename string) (int64, bool) {
	re := regexp.MustCompile(`Snipaste_(\d{4})-(\d{2})-(\d{2})_(\d{2})-(\d{2})-(\d{2})`)
	matches := re.FindStringSubmatch(filename)
	if len(matches) != 7 {
		return 0, false
	}

	year, _ := strconv.Atoi(matches[1])
	month, _ := strconv.Atoi(matches[2])
	day, _ := strconv.Atoi(matches[3])
	hour, _ := strconv.Atoi(matches[4])
	min, _ := strconv.Atoi(matches[5])
	sec, _ := strconv.Atoi(matches[6])

	t := time.Date(year, time.Month(month), day, hour, min, sec, 0, time.UTC)
	return t.UnixMilli(), true
}

// parsePattern6: tb_image_share_YYYYMMDDHHMMSS (13 digits)
func parsePattern6(filename string) (int64, bool) {
	re := regexp.MustCompile(`tb_image_share_(\d{13})`)
	matches := re.FindStringSubmatch(filename)
	if len(matches) != 2 {
		return 0, false
	}

	ts, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil || ts < 0 || ts > 9999999999999 {
		return 0, false
	}
	return ts, true
}

// parsePattern7: TIM图片YYYYMMDDHHMMSS or TIM.*YYYYMMDDHHMMSS
func parsePattern7(filename string) (int64, bool) {
	re := regexp.MustCompile(`TIM.*?(\d{8})(\d{6})`)
	matches := re.FindStringSubmatch(filename)
	if len(matches) != 3 {
		return 0, false
	}

	dateStr := matches[1]
	timeStr := matches[2]

	return parseYYYYMMDDhhmmss(dateStr, timeStr)
}

// parseYYYYMMDDhhmmss is a helper to parse YYYYMMDD and HHMMSS strings to Unix ms
func parseYYYYMMDDhhmmss(dateStr, timeStr string) (int64, bool) {
	if len(dateStr) != 8 || len(timeStr) != 6 {
		return 0, false
	}

	year, _ := strconv.Atoi(dateStr[0:4])
	month, _ := strconv.Atoi(dateStr[4:6])
	day, _ := strconv.Atoi(dateStr[6:8])
	hour, _ := strconv.Atoi(timeStr[0:2])
	min, _ := strconv.Atoi(timeStr[2:4])
	sec, _ := strconv.Atoi(timeStr[4:6])

	// Validate ranges
	if month < 1 || month > 12 || day < 1 || day > 31 || hour < 0 || hour > 23 || min < 0 || min > 59 || sec < 0 || sec > 59 {
		return 0, false
	}

	t := time.Date(year, time.Month(month), day, hour, min, sec, 0, time.UTC)
	return t.UnixMilli(), true
}
