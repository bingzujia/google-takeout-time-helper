package screenshotter

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bingzujia/google-takeout-time-helper/internal/logutil"
)

// Config contains screenshotter command configuration
type Config struct {
	Dir    string
	DryRun bool
	Logger *logutil.Logger
}

// Stats contains execution statistics
type Stats struct {
	Renamed int
	Skipped int
	Errors  int
}

// Run executes the screenshot renaming process
// Now processes all files in the input directory (not just screenshots)
// Uses priority-based timestamp resolution: filename → modtime → skip
func Run(cfg Config) (Stats, error) {
	stats := Stats{}

	// Validate input directory
	info, err := os.Stat(cfg.Dir)
	if err != nil {
		return stats, fmt.Errorf("invalid input directory: %w", err)
	}
	if !info.IsDir() {
		return stats, fmt.Errorf("input path is not a directory")
	}

	// Read directory entries
	entries, err := os.ReadDir(cfg.Dir)
	if err != nil {
		return stats, fmt.Errorf("failed to read directory: %w", err)
	}

	// Process each entry in directory
	for _, entry := range entries {
		// Skip directories
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		originalPath := filepath.Join(cfg.Dir, name)

		// Resolve timestamp with priority: filename → modtime → fail
		timestamp, ok, source := ResolveTimestamp(originalPath, name)
		if !ok {
			stats.Skipped++
			cfg.Logger.Skip("rename-screenshot", name)
			continue
		}

		// Get extension
		ext := filepath.Ext(name)

		// Generate new name
		newName := generateNewName(timestamp, ext)

		// Resolve conflicts
		finalName := resolveConflict(cfg.Dir, newName)
		if finalName == "" {
			// All 999 suffixes occupied
			stats.Errors++
			cfg.Logger.Fail("rename-screenshot", name, "all conflict suffixes occupied")
			continue
		}
		finalPath := filepath.Join(cfg.Dir, finalName)

		// Perform rename
		if !cfg.DryRun {
			if err := os.Rename(originalPath, finalPath); err != nil {
				stats.Errors++
				cfg.Logger.Fail("rename-screenshot", name, fmt.Sprintf("rename error: %v", err))
				continue
			}
		}

		stats.Renamed++
		// Log with source indication
		if source != "" {
			cfg.Logger.Info("rename-screenshot", name+" → "+finalName+": from "+source)
		} else {
			cfg.Logger.Info("rename-screenshot", name+" → "+finalName)
		}
	}

	return stats, nil
}
