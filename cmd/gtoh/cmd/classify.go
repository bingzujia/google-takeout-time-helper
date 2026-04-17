package cmd

import (
	"fmt"
	"os"

	"github.com/bingzujia/g_photo_take_out_helper/internal/classifier"
	"github.com/bingzujia/g_photo_take_out_helper/internal/logutil"
	"github.com/spf13/cobra"
)

var classifyCmd = &cobra.Command{
	Use:   "classify",
	Short: "Classify media files into camera, screenshot, wechat, seemsCamera",
	Long: `Classify media files from the root of --input-dir.

Files are moved into subdirectories of --output-dir:
  camera/      — filename matches known camera patterns (IMG_, VID_, PXL_, etc.)
  screenshot/  — filename contains "screenshot"
  wechat/      — filename starts with "mmexport"
  seemsCamera/ — no filename match, but exiftool detects camera Make/Model

Files that do not match any rule are left in place (counted as skipped).`,
	Args: cobra.NoArgs,
	RunE: runClassify,
}

var (
	classifyDryRun   bool
	classifyInputDir string
	classifyOutputDir string
)

func init() {
	classifyCmd.Flags().BoolVar(&classifyDryRun, "dry-run", false, "preview classification without moving files")
	classifyCmd.Flags().StringVar(&classifyInputDir, "input-dir", "", "input directory containing media files to classify")
	classifyCmd.Flags().StringVar(&classifyOutputDir, "output-dir", "", "output directory for classified subdirectories")
	_ = classifyCmd.MarkFlagRequired("input-dir")
	_ = classifyCmd.MarkFlagRequired("output-dir")
	rootCmd.AddCommand(classifyCmd)
}

func runClassify(_ *cobra.Command, _ []string) error {
	inputDir := classifyInputDir
	outputDir := classifyOutputDir

	if _, err := os.Stat(inputDir); os.IsNotExist(err) {
		return fmt.Errorf("input directory does not exist: %s", inputDir)
	}

	fmt.Printf("Input:  %s\n", inputDir)
	fmt.Printf("Output: %s\n", outputDir)

	if classifyDryRun {
		fmt.Println("\nDry-run mode — no files will be moved")
	} else {
		fmt.Println()
	}

	logger, err := logutil.OpenLog(outputDir, "classify", classifyDryRun)
	if err != nil {
		return fmt.Errorf("open log: %w", err)
	}
	defer logger.Close()

	result, err := classifier.Run(classifier.Config{
		InputDir:     inputDir,
		OutputDir:    outputDir,
		DryRun:       classifyDryRun,
		ShowProgress: true,
		Logger:       logger,
	})
	if err != nil {
		return err
	}

	fmt.Println()
	if classifyDryRun {
		fmt.Println("Dry-run complete! (no files were modified)")
	} else {
		fmt.Println("Classification complete!")
	}
	fmt.Printf("  Camera:       %d files\n", result.Camera)
	fmt.Printf("  Screenshot:   %d files\n", result.Screenshot)
	fmt.Printf("  WeChat:       %d files\n", result.Wechat)
	fmt.Printf("  SeemsCamera:  %d files\n", result.SeemsCamera)
	fmt.Printf("  Skipped:      %d files\n", result.Skipped)
	if !classifyDryRun {
		fmt.Printf("  Log:          %s\n", logger.Path())
	}

	return nil
}
