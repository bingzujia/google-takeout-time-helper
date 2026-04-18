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

Files without a DateTimeOriginal field are skipped.
Files whose extension does not match the actual content (e.g. a JPEG named .png)
are temporarily renamed to the correct extension, processed, then restored.

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

	logFail := func(filePath, detail string) {
		logger.Fail("write-exif", filePath, detail)
	}
	logSkip := func(filePath, reason string) {
		logger.Skip(reason, filePath)
	}
	logInfo := func(filePath, detail string) {
		logger.Info(detail, filePath)
	}

	processed, failed, noExif := 0, 0, 0
	writer := migrator.ExifWriter{}
	processed, failed, noExif = runFixExifFiles(mediaFiles, fixExifRunOptions{
		DryRun:           false,
		PrepareFile:      prepareFileForExif,
		ResolveTimestamp: resolveTimestamp,
		WriteTimestamp:   writer.WriteTimestamp,
		WriteLog:         logFail,
		LogSkip:          logSkip,
		LogInfo:          logInfo,
		ReportFailure:    reportFixExifFailure,
		WorkerCount:      fixExifWorkerCount(),
		ShowProgress:     true,
	})

	skipped += noExif
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
	PrepareFile      func(string) (workPath string, cleanup func() error, mismatch string, err error)
	ResolveTimestamp func(string) (time.Time, bool)
	WriteTimestamp   func(string, time.Time) error
	WriteLog         func(string, string)
	LogSkip          func(string, string)
	LogInfo          func(string, string)
	ReportDryRun     func(string, time.Time, bool)
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

func runFixExifFiles(mediaFiles []string, opts fixExifRunOptions) (processed, failed, skipped int) {
	if len(mediaFiles) == 0 {
		return 0, 0, 0
	}

	workers := opts.WorkerCount
	if workers < 1 {
		workers = fixExifWorkerCount()
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var processedCount int
	var failedCount int
	var skippedCount int
	var completed atomic.Int64

	reporter := progress.NewReporter(len(mediaFiles), opts.ShowProgress)
	defer reporter.Close()

	jobCh := make(chan string, workers)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for filePath := range jobCh {
				func() {
					if opts.DryRun {
						t, ok := opts.ResolveTimestamp(filePath)
						if opts.ReportDryRun != nil {
							mu.Lock()
							opts.ReportDryRun(filePath, t, ok)
							mu.Unlock()
						}
						return
					}

					workPath := filePath
					var mismatch string
					cleanup := func() error { return nil }

					if opts.PrepareFile != nil {
						var err error
						workPath, cleanup, mismatch, err = opts.PrepareFile(filePath)
						if err != nil {
							mu.Lock()
							skippedCount++
							if opts.LogSkip != nil {
								opts.LogSkip(filePath, err.Error())
							}
							mu.Unlock()
							return
						}
					}

					t, ok := opts.ResolveTimestamp(workPath)
					if !ok {
						_ = cleanup()
						mu.Lock()
						skippedCount++
						if opts.LogSkip != nil {
							opts.LogSkip(filePath, "no DateTimeOriginal")
						}
						mu.Unlock()
						return
					}

					if err := opts.WriteTimestamp(workPath, t); err != nil {
						_ = cleanup()
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
						return
					}

					if err := cleanup(); err != nil {
						mu.Lock()
						failedCount++
						detail := fmt.Sprintf("restore rename failed: %v", err)
						if opts.WriteLog != nil {
							opts.WriteLog(filePath, detail)
						}
						if opts.ReportFailure != nil {
							opts.ReportFailure(filePath, detail)
						}
						mu.Unlock()
						return
					}

					if mismatch != "" {
						mu.Lock()
						if opts.LogInfo != nil {
							opts.LogInfo(filePath, mismatch)
						}
						mu.Unlock()
					}

					mu.Lock()
					processedCount++
					mu.Unlock()
				}()
				reporter.Update(int(completed.Add(1)))
			}
		}()
	}

	for _, filePath := range mediaFiles {
		jobCh <- filePath
	}
	close(jobCh)
	wg.Wait()

	return processedCount, failedCount, skippedCount
}

// resolveTimestamp reads the DateTimeOriginal EXIF field from the given media
// file. Returns (zero, false) if the field is absent.
func resolveTimestamp(filePath string) (t time.Time, ok bool) {
	return parser.ParseEXIFTimestamp(filePath)
}

func reportFixExifDryRun(filePath string, t time.Time, ok bool) {
	if !ok {
		progress.Info("  %s  (no DateTimeOriginal; will skip)", filePath)
		return
	}
	progress.Info("  %s  DateTimeOriginal=%s", filePath, t.Format("2006:01:02 15:04:05"))
}

func reportFixExifFailure(filePath, detail string) {
	progress.Error("FAIL %s: %s", filepath.Base(filePath), detail)
}

// prepareFileForExif checks whether the file's extension matches its actual
// content. If there is a mismatch, the file is temporarily renamed to the
// correct extension so exiftool can operate on it.
//
// Returns:
//   - workPath: path to pass to exiftool (may differ from filePath)
//   - cleanup: must be called after exiftool operations to restore the original name
//   - mismatch: non-empty description of the mismatch (for logging on success)
//   - err: non-nil when the actual type is known but cannot be mapped to an
//     extension — the caller should skip the file
//
// If file-type detection fails entirely (e.g. the `file` command is absent),
// the file is returned unchanged so exiftool can attempt it anyway.
func prepareFileForExif(filePath string) (workPath string, cleanup func() error, mismatch string, err error) {
	ft, detErr := migrator.DetectFileAll(filePath)
	if detErr != nil {
		// Detection unavailable; fall through and let exiftool try.
		return filePath, func() error { return nil }, "", nil
	}
	if !ft.TypeKnown {
		return "", nil, "", fmt.Errorf("unknown file type: %s", ft.MimeType)
	}
	if ft.NewExt == "" {
		// Extension already matches the actual content.
		return filePath, func() error { return nil }, "", nil
	}

	// Mismatch: rename to correct extension before calling exiftool.
	dir := filepath.Dir(filePath)
	stem := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	tmpPath := filepath.Join(dir, stem+ft.NewExt)
	if err := os.Rename(filePath, tmpPath); err != nil {
		return "", nil, "", fmt.Errorf("rename for exiftool: %w", err)
	}

	oldExt := strings.ToLower(filepath.Ext(filePath))
	msg := fmt.Sprintf("extension mismatch: %s→%s", oldExt, ft.NewExt)
	return tmpPath, func() error { return os.Rename(tmpPath, filePath) }, msg, nil
}
