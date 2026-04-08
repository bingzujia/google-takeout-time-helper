package parser

import (
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var imgVIDPatterns = []struct {
	re    *regexp.Regexp
	parse func(m []string) (time.Time, bool)
}{
	// IMG_20250727_141938 or VID_20250727_141938
	{
		re: regexp.MustCompile(`(?i)^(?:IMG|VID)_(\d{8})_(\d{6})`),
		parse: func(m []string) (time.Time, bool) {
			return parseDateTime8_6(m[1], m[2])
		},
	},
	// IMG20250409084814 or VID20250409084814
	{
		re: regexp.MustCompile(`(?i)^(?:IMG|VID)(\d{8})(\d{6})`),
		parse: func(m []string) (time.Time, bool) {
			return parseDateTime8_6(m[1], m[2])
		},
	},
}

// ParseIMGVIDFilename parses timestamps from IMG/VID prefixed filenames.
func ParseIMGVIDFilename(filename string) (time.Time, bool) {
	base := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	for _, p := range imgVIDPatterns {
		m := p.re.FindStringSubmatch(base)
		if m != nil {
			return p.parse(m)
		}
	}
	return time.Time{}, false
}
