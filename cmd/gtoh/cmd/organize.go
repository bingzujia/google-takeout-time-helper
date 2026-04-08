package cmd

import (
	"fmt"

	"github.com/bingzujia/g_photo_take_out_helper/internal/organizer"
	"github.com/bingzujia/g_photo_take_out_helper/internal/progress"
	"github.com/spf13/cobra"
)

var organizeCmd = &cobra.Command{
	Use:   "organize",
	Short: "Organize photos by type (camera, screenshot, wechat)",
	RunE:  runOrganize,
}

var organizeMode string
var organizeDir string
var organizeDestDir string
var organizeDryRun bool
var organizeList bool
var organizeRecursive bool

func init() {
	organizeCmd.Flags().StringVar(&organizeMode, "mode", "", "camera|screenshot|wechat (required)")
	organizeCmd.Flags().StringVar(&organizeDir, "dir", ".", "source directory")
	organizeCmd.Flags().StringVar(&organizeDestDir, "dest", "", "destination directory (defaults to <dir>/<mode>)")
	organizeCmd.Flags().BoolVar(&organizeDryRun, "dry-run", false, "preview only")
	organizeCmd.Flags().BoolVar(&organizeList, "list", false, "list matching files without moving")
	organizeCmd.Flags().BoolVar(&organizeRecursive, "recursive", false, "scan source directory recursively")
	_ = organizeCmd.MarkFlagRequired("mode")
}

func runOrganize(_ *cobra.Command, _ []string) error {
	mode := organizer.Mode(organizeMode)
	switch mode {
	case organizer.ModeCamera, organizer.ModeScreenshot, organizer.ModeWechat:
	default:
		return fmt.Errorf("invalid mode %q: must be camera, screenshot, or wechat", organizeMode)
	}

	destDir := organizeDestDir
	if destDir == "" {
		destDir = organizeDir + "/" + organizeMode
	}

	cfg := organizer.Config{
		Mode:       mode,
		SourceDirs: []string{organizeDir},
		DestDir:    destDir,
		DryRun:     organizeDryRun || organizeList,
		Recursive:  organizeRecursive,
	}

	result, err := organizer.Run(cfg)
	if err != nil {
		return err
	}

	if organizeList {
		progress.Info("Matching files: %d", result.Moved)
	} else {
		progress.Success("Done. Moved: %d, Skipped: %d", result.Moved, result.Skipped)
	}
	return nil
}
