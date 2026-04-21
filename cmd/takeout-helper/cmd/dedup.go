package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bingzujia/google-takeout-time-helper/internal/dedup"
	"github.com/bingzujia/google-takeout-time-helper/internal/logutil"
	"github.com/bingzujia/google-takeout-time-helper/internal/progress"
	"github.com/spf13/cobra"
)

var dedupCmd = &cobra.Command{
	Use:   "dedup",
	Short: "Find and group duplicate images in a directory",
	Long: `Scan the top-level image files in --input-dir and move each group of
duplicates into <input_dir>/dedup/group-001/, group-002/, etc.

Only immediate (non-recursive) contents of --input-dir are scanned.
Supported image formats: jpg, jpeg, png, gif, bmp, tiff, tif, webp, heic, heif.

Use --dry-run to preview what would be moved without touching any files.`,
	Args: cobra.NoArgs,
	RunE: runDedup,
}

var (
	dedupDryRun        bool
	dedupThreshold     int
	dedupNoCache       bool
	dedupCacheDir      string
	dedupMaxDecodeMB   int
	dedupDecodeWorkers int
	dedupInputDir      string
	dedupAuto          bool
	dedupConvertHEIC   bool
)

func init() {
	dedupCmd.Flags().BoolVar(&dedupDryRun, "dry-run", false, "preview duplicate groups without moving files")
	dedupCmd.Flags().IntVar(&dedupThreshold, "threshold", 10, "max perceptual hash distance to consider images as duplicates")
	dedupCmd.Flags().BoolVar(&dedupNoCache, "no-cache", false, "disable hash cache, always recompute hashes from disk")
	dedupCmd.Flags().StringVar(&dedupCacheDir, "cache-dir", "", "directory for hash cache DB (default: <input_dir>/.gtoh_cache)")
	dedupCmd.Flags().IntVar(&dedupMaxDecodeMB, "max-decode-mb", 500, "skip images larger than this size (MB) to prevent OOM")
	dedupCmd.Flags().IntVar(&dedupDecodeWorkers, "decode-workers", 0, "max concurrent image decodes (0 = unlimited)")
	dedupCmd.Flags().BoolVar(&dedupAuto, "auto", false, "automatic mode: keep largest file in root dir, all files in group-xxx/")
	dedupCmd.Flags().BoolVar(&dedupConvertHEIC, "convert-heic", true, "convert HEIC/HEIF files to JPEG for processing")
	dedupCmd.Flags().StringVar(&dedupInputDir, "input-dir", "", "input directory to scan for duplicates")
	_ = dedupCmd.MarkFlagRequired("input-dir")
	rootCmd.AddCommand(dedupCmd)
}

func runDedup(_ *cobra.Command, _ []string) error {
	inputDir := dedupInputDir

	// validate input directory
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
		Threshold:     dedupThreshold,
		Recursive:     false,
		DryRun:        dedupDryRun,
		ShowProgress:  true,
		NoCache:       dedupNoCache,
		CacheDir:      dedupCacheDir,
		MaxDecodeMB:   dedupMaxDecodeMB,
		DecodeWorkers: dedupDecodeWorkers,
		Auto:          dedupAuto,
		ConvertHEIC:   dedupConvertHEIC,
	}
	result, err := dedup.Run(inputDir, cfg)
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	logger, logErr := logutil.OpenLog(inputDir, "dedup", dedupDryRun)
	if logErr != nil {
		return fmt.Errorf("open log: %w", logErr)
	}
	defer logger.Close()

	// Task 3.3: print per-file warnings without stopping
	for _, fe := range result.Errors {
		progress.Warning("%s: %s", fe.Path, fe.Error)
	}

	if result.TotalGroups == 0 {
		fmt.Printf("Scanned %d image(s) — no duplicates found.\n", result.TotalScanned)
		return nil
	}

	// Output detailed group logging
	outputGroupLogging(result, inputDir, dedupAuto)

	// Tasks 2.2 + 2.3: move (or preview) each group
	dedupDir := filepath.Join(inputDir, "dedup")
	dedupAutoDir := filepath.Join(inputDir, "dedup-auto")
	totalMoved := 0

	for i, group := range result.Groups {
		groupName := fmt.Sprintf("group-%03d", i+1)

		for j, f := range group.Files {
			var dest string
			var groupDir string

			if dedupAuto {
				if j == group.Keep {
					// Kept file to root dedup-auto directory
					dest = filepath.Join(dedupAutoDir, filepath.Base(f.Path))
					groupDir = dedupAutoDir
				} else {
					// Non-kept file to group subdirectory
					groupDir = filepath.Join(dedupAutoDir, groupName)
					dest = filepath.Join(groupDir, filepath.Base(f.Path))
				}
			} else {
				// Standard mode: all files to group subdirectory under dedup
				groupDir = filepath.Join(dedupDir, groupName)
				dest = destPath(groupDir, filepath.Base(f.Path))
			}

			if !dedupDryRun {
				if err := os.MkdirAll(groupDir, 0755); err != nil {
					return fmt.Errorf("create group dir %s: %w", groupDir, err)
				}
				if err := os.Rename(f.Path, dest); err != nil {
					logger.Fail("move", f.Path, err.Error())
					return fmt.Errorf("move %s → %s: %w", f.Path, dest, err)
				}
				logger.Info(groupName, f.Path)
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
	if !dedupDryRun {
		fmt.Printf("  Log:              %s\n", logger.Path())
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

// outputGroupLogging outputs detailed log for each duplicate group.
// For standard mode (isAutoMode=false): all files → dedup/group-NNN/
// For auto mode (isAutoMode=true): kept file → dedup-auto/, others → dedup-auto/group-NNN/
func outputGroupLogging(result *dedup.Result, inputDir string, isAutoMode bool) {
	if result == nil || result.TotalGroups == 0 {
		return
	}

	// Handle nil Groups field
	if result.Groups == nil {
		return
	}

	for i, group := range result.Groups {
		// Skip empty or single-file groups
		if len(group.Files) < 2 {
			continue
		}

		// Generate group name (3-digit format)
		groupName := fmt.Sprintf("group-%03d", i+1)

		// Output group header
		fmt.Printf("[%s] %d duplicate file(s):\n", groupName, len(group.Files))

		// Iterate through each file
		for j, file := range group.Files {
			// Validate file path
			if file.Path == "" {
				fmt.Fprintf(os.Stderr, "WARNING: %s: empty file path at index %d\n", groupName, j)
				continue
			}

			filename := filepath.Base(file.Path)

			// Calculate destination path
			var destPath string
			if isAutoMode {
				if j == group.Keep {
					// Kept file to root directory
					destPath = filepath.Join(inputDir, "dedup-auto", filename)
				} else {
					// Non-kept files to subdirectory
					destPath = filepath.Join(inputDir, "dedup-auto", groupName, filename)
				}
			} else {
				// Standard mode: all files to group-xxx subdirectory
				destPath = filepath.Join(inputDir, "dedup", groupName, filename)
			}

			// Generate [KEEP] marker (only in auto mode)
			marker := ""
			if isAutoMode && j == group.Keep {
				marker = " [KEEP]"
			}

			// Output file line (special characters are output as-is, no escaping)
			fmt.Printf("  %s → %s%s\n", file.Path, destPath, marker)
		}

		// Empty line between groups
		fmt.Println()
	}
}

