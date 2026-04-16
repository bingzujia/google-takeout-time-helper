package cmd

import (
	"fmt"
	"os"

	"github.com/bingzujia/g_photo_take_out_helper/internal/heicconv"
	"github.com/spf13/cobra"
)

var toHeicCmd = &cobra.Command{
	Use:   "to-heic <input_dir>",
	Short: "Convert root-level images in a directory to HEIC in place",
	Long: `Convert decodable image files directly under input_dir to .heic in place.

Only regular files in the root of input_dir are considered. Existing target
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
	Args: cobra.ExactArgs(1),
	RunE: runToHeic,
}

var (
	toHeicDryRun bool
	toHeicWorkers int
)

func init() {
	toHeicCmd.Flags().BoolVar(&toHeicDryRun, "dry-run", false, "preview HEIC conversions without modifying files")
	toHeicCmd.Flags().IntVar(&toHeicWorkers, "workers", 2, "number of concurrent conversion workers (1–N; reduce to limit memory)")
	rootCmd.AddCommand(toHeicCmd)
}

func runToHeic(_ *cobra.Command, args []string) error {
	inputDir := args[0]

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

	if !toHeicDryRun {
		if err := heicconv.ValidateEncoderSupport(); err != nil {
			return err
		}
	}

	fmt.Printf("Input:   %s\n", inputDir)
	fmt.Printf("Workers: %d\n", toHeicWorkers)
	if toHeicDryRun {
		fmt.Println("Mode:    dry-run (no files will be modified)")
	}
	fmt.Println()

	stats, err := heicconv.Run(heicconv.Config{
		InputDir:     inputDir,
		DryRun:       toHeicDryRun,
		ShowProgress: true,
		Workers:      toHeicWorkers,
	})
	if err != nil {
		return err
	}

	fmt.Println()
	if toHeicDryRun {
		fmt.Println("Dry-run complete! (no files were modified)")
	} else {
		fmt.Println("HEIC conversion complete!")
	}
	fmt.Printf("  Root files scanned:   %d\n", stats.Scanned)
	if toHeicDryRun {
		fmt.Printf("  Planned conversions:  %d\n", stats.Planned)
	} else {
		fmt.Printf("  Converted:            %d\n", stats.Converted)
	}
	fmt.Printf("  Extension corrected:  %d\n", stats.RenamedExtensions)
	fmt.Printf("  Skipped (conflict):   %d\n", stats.SkippedConflicts)
	fmt.Printf("  Skipped (already HEIC): %d\n", stats.SkippedAlreadyHEIC)
	fmt.Printf("  Skipped (unsupported): %d\n", stats.SkippedUnsupported)
	fmt.Printf("  Failed:               %d\n", stats.Failed)

	return nil
}
