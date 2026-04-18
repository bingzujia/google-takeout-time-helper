package cmd

import (
	"fmt"
	"os/exec"
	"path/filepath"
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

var fixNameCmd = &cobra.Command{
	Use:   "fix-name",
	Short: "Sync filename datetime → DateTimeOriginal, CreateDate & ModifyDate using exiftool",
	Long: `fix-name parses the datetime embedded in each media filename, compares it
with the EXIF DateTimeOriginal field, and writes DateTimeOriginal + CreateDate +
ModifyDate when the filename timestamp is earlier than the existing EXIF value,
or when no EXIF timestamp is present at all.

Files whose names contain no parseable datetime are skipped.

Requires exiftool to be installed and available in PATH.`,
	RunE: runFixName,
}

var fixNameDir string
var fixNameDryRun bool

func init() {
	fixNameCmd.Flags().StringVar(&fixNameDir, "input-dir", "", "target directory")
	fixNameCmd.Flags().BoolVar(&fixNameDryRun, "dry-run", false, "preview only, do not modify files")
	_ = fixNameCmd.MarkFlagRequired("input-dir")
	rootCmd.AddCommand(fixNameCmd)
}

func runFixName(_ *cobra.Command, _ []string) error {
	if _, err := exec.LookPath("exiftool"); err != nil {
		return fmt.Errorf("exiftool not found in PATH: install exiftool and retry")
	}

	mediaFiles, skipped, err := collectMediaFiles(fixNameDir)
	if err != nil {
		return err
	}

	if len(mediaFiles) == 0 {
		progress.Info("No media files found in %q", fixNameDir)
		progress.Success("Done. Processed: 0, Failed: 0, Skipped: %d", skipped)
		return nil
	}

	if fixNameDryRun {
		processed, _, noDate := runFixNameFiles(mediaFiles, fixNameRunOptions{
			DryRun:       true,
			WorkerCount:  fixExifWorkerCount(),
			ShowProgress: true,
		})
		progress.Success("Dry-run complete. Would write: %d, No filename datetime: %d, Skipped (not earlier): shown above", processed, noDate)
		return nil
	}

	logger, err := logutil.OpenLog(fixNameDir, "fix-name", false)
	if err != nil {
		return fmt.Errorf("opening log file: %w", err)
	}
	defer logger.Close()

	writeLog := func(filePath, detail string) {
		logger.Fail("write-exif", filePath, detail)
	}
	logSkip := func(filePath, reason string) {
		logger.Skip(reason, filePath)
	}
	logInfo := func(filePath, detail string) {
		logger.Info(detail, filePath)
	}

	writer := migrator.ExifWriter{}
	processed, failed, noDate := runFixNameFiles(mediaFiles, fixNameRunOptions{
		DryRun:       false,
		PrepareFile:  prepareFileForExif,
		WriteAll:     writer.WriteTimestamp,
		WriteLog:     writeLog,
		LogSkip:      logSkip,
		LogInfo:      logInfo,
		WorkerCount:  fixExifWorkerCount(),
		ShowProgress: true,
	})

	skipped += noDate
	if failed > 0 {
		progress.Success("Done. Processed: %d, Failed: %d, Skipped: %d", processed, failed, skipped)
		progress.Info("Log: %s", logger.Path())
	} else {
		progress.Success("Done. Processed: %d, Failed: %d, Skipped: %d", processed, failed, skipped)
	}
	return nil
}

type fixNameRunOptions struct {
	DryRun       bool
	PrepareFile  func(string) (workPath string, cleanup func() error, mismatch string, err error)
	WriteAll     func(string, time.Time) error
	WriteLog     func(string, string)
	LogSkip      func(string, string)
	LogInfo      func(string, string)
	WorkerCount  int
	ShowProgress bool
}

// isPXLFile reports whether filename has the PXL_ prefix used by Google Pixel
// cameras. Pixel embeds UTC in the filename, so the timestamp must be shifted
// to local time before writing to EXIF.
func isPXLFile(filename string) bool {
	return strings.HasPrefix(strings.ToUpper(filepath.Base(filename)), "PXL_")
}

// runFixNameFiles processes files and returns (processed, failed, skipped) counts.
// skipped covers files with no parseable filename datetime and files that could
// not have their type detected.
func runFixNameFiles(mediaFiles []string, opts fixNameRunOptions) (processed, failed, skipped int) {
	if len(mediaFiles) == 0 {
		return 0, 0, 0
	}

	workers := opts.WorkerCount
	if workers < 1 {
		workers = fixExifWorkerCount()
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var processedCount, failedCount, skippedCount int
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
					nameTime, nameOk := parser.ParseFilenameTimestamp(filepath.Base(filePath))
					if !nameOk {
						mu.Lock()
						skippedCount++
						mu.Unlock()
						return
					}

					// PXL_ filenames embed UTC; convert to local time so EXIF
					// receives the wall-clock value in the user's timezone.
					if isPXLFile(filePath) {
						nameTime = nameTime.In(time.Local)
					}

					if opts.DryRun {
						mu.Lock()
						processedCount++
						progress.Info("  %s  would write: %s", filePath, nameTime.Format("2006-01-02 15:04:05"))
						mu.Unlock()
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

					if err := opts.WriteAll(workPath, nameTime); err != nil {
						_ = cleanup()
						mu.Lock()
						failedCount++
						if opts.WriteLog != nil {
							opts.WriteLog(filePath, err.Error())
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
