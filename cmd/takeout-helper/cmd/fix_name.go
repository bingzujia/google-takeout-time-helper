package cmd

import (
	"fmt"
	"os/exec"
	"path/filepath"
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
		processed, noparse := runFixNameFiles(mediaFiles, fixNameRunOptions{
			DryRun:      true,
			WorkerCount: fixExifWorkerCount(),
			ShowProgress: true,
		})
		progress.Success("Dry-run complete. Would process: %d, No filename datetime: %d, Skipped (not earlier): already counted, Other: %d", processed, noparse, 0)
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

	writer := migrator.ExifWriter{}
	processed, failed := runFixNameFiles(mediaFiles, fixNameRunOptions{
		DryRun:      false,
		WriteAll:    writer.WriteTimestamp,
		WriteLog:    writeLog,
		WorkerCount: fixExifWorkerCount(),
		ShowProgress: true,
	})

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
	WriteAll     func(string, time.Time) error
	WriteLog     func(string, string)
	WorkerCount  int
	ShowProgress bool
}

// runFixNameFiles processes files and returns (processed, noFilenameDate) counts.
func runFixNameFiles(mediaFiles []string, opts fixNameRunOptions) (processed, noFilenameDate int) {
	if len(mediaFiles) == 0 {
		return 0, 0
	}

	workers := opts.WorkerCount
	if workers < 1 {
		workers = fixExifWorkerCount()
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var processedCount, noDateCount int
	var completed atomic.Int64

	reporter := progress.NewReporter(len(mediaFiles), opts.ShowProgress)
	defer reporter.Close()

	jobCh := make(chan string, workers)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for filePath := range jobCh {
				nameTime, nameOk := parser.ParseFilenameTimestamp(filepath.Base(filePath))
				if !nameOk {
					mu.Lock()
					noDateCount++
					mu.Unlock()
					reporter.Update(int(completed.Add(1)))
					continue
				}

				exifTime, exifOk := parser.ParseEXIFTimestamp(filePath)

				// Write only when filename is earlier than EXIF, or EXIF is absent
				shouldWrite := !exifOk || nameTime.Before(exifTime)

				if opts.DryRun {
					mu.Lock()
					if shouldWrite {
						processedCount++
						action := "would write (no EXIF)"
						if exifOk {
							action = fmt.Sprintf("would write (filename %s < exif %s)",
								nameTime.Format("2006-01-02 15:04:05"),
								exifTime.Format("2006-01-02 15:04:05"))
						}
						progress.Info("  %s  %s", filePath, action)
					}
					mu.Unlock()
					reporter.Update(int(completed.Add(1)))
					continue
				}

				if !shouldWrite {
					reporter.Update(int(completed.Add(1)))
					continue
				}

				if err := opts.WriteAll(filePath, nameTime); err != nil {
					detail := err.Error()
					mu.Lock()
					if opts.WriteLog != nil {
						opts.WriteLog(filePath, detail)
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

	return processedCount, noDateCount
}
