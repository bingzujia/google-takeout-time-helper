package organizer

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bingzujia/google-takeout-time-helper/internal/mediatype"
)

// Mode determines which files to organize.
type Mode string

const (
	ModeCamera     Mode = "camera"
	ModeScreenshot Mode = "screenshot"
	ModeWechat     Mode = "wechat"
)

var cameraPrefixes = []string{"WP_", "IMG_", "IMG", "VID_", "VID", "P_", "PXL_", "DSC_"}
var cameraDatePattern = regexp.MustCompile(`^\d{8}_\d{6}`)

// Classify returns the Mode that best matches name, and true if any mode
// matched. Returns ("", false) when the file does not match any known pattern.
func Classify(name string) (Mode, bool) {
	for _, mode := range []Mode{ModeWechat, ModeScreenshot, ModeCamera} {
		if matches(name, mode) {
			return mode, true
		}
	}
	return "", false
}

func matches(name string, mode Mode) bool {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
	lower := strings.ToLower(name)
	base := strings.ToLower(strings.TrimSuffix(name, filepath.Ext(name)))

	switch mode {
	case ModeCamera:
		if !mediatype.IsImage(ext) && !mediatype.IsVideo(ext) {
			return false
		}
		for _, prefix := range cameraPrefixes {
			if strings.HasPrefix(name, prefix) {
				return true
			}
		}
		return cameraDatePattern.MatchString(base)
	case ModeScreenshot:
		if !mediatype.IsImage(ext) {
			return false
		}
		return strings.Contains(lower, "screenshot")
	case ModeWechat:
		if !mediatype.IsImage(ext) && !mediatype.IsVideo(ext) {
			return false
		}
		return strings.HasPrefix(lower, "mmexport")
	}
	return false
}
