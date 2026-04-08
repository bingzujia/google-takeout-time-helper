package cmd

import (
	"os"
	"path/filepath"
	"time"

	"github.com/bingzujia/g_photo_take_out_helper/internal/parser"
	"github.com/bingzujia/g_photo_take_out_helper/internal/progress"
	"github.com/spf13/cobra"
)

var fixImgCmd = &cobra.Command{
	Use:   "fix-img",
	Short: "Fix timestamps of IMG/VID files using embedded filename timestamp",
	RunE:  runFixImg,
}

var fixImgDir string
var fixImgDryRun bool

func init() {
	fixImgCmd.Flags().StringVar(&fixImgDir, "dir", ".", "target directory")
	fixImgCmd.Flags().BoolVar(&fixImgDryRun, "dry-run", false, "preview only")
}

func runFixImg(_ *cobra.Command, _ []string) error {
	fixed := 0
	skipped := 0

	err := filepath.WalkDir(fixImgDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}

		t, ok := parser.ParseIMGVIDFilename(d.Name())
		if !ok {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		// Skip if mtime already within 1s of parsed time
		if diff := info.ModTime().Sub(t); diff < time.Second && diff > -time.Second {
			skipped++
			return nil
		}

		if fixImgDryRun {
			progress.Info("Would set %s → %v", path, t)
			fixed++
			return nil
		}

		if err := os.Chtimes(path, t, t); err != nil {
			progress.Error("Chtimes %s: %v", path, err)
			return nil
		}
		fixed++
		return nil
	})

	if err != nil {
		return err
	}

	progress.Success("Done. Fixed: %d, Skipped: %d", fixed, skipped)
	return nil
}
