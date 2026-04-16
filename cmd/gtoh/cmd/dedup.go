package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bingzujia/g_photo_take_out_helper/internal/dedup"
	"github.com/bingzujia/g_photo_take_out_helper/internal/progress"
	"github.com/spf13/cobra"
)

var dedupCmd = &cobra.Command{
	Use:   "dedup <input_dir>",
	Short: "Find and group duplicate images in a directory",
	Long: `Scan the top-level image files in <input_dir> and move each group of
duplicates into <input_dir>/dedup/group-001/, group-002/, etc.

Only immediate (non-recursive) contents of <input_dir> are scanned.
Supported image formats: jpg, jpeg, png, gif, bmp, tiff, tif, webp, heic, heif.

Use --dry-run to preview what would be moved without touching any files.`,
	Args: cobra.ExactArgs(1),
	RunE: runDedup,
}

var (
	dedupDryRun    bool
	dedupThreshold int
)

func init() {
	dedupCmd.Flags().BoolVar(&dedupDryRun, "dry-run", false, "preview duplicate groups without moving files")
	dedupCmd.Flags().IntVar(&dedupThreshold, "threshold", 10, "max perceptual hash distance to consider images as duplicates")
	rootCmd.AddCommand(dedupCmd)
}

func runDedup(_ *cobra.Command, args []string) error {
	inputDir := args[0]

	// Task 3.2: validate input directory
	if _, err := os.Stat(inputDir); os.IsNotExist(err) {
		return fmt.Errorf("input directory does not exist: %s", inputDir)
	}

	fmt.Printf("Input:     %s\n", inputDir)
	fmt.Printf("Threshold: %d\n", dedupThreshold)
	if dedupDryRun {
		fmt.Println("Mode:      dry-run (no files will be moved)")
	} else {
		fmt.Println("Mode:      move")
	}
	fmt.Println()

	// Task 2.1: call dedup.Run with Recursive: false (top-level only)
	cfg := dedup.Config{
		Threshold:    dedupThreshold,
		Recursive:    false,
		DryRun:       dedupDryRun,
		ShowProgress: true,
	}
	result, err := dedup.Run(inputDir, cfg)
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	// Task 3.3: print per-file warnings without stopping
	for _, fe := range result.Errors {
		progress.Warning("%s: %s", fe.Path, fe.Error)
	}

	if result.TotalGroups == 0 {
		fmt.Printf("Scanned %d image(s) — no duplicates found.\n", result.TotalScanned)
		return nil
	}

	// Tasks 2.2 + 2.3: move (or preview) each group
	dedupDir := filepath.Join(inputDir, "dedup")
	totalMoved := 0

	for i, group := range result.Groups {
		groupName := fmt.Sprintf("group-%03d", i+1)
		groupDir := filepath.Join(dedupDir, groupName)

		fmt.Printf("[%s] %d duplicate file(s):\n", groupName, len(group.Files))
		for _, f := range group.Files {
			dest := destPath(groupDir, filepath.Base(f.Path))
			fmt.Printf("  %s → %s\n", f.Path, dest)

			if !dedupDryRun {
				if err := os.MkdirAll(groupDir, 0755); err != nil {
					return fmt.Errorf("create group dir %s: %w", groupDir, err)
				}
				if err := os.Rename(f.Path, dest); err != nil {
					return fmt.Errorf("move %s → %s: %w", f.Path, dest, err)
				}
				totalMoved++
			}
		}
	}

	// Task 3.1: print summary
	fmt.Println()
	if dedupDryRun {
		fmt.Println("Dry-run complete! (no files were moved)")
	} else {
		fmt.Println("Dedup complete!")
	}
	fmt.Printf("  Images scanned:   %d\n", result.TotalScanned)
	fmt.Printf("  Duplicate groups: %d\n", result.TotalGroups)
	if dedupDryRun {
		fmt.Printf("  Would move:       %d file(s)\n", result.TotalDupes+result.TotalGroups)
	} else {
		fmt.Printf("  Files moved:      %d\n", totalMoved)
	}

	return nil
}

// destPath returns a destination path under dir for a file named base,
// appending _1, _2, … suffixes to avoid overwriting an existing file.
func destPath(dir, base string) string {
	candidate := filepath.Join(dir, base)
	if _, err := os.Stat(candidate); os.IsNotExist(err) {
		return candidate
	}
	ext := filepath.Ext(base)
	name := base[:len(base)-len(ext)]
	for i := 1; ; i++ {
		candidate = filepath.Join(dir, fmt.Sprintf("%s_%d%s", name, i, ext))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}
