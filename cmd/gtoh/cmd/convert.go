package cmd

import (
	"fmt"
	"os"

	"github.com/bingzujia/google-takeout-time-helper/internal/heicconv"
	"github.com/bingzujia/google-takeout-time-helper/internal/logutil"
	"github.com/spf13/cobra"
)

var convertCmd = &cobra.Command{
	Use:   "convert",
	Short: "Convert root-level images in a directory to HEIC in place",
	Long: `Convert decodable image files directly under --input-dir to .heic in place.

Only regular files in the root of --input-dir are considered. Existing target
.heic files are skipped. If a file extension does not match the actual image
content, gtoh corrects the source extension before converting, migrates EXIF
metadata onto the new HEIC file, and deletes the original only after success.

Requires:
  - heif-enc: sudo apt-get install -y libheif-examples
  - exiftool

HEIC encoding quality:
  - heif-enc: quality 35 (0–100 scale)

Images larger than 40 million pixels are detected as oversized and
are processed one at a time to reduce peak memory usage.`,
	Args: cobra.NoArgs,
	RunE: runConvert,
}

var (
	convertDryRun   bool
	convertWorkers  int
	convertInputDir string
)

func init() {
	convertCmd.Flags().BoolVar(&convertDryRun, "dry-run", false, "preview HEIC conversions without modifying files")
	convertCmd.Flags().IntVar(&convertWorkers, "workers", 2, "number of concurrent conversion workers (1–N; reduce to limit memory)")
	convertCmd.Flags().StringVar(&convertInputDir, "input-dir", "", "input directory containing images to convert")
	_ = convertCmd.MarkFlagRequired("input-dir")
	rootCmd.AddCommand(convertCmd)
}

func runConvert(_ *cobra.Command, _ []string) error {
	inputDir := convertInputDir

	info, err := os.Stat(inputDir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("input directory does not exist: %s", inputDir)
		}
		return fmt.Errorf("stat input directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("input path is not a directory: %s", inputDir)
	}

	if !convertDryRun {
		if err := heicconv.ValidateEncoderSupport(); err != nil {
			return err
		}
	}

	fmt.Printf("Input:   %s\n", inputDir)
	fmt.Printf("Workers: %d\n", convertWorkers)
	if convertDryRun {
		fmt.Println("Mode:    dry-run (no files will be modified)")
	}
	fmt.Println()

	logger, err := logutil.OpenLog(inputDir, "convert", convertDryRun)
	if err != nil {
		return fmt.Errorf("open log: %w", err)
	}
	defer logger.Close()

	stats, err := heicconv.Run(heicconv.Config{
		InputDir:     inputDir,
		DryRun:       convertDryRun,
		ShowProgress: true,
		Workers:      convertWorkers,
		Logger:       logger,
	})
	if err != nil {
		return err
	}

	fmt.Println()
	if convertDryRun {
		fmt.Println("Dry-run complete! (no files were modified)")
	} else {
		fmt.Println("HEIC conversion complete!")
	}
	fmt.Printf("  Root files scanned:   %d\n", stats.Scanned)
	if convertDryRun {
		fmt.Printf("  Planned conversions:  %d\n", stats.Planned)
	} else {
		fmt.Printf("  Converted:            %d\n", stats.Converted)
	}
	fmt.Printf("  Extension corrected:  %d\n", stats.RenamedExtensions)
	fmt.Printf("  Skipped (conflict):   %d\n", stats.SkippedConflicts)
	fmt.Printf("  Skipped (already HEIC): %d\n", stats.SkippedAlreadyHEIC)
	fmt.Printf("  Skipped (unsupported): %d\n", stats.SkippedUnsupported)
	fmt.Printf("  Failed:               %d\n", stats.Failed)
	if !convertDryRun {
		fmt.Printf("  Log:                  %s\n", logger.Path())
	}

	return nil
}
