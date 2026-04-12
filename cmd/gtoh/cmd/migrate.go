package cmd

import (
	"fmt"
	"os"

	"github.com/bingzujia/g_photo_take_out_helper/internal/migrator"
	"github.com/spf13/cobra"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate <input_dir> <output_dir>",
	Short: "Migrate Google Takeout photos with EXIF metadata",
	Long: `Migrate photos from Google Takeout to a clean directory structure.

Scans year folders (Photos from XXXX) in the input directory, extracts
timestamps and GPS from EXIF/filename/JSON sidecars, copies files to
the output directory, writes EXIF metadata via exiftool, and generates
SHA-256-based metadata JSON files.`,
	Args: cobra.ExactArgs(2),
	RunE: runMigrate,
}

var migrateDryRun bool

func init() {
	migrateCmd.Flags().BoolVar(&migrateDryRun, "dry-run", false, "preview migration without modifying files")
	rootCmd.AddCommand(migrateCmd)
}

func runMigrate(_ *cobra.Command, args []string) error {
	inputDir := args[0]
	outputDir := args[1]

	// Validate input directory
	if _, err := os.Stat(inputDir); os.IsNotExist(err) {
		return fmt.Errorf("input directory does not exist: %s", inputDir)
	}

	fmt.Printf("Input:  %s\n", inputDir)
	fmt.Printf("Output: %s\n", outputDir)

	if migrateDryRun {
		fmt.Println("\nDry-run mode — no files will be modified")
	} else {
		fmt.Println()
	}

	stats, err := migrator.Run(migrator.Config{
		InputDir:     inputDir,
		OutputDir:    outputDir,
		ShowProgress: !migrateDryRun,
		DryRun:       migrateDryRun,
	})
	if err != nil {
		return err
	}

	// Print summary
	fmt.Println()
	if migrateDryRun {
		fmt.Println("Dry-run complete! (no files were modified)")
	} else {
		fmt.Println("Processing complete!")
	}
	fmt.Printf("  Scanned:            %d files\n", stats.Scanned)
	fmt.Printf("  Processed:          %d files\n", stats.Processed)
	fmt.Printf("  Skipped (no time):  %d files\n", stats.SkippedNoTime)
	fmt.Printf("  Skipped (exists):   %d files\n", stats.SkippedExists)
	fmt.Printf("  Failed (exiftool):  %d files\n", stats.FailedExif)
	fmt.Printf("  Failed (other):     %d files\n", stats.FailedOther)
	fmt.Printf("  Manual review:      %d files\n", stats.ManualReview)
	if migrateDryRun {
		fmt.Println("  Log:                (not created in dry-run)")
	} else {
		fmt.Printf("  Log:                %s/gtoh.log\n", outputDir)
	}

	return nil
}
