package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
	// Task 2.1: check exiftool in PATH
	exiftoolPath, err := exec.LookPath("exiftool")
	if err != nil {
		return fmt.Errorf("exiftool not found in PATH: install exiftool and retry")
	}

	// Task 2.2: read first-level directory entries, skip subdirs
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
		// Task 2.3: filter by media extension whitelist
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if !mediaExts[ext] {
			skipped++
			continue
		}
		mediaFiles = append(mediaFiles, filepath.Join(fixExifDatesDir, e.Name()))
	}

	if len(mediaFiles) == 0 {
		progress.Info("No media files found in %q", fixExifDatesDir)
		progress.Success("Done. Processed: 0, Skipped: %d", skipped)
		return nil
	}

	// Task 2.4: dry-run mode — print what would be executed and exit
	if fixExifDatesDryRun {
		args := buildExiftoolArgs(mediaFiles)
		progress.Info("Would run: %s %s", exiftoolPath, strings.Join(args, " "))
		for _, f := range mediaFiles {
			progress.Info("  %s", f)
		}
		progress.Success("Dry-run complete. Would process: %d, Skipped: %d", len(mediaFiles), skipped)
		return nil
	}

	// Task 2.5: batch exiftool call
	args := buildExiftoolArgs(mediaFiles)
	cmd := exec.Command(exiftoolPath, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		progress.Error("exiftool error: %v\n%s", err, string(out))
		return fmt.Errorf("exiftool failed: %w", err)
	}
	if len(out) > 0 {
		progress.Info("%s", strings.TrimSpace(string(out)))
	}

	// Task 2.6: print summary
	progress.Success("Done. Processed: %d, Skipped: %d", len(mediaFiles), skipped)
	return nil
}

// buildExiftoolArgs constructs the exiftool argument list for the batch write.
func buildExiftoolArgs(files []string) []string {
	args := []string{
		"-overwrite_original",
		"-CreateDate<DateTimeOriginal",
		"-ModifyDate<DateTimeOriginal",
	}
	return append(args, files...)
}
