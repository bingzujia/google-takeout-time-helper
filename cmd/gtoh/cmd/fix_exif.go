package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bingzujia/google-takeout-time-helper/internal/logutil"
	"github.com/bingzujia/google-takeout-time-helper/internal/migrator"
	"github.com/bingzujia/google-takeout-time-helper/internal/parser"
	"github.com/bingzujia/google-takeout-time-helper/internal/progress"
	"github.com/spf13/cobra"
)

var fixExifCmd = &cobra.Command{
	Use:   "fix-exif",
	Short: "Sync DateTimeOriginal → CreateDate & ModifyDate using exiftool",
	Long: `fix-exif reads the DateTimeOriginal EXIF field from media files in the
given directory (first level only, non-recursive) and writes the same value to
CreateDate and ModifyDate, using exiftool.

Requires exiftool to be installed and available in PATH.`,
	RunE: runFixExif,
}

var fixExifDir string
var fixExifDryRun bool

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
	fixExifCmd.Flags().StringVar(&fixExifDir, "input-dir", "", "target directory")
	fixExifCmd.Flags().BoolVar(&fixExifDryRun, "dry-run", false, "preview only, do not modify files")
	_ = fixExifCmd.MarkFlagRequired("input-dir")
	rootCmd.AddCommand(fixExifCmd)
}

func runFixExif(_ *cobra.Command, _ []string) error {
	if _, err := exec.LookPath("exiftool"); err != nil {
		return fmt.Errorf("exiftool not found in PATH: install exiftool and retry")
	}

	mediaFiles, skipped, err := collectMediaFiles(fixExifDir)
	if err != nil {
		return err
	}

	if len(mediaFiles) == 0 {
		progress.Info("No media files found in %q", fixExifDir)
		progress.Success("Done. Processed: 0, Failed: 0, Skipped: %d", skipped)
		return nil
	}

	// Dry-run: resolve timestamp per file and print it.
	if fixExifDryRun {
		runFixExifFiles(mediaFiles, fixExifRunOptions{
			DryRun:           true,
			ResolveTimestamp: resolveTimestamp,
			ReportDryRun:     reportFixExifDryRun,
			WorkerCount:      fixExifWorkerCount(),
			ShowProgress:     true,
		})
		progress.Success("Dry-run complete. Would process: %d, Skipped: %d", len(mediaFiles), skipped)
		return nil
	}

	// Open log file for structured logging.
	logger, err := logutil.OpenLog(fixExifDir, "fix-exif", false)
	if err != nil {
		return fmt.Errorf("opening log file: %w", err)
	}
	defer logger.Close()

	writeLog := func(filePath, detail string) {
		logger.Fail("write-exif", filePath, detail)
	}

	processed, failed := 0, 0
	writer := migrator.ExifWriter{}
	processed, failed = runFixExifFiles(mediaFiles, fixExifRunOptions{
		DryRun:           false,
		ResolveTimestamp: resolveTimestamp,
		WriteTimestamp:   writer.WriteTimestamp,
		WriteLog:         writeLog,
		ReportFailure:    reportFixExifFailure,
		WorkerCount:      fixExifWorkerCount(),
		ShowProgress:     true,
	})

	if failed > 0 {
		progress.Success("Done. Processed: %d, Failed: %d, Skipped: %d", processed, failed, skipped)
		progress.Info("Log: %s", logger.Path())
	} else {
		progress.Success("Done. Processed: %d, Failed: %d, Skipped: %d", processed, failed, skipped)
	}
	return nil
}

type fixExifRunOptions struct {
	DryRun           bool
	ResolveTimestamp func(string) (time.Time, string, bool)
	WriteTimestamp   func(string, time.Time) error
	WriteLog         func(string, string)
	ReportDryRun     func(string, time.Time, string, bool)
	ReportFailure    func(string, string)
	WorkerCount      int
	ShowProgress     bool
}

func collectMediaFiles(dir string) ([]string, int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, 0, fmt.Errorf("reading directory %q: %w", dir, err)
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
		mediaFiles = append(mediaFiles, filepath.Join(dir, e.Name()))
	}
	return mediaFiles, skipped, nil
}

func fixExifWorkerCount() int {
	workers := runtime.NumCPU()
	if workers > 8 {
		workers = 8
	}
	if workers < 1 {
		return 1
	}
	return workers
}

func runFixExifFiles(mediaFiles []string, opts fixExifRunOptions) (processed, failed int) {
	if len(mediaFiles) == 0 {
		return 0, 0
	}

	workers := opts.WorkerCount
	if workers < 1 {
		workers = fixExifWorkerCount()
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var processedCount int
	var failedCount int
	var completed atomic.Int64

	reporter := progress.NewReporter(len(mediaFiles), opts.ShowProgress)
	defer reporter.Close()

	jobCh := make(chan string, workers)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for filePath := range jobCh {
				if opts.DryRun {
					t, src, ok := opts.ResolveTimestamp(filePath)
					if opts.ReportDryRun != nil {
						mu.Lock()
						opts.ReportDryRun(filePath, t, src, ok)
						mu.Unlock()
					}
					reporter.Update(int(completed.Add(1)))
					continue
				}

				t, src, ok := opts.ResolveTimestamp(filePath)
				if !ok {
					mu.Lock()
					failedCount++
					if opts.WriteLog != nil {
						opts.WriteLog(filePath, "no DateTimeOriginal and no filename timestamp")
					}
					if opts.ReportFailure != nil {
						opts.ReportFailure(filePath, "no DateTimeOriginal and no filename timestamp")
					}
					mu.Unlock()
					reporter.Update(int(completed.Add(1)))
					continue
				}

				if src == "filename" && opts.WriteLog != nil {
					mu.Lock()
					opts.WriteLog(filePath, "no DateTimeOriginal; timestamp from filename")
					mu.Unlock()
				}

				if err := opts.WriteTimestamp(filePath, t); err != nil {
					detail := err.Error()
					mu.Lock()
					failedCount++
					if opts.WriteLog != nil {
						opts.WriteLog(filePath, detail)
					}
					if opts.ReportFailure != nil {
						opts.ReportFailure(filePath, detail)
					}
					mu.Unlock()
					reporter.Update(int(completed.Add(1)))
					continue
				}

				mu.Lock()
				processedCount++
				mu.Unlock()
				reporter.Update(int(completed.Add(1)))
			}
		}()
	}

	for _, filePath := range mediaFiles {
		jobCh <- filePath
	}
	close(jobCh)
	wg.Wait()

	return processedCount, failedCount
}

func reportFixExifDryRun(filePath string, t time.Time, src string, ok bool) {
	if !ok {
		progress.Info("  %s  (no DateTimeOriginal and no filename timestamp)", filePath)
		return
	}
	if src == "filename" {
		progress.Info("  %s  DateTimeOriginal=%s (from filename)", filePath, t.Format("2006:01:02 15:04:05"))
		return
	}
	progress.Info("  %s  DateTimeOriginal=%s", filePath, t.Format("2006:01:02 15:04:05"))
}

func reportFixExifFailure(filePath, detail string) {
	progress.Error("FAIL %s: %s", filepath.Base(filePath), detail)
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
