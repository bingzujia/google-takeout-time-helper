package renamer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Config holds renamer settings.
type Config struct {
	Dir    string
	DryRun bool
}

// Result holds counts after a Run.
type Result struct {
	Renamed int
	Skipped int
	Errors  int
}

var imageExts = setOf("jpg", "jpeg", "png", "gif", "bmp", "tiff", "tif", "heic", "heif", "webp", "avif", "raw", "cr2", "nef", "arw", "dng")
var videoExts = setOf("mp4", "mov", "avi", "mkv", "wmv", "flv", "3gp", "m4v", "webm", "mpg", "mpeg", "asf", "rm", "rmvb", "vob", "ts", "mts", "m2ts")

// Run renames media files in Dir based on their mtime.
func Run(cfg Config) (Result, error) {
	entries, err := os.ReadDir(cfg.Dir)
	if err != nil {
		return Result{}, fmt.Errorf("read dir: %w", err)
	}

	var result Result
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
		if !imageExts[ext] && !videoExts[ext] {
			continue
		}

		fullPath := filepath.Join(cfg.Dir, name)
		info, err := e.Info()
		if err != nil {
			result.Errors++
			continue
		}

		mtime := info.ModTime()
		prefix := "IMG"
		if videoExts[ext] {
			prefix = "VID"
		}

		newName := generateName(cfg.Dir, prefix, mtime, "."+ext, name)
		if newName == name {
			result.Skipped++
			continue
		}

		if cfg.DryRun {
			fmt.Printf("  %s -> %s\n", name, newName)
			result.Renamed++
			continue
		}

		if err := os.Rename(fullPath, filepath.Join(cfg.Dir, newName)); err != nil {
			result.Errors++
			continue
		}
		result.Renamed++
	}
	return result, nil
}

// generateName picks a non-conflicting name for the file.
func generateName(dir, prefix string, t time.Time, ext, currentName string) string {
	for i := 0; i < 999; i++ {
		candidate := fmt.Sprintf("%s%s%s", prefix, t.Add(time.Duration(i)*time.Second).Format("20060102150405"), ext)
		if candidate == currentName {
			return currentName
		}
		if _, err := os.Stat(filepath.Join(dir, candidate)); os.IsNotExist(err) {
			return candidate
		}
	}
	return currentName
}

func setOf(vals ...string) map[string]bool {
	m := make(map[string]bool, len(vals))
	for _, v := range vals {
		m[v] = true
	}
	return m
}
