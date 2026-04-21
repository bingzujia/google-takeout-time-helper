package qqmedia

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/bingzujia/google-takeout-time-helper/internal/logutil"
	"github.com/bingzujia/google-takeout-time-helper/internal/workerpool"
)

// Config holds configuration for the format-qq-media operation
type Config struct {
	Dir    string         // Input directory path
	DryRun bool           // Preview mode (no file modifications)
	Logger *logutil.Logger // Logger instance (pointer)
}

// Stats holds statistics about the operation
type Stats struct {
	Renamed int // Successfully renamed files
	Skipped int // Skipped files (no timestamp, unsupported type)
	Errors  int // Failed operations
}

// Run processes all media files in the directory
func Run(cfg Config) (Stats, error) {
	stats := Stats{}
	statsMutex := &sync.Mutex{}

	// Validate input directory
	info, err := os.Stat(cfg.Dir)
	if err != nil {
		return stats, fmt.Errorf("cannot access directory: %w", err)
	}
	if !info.IsDir() {
		return stats, fmt.Errorf("path is not a directory: %s", cfg.Dir)
	}

	// Scan directory (non-recursive)
	entries, err := os.ReadDir(cfg.Dir)
	if err != nil {
		return stats, fmt.Errorf("failed to read directory: %w", err)
	}

	// Filter to files only (exclude directories)
	var files []os.DirEntry
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, entry)
		}
	}

	// Process files with worker pool
	numWorkers := min(len(files), getMaxWorkers())
	if numWorkers < 1 {
		numWorkers = 1
	}

	type job struct {
		file os.DirEntry
		path string
	}

	jobs := make([]job, len(files))
	for i, file := range files {
		jobs[i] = job{file: file, path: filepath.Join(cfg.Dir, file.Name())}
	}

	err = workerpool.Run(jobs, numWorkers, func(j job) error {
		if err := processFile(j.path, cfg, &stats, statsMutex); err != nil {
			cfg.Logger.Fail(j.file.Name(), "process error", fmt.Sprintf("%v", err))
			statsMutex.Lock()
			stats.Errors++
			statsMutex.Unlock()
		}
		return nil
	})

	if err != nil {
		return stats, err
	}

	return stats, nil
}

// processFile handles a single file
func processFile(filePath string, cfg Config, stats *Stats, mutex *sync.Mutex) error {
	fileName := filepath.Base(filePath)

	// Detect media type
	mediaType, err := DetectMediaType(filePath)
	if err != nil {
		cfg.Logger.Skip("unsupported media type", fileName)
		mutex.Lock()
		stats.Skipped++
		mutex.Unlock()
		return nil
	}

	// Skip unsupported types
	if mediaType == "" {
		cfg.Logger.Skip("unsupported media type", fileName)
		mutex.Lock()
		stats.Skipped++
		mutex.Unlock()
		return nil
	}

	// Parse timestamp from filename
	timestamp, err := ParseTimestamp(fileName)
	if err != nil {
		cfg.Logger.Skip("timestamp parse error", fileName)
		mutex.Lock()
		stats.Skipped++
		mutex.Unlock()
		return nil
	}

	// Fallback to mtime if no timestamp found
	if timestamp == 0 {
		timestamp, err = GetFileModTime(filePath)
		if err != nil {
			cfg.Logger.Fail("mtime lookup error", fileName, fmt.Sprintf("%v", err))
			mutex.Lock()
			stats.Errors++
			mutex.Unlock()
			return nil
		}
	}

	// Generate new filename
	ext := filepath.Ext(fileName)
	newName := GenerateNewName(mediaType, timestamp, ext)

	// Resolve conflicts
	finalName, err := ResolveConflict(newName, cfg.Dir)
	if err != nil {
		cfg.Logger.Fail("conflict error", fileName, fmt.Sprintf("%v", err))
		mutex.Lock()
		stats.Errors++
		mutex.Unlock()
		return nil
	}

	// Log the operation
	if cfg.DryRun {
		cfg.Logger.Info(fmt.Sprintf("%s → %s", fileName, finalName), "")
		mutex.Lock()
		stats.Renamed++
		mutex.Unlock()
	} else {
		// Rename file
		newPath := filepath.Join(cfg.Dir, finalName)
		if err := os.Rename(filePath, newPath); err != nil {
			cfg.Logger.Fail("rename error", fileName, fmt.Sprintf("%v", err))
			mutex.Lock()
			stats.Errors++
			mutex.Unlock()
			return nil
		}

		cfg.Logger.Info(fmt.Sprintf("%s → %s", fileName, finalName), "")
		mutex.Lock()
		stats.Renamed++
		mutex.Unlock()
	}

	return nil
}

// getMaxWorkers returns the optimal number of workers
func getMaxWorkers() int {
	max := 8
	// Try to get a reasonable number of workers
	return max
}

// min returns the smaller of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
