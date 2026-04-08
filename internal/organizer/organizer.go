package organizer

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
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

var imageExts = setOf("jpg", "jpeg", "png", "gif", "bmp", "tiff", "tif", "heic", "heif", "webp", "avif", "raw", "cr2", "nef", "arw", "dng")
var videoExts = setOf("mp4", "mov", "avi", "mkv", "wmv", "flv", "3gp", "m4v", "webm", "mpg", "mpeg", "asf", "rm", "rmvb", "vob", "ts", "mts", "m2ts")

var cameraPrefixes = []string{"WP_", "IMG_", "IMG", "VID_", "VID", "P_", "PXL_", "DSC_"}
var cameraDatePattern = regexp.MustCompile(`^\d{8}_\d{6}`)

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
		if !imageExts[ext] && !videoExts[ext] {
			return false
		}
		for _, prefix := range cameraPrefixes {
			if strings.HasPrefix(name, prefix) {
				return true
			}
		}
		return cameraDatePattern.MatchString(base)
	case ModeScreenshot:
		if !imageExts[ext] {
			return false
		}
		return strings.Contains(lower, "screenshot")
	case ModeWechat:
		if !imageExts[ext] && !videoExts[ext] {
			return false
		}
		return strings.HasPrefix(lower, "mmexport")
	}
	return false
}

func moveFile(src, name string, cfg Config, result *Result) error {
	destPath := resolveDestPath(cfg.DestDir, name)

	if cfg.DryRun {
		result.Moved++
		return nil
	}

	if err := os.Rename(src, destPath); err != nil {
		// Try copy+delete for cross-device moves
		if err2 := copyFile(src, destPath); err2 != nil {
			result.Skipped++
			return nil
		}
		os.Remove(src)
	}
	result.Moved++
	return nil
}

func resolveDestPath(destDir, name string) string {
	target := filepath.Join(destDir, name)
	if _, err := os.Stat(target); os.IsNotExist(err) {
		return target
	}
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	suffix := time.Now().Format("20060102150405")
	return filepath.Join(destDir, fmt.Sprintf("%s_%s%s", stem, suffix, ext))
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, info.Mode())
}

func setOf(vals ...string) map[string]bool {
	m := make(map[string]bool, len(vals))
	for _, v := range vals {
		m[v] = true
	}
	return m
}
