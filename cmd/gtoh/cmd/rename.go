package cmd

import (
	"fmt"

	"github.com/bingzujia/google-takeout-time-helper/internal/logutil"
	"github.com/bingzujia/google-takeout-time-helper/internal/renamer"
	"github.com/spf13/cobra"
)

var renameInputDir string

var renameCmd = &cobra.Command{
	Use:   "rename",
	Short: "Rename media files using timestamp-based naming conventions",
	Long: `Rename media files in the specified directory using timestamp-based naming:

  HEIC/HEIF 图片: IMG{YYYYMMDD}{HHMMSS}.{ext}
  其他图片:       IMG_{YYYYMMDD}_{HHMMSS}.{ext}
  独立视频:       VID{YYYYMMDD}{HHMMSS}.{ext}
  连拍（Burst）:  IMG{YYYYMMDD}{HHMMSS}_BURST{NNN}.{ext}（HEIC）
                  IMG_{YYYYMMDD}_{HHMMSS}_BURST{NNN}.{ext}（其他）

连拍检测：文件名匹配 YYYYMMDD_HHMMSS_NNN.ext 且同前缀 ≥2 个文件时触发。
MP4 伴侣：与图片同名的 .mp4 文件随图片一起重命名。
冲突处理：目标名已存在时自动追加 _001、_002 后缀。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dryRun, _ := cmd.Flags().GetBool("dry-run")

		logger, err := logutil.OpenLog(renameInputDir, "rename", dryRun)
		if err != nil {
			return fmt.Errorf("open log: %w", err)
		}
		defer logger.Close()

		cfg := renamer.Config{Dir: renameInputDir, DryRun: dryRun, Logger: logger}
		result, err := renamer.Run(cfg)
		if err != nil {
			return err
		}
		fmt.Printf("Renamed: %d, Skipped: %d, Errors: %d\n", result.Renamed, result.Skipped, result.Errors)
		if !dryRun {
			fmt.Printf("Log: %s\n", logger.Path())
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(renameCmd)
	renameCmd.Flags().StringVar(&renameInputDir, "input-dir", "", "目标目录")
	renameCmd.Flags().Bool("dry-run", false, "仅预览重命名，不实际修改")
	_ = renameCmd.MarkFlagRequired("input-dir")
}
