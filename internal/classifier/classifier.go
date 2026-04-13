package classifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/bingzujia/g_photo_take_out_helper/internal/organizer"
)

// Category is the destination bucket for a classified file.
type Category string

const (
	CategoryCamera      Category = "camera"
	CategoryScreenshot  Category = "screenshot"
	CategoryWechat      Category = "wechat"
	CategorySeemsCamera Category = "seemsCamera"
)

// Config holds settings for a classify run.
type Config struct {
	InputDir  string
	OutputDir string
	DryRun    bool
}

// Result holds counts after a Run.
type Result struct {
	Camera      int
	Screenshot  int
	Wechat      int
	SeemsCamera int
	Skipped     int
}

// Run classifies media files from first-level subdirectories of cfg.InputDir
// and moves them into category subdirectories under cfg.OutputDir.
func Run(cfg Config) (Result, error) {
	var result Result

	subdirs, err := scanFirstLevel(cfg.InputDir)
	if err != nil {
		return result, fmt.Errorf("scan input dir: %w", err)
	}

	for _, subdir := range subdirs {
		entries, err := os.ReadDir(subdir)
		if err != nil {
			return result, fmt.Errorf("read dir %s: %w", subdir, err)
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			filePath := filepath.Join(subdir, e.Name())
			cat, ok := classifyFile(e.Name())
			if !ok {
				// Fallback: inspect EXIF for camera device info.
				hasCam, _ := exiftoolFallback(filePath)
				if hasCam {
					cat = CategorySeemsCamera
				} else {
					result.Skipped++
					continue
				}
			}
			if err := moveToCategory(filePath, e.Name(), cfg.OutputDir, cat, cfg.DryRun, &result); err != nil {
				return result, err
			}
		}
	}
	return result, nil
}

// scanFirstLevel returns paths of immediate subdirectories of inputDir.
func scanFirstLevel(inputDir string) ([]string, error) {
	entries, err := os.ReadDir(inputDir)
	if err != nil {
		return nil, err
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, filepath.Join(inputDir, e.Name()))
		}
	}
	return dirs, nil
}

// classifyFile maps organizer filename rules to a Category.
func classifyFile(name string) (Category, bool) {
	mode, ok := organizer.Classify(name)
	if !ok {
		return "", false
	}
	switch mode {
	case organizer.ModeCamera:
		return CategoryCamera, true
	case organizer.ModeScreenshot:
		return CategoryScreenshot, true
	case organizer.ModeWechat:
		return CategoryWechat, true
	default:
		return "", false
	}
}

// exifDeviceOutput mirrors the exiftool JSON output for Make/Model tags.
type exifDeviceOutput struct {
	Make  string `json:"Make"`
	Model string `json:"Model"`
}

// exiftoolFallback returns true if the file's EXIF Make or Model tag is non-empty.
// Returns (false, nil) gracefully when exiftool is not installed or the command fails.
func exiftoolFallback(path string) (bool, error) {
	if _, err := exec.LookPath("exiftool"); err != nil {
		fmt.Fprintf(os.Stderr, "warning: exiftool not found, skipping EXIF fallback for %s\n", path)
		return false, nil
	}

	cmd := exec.Command("exiftool", "-Make", "-Model", "-j", path)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return false, nil
	}

	var results []exifDeviceOutput
	if err := json.Unmarshal(stdout.Bytes(), &results); err != nil || len(results) == 0 {
		return false, nil
	}
	r := results[0]
	return strings.TrimSpace(r.Make) != "" || strings.TrimSpace(r.Model) != "", nil
}

// moveToCategory moves src into <outputDir>/<category>/, respecting dry-run mode.
func moveToCategory(src, name, outputDir string, cat Category, dryRun bool, result *Result) error {
	destDir := filepath.Join(outputDir, string(cat))

	if dryRun {
		fmt.Printf("  [dry-run] %s  →  %s/%s\n", src, string(cat), name)
		incrementResult(result, cat)
		return nil
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("create dest dir %s: %w", destDir, err)
	}

	destPath := resolveDestPath(destDir, name)
	if err := os.Rename(src, destPath); err != nil {
		// Try copy+delete for cross-device moves.
		if err2 := copyFile(src, destPath); err2 != nil {
			result.Skipped++
			return nil
		}
		os.Remove(src)
	}
	incrementResult(result, cat)
	return nil
}

func incrementResult(r *Result, cat Category) {
	switch cat {
	case CategoryCamera:
		r.Camera++
	case CategoryScreenshot:
		r.Screenshot++
	case CategoryWechat:
		r.Wechat++
	case CategorySeemsCamera:
		r.SeemsCamera++
	}
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
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	if err = out.Close(); err != nil {
		return err
	}
	return os.Chtimes(dst, info.ModTime(), info.ModTime())
}
