package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/bingzujia/g_photo_take_out_helper/internal/cleaner"
	"github.com/bingzujia/g_photo_take_out_helper/internal/progress"
	"github.com/spf13/cobra"
)

var cleanJSONCmd = &cobra.Command{
	Use:   "clean-json",
	Short: "Delete all JSON sidecar files recursively",
	RunE:  runCleanJSON,
}

var cleanJSONDir string
var cleanJSONDryRun bool
var cleanJSONYes bool

func init() {
	cleanJSONCmd.Flags().StringVar(&cleanJSONDir, "dir", ".", "target directory")
	cleanJSONCmd.Flags().BoolVar(&cleanJSONDryRun, "dry-run", false, "preview count only")
	cleanJSONCmd.Flags().BoolVar(&cleanJSONYes, "yes", false, "skip confirmation prompt")
}

func runCleanJSON(_ *cobra.Command, _ []string) error {
	if cleanJSONDryRun {
		result, err := cleaner.Run(cleaner.Config{Dir: cleanJSONDir, DryRun: true})
		if err != nil {
			return err
		}
		progress.Info("Would delete %d JSON file(s)", result.Deleted)
		return nil
	}

	if !cleanJSONYes {
		fmt.Printf("Delete all JSON files under %q? [y/N]: ", cleanJSONDir)
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
		if answer != "y" && answer != "yes" {
			progress.Info("Aborted.")
			return nil
		}
	}

	result, err := cleaner.Run(cleaner.Config{Dir: cleanJSONDir, DryRun: false})
	if err != nil {
		return err
	}

	progress.Success("Done. Deleted: %d, Failed: %d", result.Deleted, result.Failed)
	return nil
}
