package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/bingzujia/g_photo_take_out_helper/internal/migrator"
	"github.com/bingzujia/g_photo_take_out_helper/internal/parser"
	"github.com/bingzujia/g_photo_take_out_helper/internal/progress"
	"github.com/spf13/cobra"
)

var fixExifDatesCmd = &cobra.Command{
	Use:   "fix-exif-dates",
	Short: "Sync DateTimeOriginal → CreateDate & ModifyDate using exiftool",
	Long: `fix-exif-dates reads the DateTimeOriginal EXIF field from media files in the
given directory (first level only, non-recursive) and writes the same value to
CreateDate and ModifyDate, using exiftool.

Requires exiftool to be installed and available in PATH.`,
	RunE: runFixExifDates,
}

var fixExifDatesDir string
var fixExifDatesDryRun bool

// mediaExts is the whitelist of recognised media file extensions (lowercase).
var mediaExts = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".heic": true,
	".heif": true,
	".mp4":  true,
	".mov":  true,
	".avi":  true,
	".3gp":  true,
	".mkv":  true,
	".webp": true,
}

func init() {
	fixExifDatesCmd.Flags().StringVar(&fixExifDatesDir, "dir", ".", "target directory")
	fixExifDatesCmd.Flags().BoolVar(&fixExifDatesDryRun, "dry-run", false, "preview only, do not modify files")
	rootCmd.AddCommand(fixExifDatesCmd)
}

func runFixExifDates(_ *cobra.Command, _ []string) error {
	if _, err := exec.LookPath("exiftool"); err != nil {
		return fmt.Errorf("exiftool not found in PATH: install exiftool and retry")
	}

	entries, err := os.ReadDir(fixExifDatesDir)
	if err != nil {
		return fmt.Errorf("reading directory %q: %w", fixExifDatesDir, err)
	}

	var mediaFiles []string
	skipped := 0

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if !mediaExts[ext] {
			skipped++
			continue
		}
		mediaFiles = append(mediaFiles, filepath.Join(fixExifDatesDir, e.Name()))
	}

	if len(mediaFiles) == 0 {
		progress.Info("No media files found in %q", fixExifDatesDir)
		progress.Success("Done. Processed: 0, Failed: 0, Skipped: %d", skipped)
		return nil
	}

	// Dry-run: resolve timestamp per file and print it.
	if fixExifDatesDryRun {
		for _, f := range mediaFiles {
			t, src, ok := resolveTimestamp(f)
			if !ok {
				progress.Info("  %s  (no DateTimeOriginal and no filename timestamp)", f)
			} else if src == "filename" {
				progress.Info("  %s  DateTimeOriginal=%s (from filename)", f, t.Format("2006:01:02 15:04:05"))
			} else {
				progress.Info("  %s  DateTimeOriginal=%s", f, t.Format("2006:01:02 15:04:05"))
			}
		}
		progress.Success("Dry-run complete. Would process: %d, Skipped: %d", len(mediaFiles), skipped)
		return nil
	}

	// Open log file for failures.
	logPath := filepath.Join(fixExifDatesDir, "gtoh-fix-exif.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("opening log file %q: %w", logPath, err)
	}
	defer logFile.Close()

	writeLog := func(filePath, detail string) {
		ts := time.Now().Format("2006-01-02 15:04:05")
		fmt.Fprintf(logFile, "[%s] FAIL write DateTimeOriginal/FileModifyDate: %s (%s)\n", ts, filePath, detail)
	}

	processed, failed := 0, 0
	writer := migrator.ExifWriter{}

	for _, f := range mediaFiles {
		t, src, ok := resolveTimestamp(f)
		if !ok {
			failed++
			writeLog(f, "no DateTimeOriginal and no filename timestamp")
			progress.Error("FAIL %s: no DateTimeOriginal and no filename timestamp", filepath.Base(f))
			continue
		}
		if src == "filename" {
			writeLog(f, "no DateTimeOriginal; timestamp from filename")
		}
		if err := writer.WriteTimestamp(f, t); err != nil {
			failed++
			detail := err.Error()
			writeLog(f, detail)
			progress.Error("FAIL %s: %s", filepath.Base(f), detail)
			continue
		}
		processed++
	}

	if failed > 0 {
		progress.Success("Done. Processed: %d, Failed: %d, Skipped: %d", processed, failed, skipped)
		progress.Info("Log: %s", logPath)
	} else {
		progress.Success("Done. Processed: %d, Failed: %d, Skipped: %d", processed, failed, skipped)
	}
	return nil
}

// resolveTimestamp tries to obtain a timestamp for the given media file.
// It first checks the EXIF DateTimeOriginal field; if absent, it falls back to
// parsing the filename. The returned source is "exif" or "filename".
func resolveTimestamp(filePath string) (t time.Time, source string, ok bool) {
	if t, ok = parser.ParseEXIFTimestamp(filePath); ok {
		return t, "exif", true
	}
	if t, ok = parser.ParseFilenameTimestamp(filePath); ok {
		return t, "filename", true
	}
	return time.Time{}, "", false
}
