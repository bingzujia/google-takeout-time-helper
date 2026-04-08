package cmd

import (
	"github.com/bingzujia/g_photo_take_out_helper/internal/progress"
	"github.com/bingzujia/g_photo_take_out_helper/internal/renamer"
	"github.com/spf13/cobra"
)

var renameCmd = &cobra.Command{
	Use:   "rename",
	Short: "Rename media files based on their modification time",
	RunE:  runRename,
}

var renameDir string
var renameDryRun bool
var renameList bool

func init() {
	renameCmd.Flags().StringVar(&renameDir, "dir", ".", "target directory")
	renameCmd.Flags().BoolVar(&renameDryRun, "dry-run", false, "preview only")
	renameCmd.Flags().BoolVar(&renameList, "list", false, "list files and proposed new names")
}

func runRename(_ *cobra.Command, _ []string) error {
	cfg := renamer.Config{
		Dir:    renameDir,
		DryRun: renameDryRun || renameList,
	}

	result, err := renamer.Run(cfg)
	if err != nil {
		return err
	}

	progress.Success("Done. Renamed: %d, Skipped: %d, Errors: %d", result.Renamed, result.Skipped, result.Errors)
	return nil
}
