package cmd

import (
	"os"
	"path/filepath"
	"strings"

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

func runFixTakeout(_ *cobra.Command, _ []string) error {
	// Find all "Photos from*" subdirs
	entries, err := os.ReadDir(fixTakeoutDir)
	if err != nil {
		return err
	}

	writer := metadata.NewWriter()

	totalMatched := 0
	totalSkipped := 0

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

		// Collect all non-JSON files
		dirEntries, err := os.ReadDir(photoDir)
		if err != nil {
			progress.Warning("Error reading %s: %v", photoDir, err)
			continue
		}

		var photos []string
		for _, e := range dirEntries {
			if e.IsDir() {
				continue
			}
			if !strings.EqualFold(filepath.Ext(e.Name()), ".json") {
				photos = append(photos, e.Name())
			}
		}

		for _, name := range photos {
			photoPath := filepath.Join(photoDir, name)
			jsonResult := matcher.JSONForFile(photoPath)
			if jsonResult == nil {
				totalSkipped++
				continue
			}

			if fixTakeoutDryRun {
				progress.Info("Would set %s → %v", photoPath, jsonResult.Timestamp)
				totalMatched++
				continue
			}

			if jsonResult.Timestamp.IsZero() {
				progress.Warning("No timestamp for %s, skipping", photoPath)
				totalSkipped++
				continue
			}

			if err := writer.WriteTimestamp(photoPath, jsonResult.Timestamp); err != nil {
				progress.Error("WriteTimestamp %s: %v", photoPath, err)
				totalSkipped++
				continue
			}

			if jsonResult.Lat != 0 || jsonResult.Lon != 0 {
				if err := writer.WriteGPS(photoPath, jsonResult.Lat, jsonResult.Lon, jsonResult.Alt); err != nil {
					progress.Warning("WriteGPS %s: %v", photoPath, err)
				}
			}

			totalMatched++
		}
	}

	progress.Success("Done. Matched: %d, Skipped: %d", totalMatched, totalSkipped)
	return nil
}
