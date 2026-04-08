package cmd

import (
	"os"
	"path/filepath"

	"github.com/bingzujia/g_photo_take_out_helper/internal/matcher"
	"github.com/bingzujia/g_photo_take_out_helper/internal/metadata"
	"github.com/bingzujia/g_photo_take_out_helper/internal/progress"
	"github.com/spf13/cobra"
)

var fixTakeoutCmd = &cobra.Command{
	Use:   "fix-takeout",
	Short: "Fix timestamps of Google Takeout photos using JSON sidecars",
	RunE:  runFixTakeout,
}

var fixTakeoutDir string
var fixTakeoutDryRun bool

func init() {
	fixTakeoutCmd.Flags().StringVar(&fixTakeoutDir, "dir", ".", "target directory")
	fixTakeoutCmd.Flags().BoolVar(&fixTakeoutDryRun, "dry-run", false, "preview only")
}

func runFixTakeout(cmd *cobra.Command, args []string) error {
	// Find all "Photos from*" subdirs
	entries, err := os.ReadDir(fixTakeoutDir)
	if err != nil {
		return err
	}

	writer := metadata.NewWriter()

	totalMatched := 0
	totalUnmatched := 0

	photoDirs := []string{}
	for _, e := range entries {
		if e.IsDir() && len(e.Name()) >= 11 && e.Name()[:11] == "Photos from" {
			photoDirs = append(photoDirs, filepath.Join(fixTakeoutDir, e.Name()))
		}
	}

	// Also check if dir itself contains JSON files
	if len(photoDirs) == 0 {
		photoDirs = append(photoDirs, fixTakeoutDir)
	}

	for _, photoDir := range photoDirs {
		progress.Info("Processing %s", photoDir)
		results, unmatched, err := matcher.MatchAll(photoDir)
		if err != nil {
			progress.Warning("Error scanning %s: %v", photoDir, err)
			continue
		}

		totalUnmatched += len(unmatched)

		for i, mr := range results {
			progress.PrintProgress(i+1, len(results))

			if fixTakeoutDryRun {
				progress.Info("Would set %s → %v", mr.PhotoFile, mr.Timestamp)
				totalMatched++
				continue
			}

			if mr.Timestamp.IsZero() {
				progress.Warning("No timestamp for %s, skipping", mr.PhotoFile)
				continue
			}

			if err := writer.WriteTimestamp(mr.PhotoFile, mr.Timestamp); err != nil {
				progress.Error("WriteTimestamp %s: %v", mr.PhotoFile, err)
				continue
			}

			if mr.Lat != 0 || mr.Lon != 0 {
				if err := writer.WriteGPS(mr.PhotoFile, mr.Lat, mr.Lon, mr.Alt); err != nil {
					progress.Warning("WriteGPS %s: %v", mr.PhotoFile, err)
				}
			}

			totalMatched++
		}
		// newline after progress bar
		if len(results) > 0 {
			println()
		}
	}

	progress.Success("Done. Matched: %d, Unmatched JSON: %d", totalMatched, totalUnmatched)
	return nil
}
