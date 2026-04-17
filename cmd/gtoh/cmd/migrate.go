package cmd

import (
	"fmt"
	"os"

	"github.com/bingzujia/google-takeout-time-helper/internal/logutil"
	"github.com/bingzujia/google-takeout-time-helper/internal/migrator"
	"github.com/spf13/cobra"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrate Google Takeout photos with EXIF metadata",
	Long: `Migrate photos from Google Takeout to a clean directory structure.

Scans year folders (Photos from XXXX) in the input directory, copies files to
the output directory, and writes CreateDate + ModifyDate from JSON sidecar
timestamps via exiftool. GPS is supplemented from JSON when absent from EXIF.
Files without a JSON sidecar are copied as-is. Generates SHA-256-based metadata
JSON files and a gtoh-log/migrate-{date}-{index}.log with per-file decisions.`,
	Args: cobra.NoArgs,
	RunE: runMigrate,
}

var (
	migrateDryRun   bool
	migrateInputDir string
	migrateOutputDir string
)

func init() {
	migrateCmd.Flags().BoolVar(&migrateDryRun, "dry-run", false, "preview migration without modifying files")
	migrateCmd.Flags().StringVar(&migrateInputDir, "input-dir", "", "input directory containing Google Takeout exports")
	migrateCmd.Flags().StringVar(&migrateOutputDir, "output-dir", "", "output directory for organized photos")
	_ = migrateCmd.MarkFlagRequired("input-dir")
	_ = migrateCmd.MarkFlagRequired("output-dir")
	rootCmd.AddCommand(migrateCmd)
}

func runMigrate(_ *cobra.Command, _ []string) error {
	inputDir := migrateInputDir
	outputDir := migrateOutputDir

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

	logger, err := logutil.OpenLog(outputDir, "migrate", migrateDryRun)
	if err != nil {
		return fmt.Errorf("open log: %w", err)
	}
	defer logger.Close()

	stats, err := migrator.Run(migrator.Config{
		InputDir:     inputDir,
		OutputDir:    outputDir,
		ShowProgress: !migrateDryRun,
		DryRun:       migrateDryRun,
		Logger:       logger,
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
	fmt.Printf("  Skipped (exists):   %d files\n", stats.SkippedExists)
	fmt.Printf("  Failed (exiftool):  %d files\n", stats.FailedExif)
	fmt.Printf("  Failed (other):     %d files\n", stats.FailedOther)
	fmt.Printf("  Manual review:      %d files\n", stats.ManualReview)
	if migrateDryRun {
		fmt.Println("  Log:                (not created in dry-run)")
	} else {
		fmt.Printf("  Log:                %s\n", logger.Path())
	}

	return nil
}
