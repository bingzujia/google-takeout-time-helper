package organizer

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bingzujia/g_photo_take_out_helper/internal/fileutil"
	"github.com/bingzujia/g_photo_take_out_helper/internal/mediatype"
)

// Mode determines which files to organize.
type Mode string

const (
	ModeCamera     Mode = "camera"
	ModeScreenshot Mode = "screenshot"
	ModeWechat     Mode = "wechat"
)

// Config holds organizer settings.
type Config struct {
	Mode       Mode
	SourceDirs []string
	DestDir    string
	DryRun     bool
	Recursive  bool
}

// Result holds counts after a Run.
type Result struct {
	Moved   int
	Skipped int
}

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

// Run executes the organizer.
func Run(cfg Config) (Result, error) {
	if err := os.MkdirAll(cfg.DestDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("create dest dir: %w", err)
	}

	var result Result
	for _, srcDir := range cfg.SourceDirs {
		if err := walkDir(srcDir, cfg, &result); err != nil {
			return result, err
		}
	}
	return result, nil
}

func walkDir(dir string, cfg Config, result *Result) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		fullPath := filepath.Join(dir, e.Name())
		if e.IsDir() {
			if cfg.Recursive {
				if err := walkDir(fullPath, cfg, result); err != nil {
					return err
				}
			}
			continue
		}
		if matches(e.Name(), cfg.Mode) {
			if err := moveFile(fullPath, e.Name(), cfg, result); err != nil {
				return err
			}
		}
	}
	return nil
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

func moveFile(src, name string, cfg Config, result *Result) error {
	destPath := fileutil.ResolveDestPath(cfg.DestDir, name)

	if cfg.DryRun {
		result.Moved++
		return nil
	}

	if err := os.Rename(src, destPath); err != nil {
		// Try copy+delete for cross-device moves
		if err2 := fileutil.CopyFile(src, destPath); err2 != nil {
			result.Skipped++
			return nil
		}
		os.Remove(src)
	}
	result.Moved++
	return nil
}

