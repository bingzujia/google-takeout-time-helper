package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bingzujia/google-takeout-time-helper/internal/logutil"
	"github.com/bingzujia/google-takeout-time-helper/internal/qqmedia"
)

var (
	qqMediaInputDir string
	qqMediaDryRun   bool
)

var formatQQMediaCmd = &cobra.Command{
	Use:   "format-qq-media",
	Short: "Format QQ exported media files with standardized naming",
	Long: `Format QQ exported media files with standardized naming based on timestamps.

Supported QQ filename timestamp patterns:
  1. _YYYYMMDD_HHMMSS         (e.g., _20170709_002844)
  2. 13-digit Unix ms          (e.g., 1688017744459)
  3. QQ视频YYYYMMDDHHMMSS       (e.g., QQ视频20150720105516)
  4. Record_YYYY-MM-DD-HH-MM-SS (e.g., Record_2024-12-19-16-07-17)
  5. Snipaste_YYYY-MM-DD_HH-MM-SS (e.g., Snipaste_2018-09-17_18-07-29)
  6. tb_image_share_13digits   (e.g., tb_image_share_1661951220361)
  7. TIM图片YYYYMMDDHHMMSS      (e.g., TIM图片20181215191143)
  Fallback: File modification time (if no timestamp pattern found)

Output format:
  • Images: Image_<unix-ms>.{ext}
  • Videos: Video_<unix-ms>.{ext}

Example:
  photo_1688017744459.jpg → Image_1688017744459.jpg
  video_20240101_120000.mp4 → Video_1704096000000.mp4

The command processes only root-level files (non-recursive) and generates a 
detailed audit log at: {input-dir}/takeout-helper-log/format-qq-media-{YYYYMMDD}-{NNN}.log`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runFormatQQMedia()
	},
}

func init() {
	rootCmd.AddCommand(formatQQMediaCmd)
	formatQQMediaCmd.Flags().StringVar(&qqMediaInputDir, "input-dir", "", "target directory containing QQ media files")
	formatQQMediaCmd.Flags().BoolVar(&qqMediaDryRun, "dry-run", false, "preview formatting without renaming files")
	formatQQMediaCmd.MarkFlagRequired("input-dir")
}

func runFormatQQMedia() error {
	logger, err := logutil.OpenLog(qqMediaInputDir, "format-qq-media", qqMediaDryRun)
	if err != nil {
		return fmt.Errorf("open log: %w", err)
	}
	defer logger.Close()

	cfg := qqmedia.Config{
		Dir:    qqMediaInputDir,
		DryRun: qqMediaDryRun,
		Logger: logger,
	}

	stats, err := qqmedia.Run(cfg)
	if err != nil {
		return err
	}

	printStats(stats.Renamed, stats.Skipped, stats.Errors)
	printLogPath(qqMediaDryRun, logger)
	return nil
}
