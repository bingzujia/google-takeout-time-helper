package cmd

import (
	"fmt"

	"github.com/bingzujia/google-takeout-time-helper/internal/logutil"
	"github.com/bingzujia/google-takeout-time-helper/internal/screenshotter"
	"github.com/spf13/cobra"
)

var (
	renameScreenshotInputDir string
	renameScreenshotDryRun   bool
)

var renameScreenshotCmd = &cobra.Command{
	Use:   "rename-screenshot",
	Short: "Rename screenshot files using timestamp-based naming conventions",
	Long: `Rename screenshot files in the specified directory using standardized naming conventions.

Supported screenshot formats:
  1. YYYY-MM-DD-HH-MM-SS-MS     (e.g., Screenshot_2025-07-18-09-23-54-65.png)
  2. YYYYMMDD_HHMMSS             (e.g., screenshot20250718_092354.jpg)
  3. YYYY-MM-DD_HH-MM-SS         (e.g., Screenshot_2025-07-18_09-23-54.png)
  4. YYYY_M_D_H_M_S              (e.g., screenshot_2025_7_18_9_23_54.png)
  5. YYYY-MM-DD (date-only)       (e.g., screenshot_2025-07-18.png)
  6a. Unix timestamp seconds (10) (e.g., screenshot1634560000.jpg)
  6b. Unix timestamp milliseconds (13) (e.g., mmscreenshot1727421404387.jpg)

Output format: Screenshot_YYYY-MM-DD-HH-MM-SS-MS.{ext}

Conflict handling: Automatically appends _001, _002, etc. when target filename exists.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runRenameScreenshot()
	},
}

func runRenameScreenshot() error {
	logger, err := logutil.OpenLog(renameScreenshotInputDir, "rename-screenshot", renameScreenshotDryRun)
	if err != nil {
		return fmt.Errorf("open log: %w", err)
	}
	defer logger.Close()

	cfg := screenshotter.Config{Dir: renameScreenshotInputDir, DryRun: renameScreenshotDryRun, Logger: logger}
	result, err := screenshotter.Run(cfg)
	if err != nil {
		return err
	}
	printStats(result.Renamed, result.Skipped, result.Errors)
	printLogPath(renameScreenshotDryRun, logger)
	return nil
}

func init() {
	rootCmd.AddCommand(renameScreenshotCmd)
	renameScreenshotCmd.Flags().StringVar(&renameScreenshotInputDir, "input-dir", "", "Target directory containing screenshot files")
	renameScreenshotCmd.Flags().BoolVar(&renameScreenshotDryRun, "dry-run", false, "Preview renamed results without modifying filesystem")
	_ = renameScreenshotCmd.MarkFlagRequired("input-dir")
}
