package screenshotter

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// generateNewName generates standardized screenshot filename
// Format: Screenshot_YYYY-MM-DD-HH-MM-SS-MS.ext
// where MS is 2-digit zero-padded milliseconds (0-99)
func generateNewName(timestamp time.Time, extension string) string {
	year, month, day := timestamp.Date()
	hour, min, sec := timestamp.Hour(), timestamp.Minute(), timestamp.Second()
	// Convert nanoseconds to 2-digit milliseconds (0-99)
	// Each step of 10_000_000 ns = 1 unit of 0-99 ms
	ms := (timestamp.Nanosecond() / 10_000_000) % 100

	baseName := fmt.Sprintf("Screenshot_%04d-%02d-%02d-%02d-%02d-%02d-%02d",
		year, month, day, hour, min, sec, ms)

	if extension != "" {
		return baseName + extension
	}
	return baseName
}

// resolveConflict handles filename conflicts by appending _001, _002, ..., _999
func resolveConflict(dir, targetName string) string {
	fullPath := filepath.Join(dir, targetName)

	// Check if target exists
	if _, err := os.Stat(fullPath); err == nil {
		// File exists, need to add suffix
		return addSuffix(dir, targetName)
	}

	// Doesn't exist, return as-is
	return targetName
}

// addSuffix appends numeric suffix before extension (up to _999)
func addSuffix(dir, targetName string) string {
	ext := filepath.Ext(targetName)
	base := targetName[:len(targetName)-len(ext)]

	for i := 1; i <= 999; i++ {
		newName := fmt.Sprintf("%s_%03d%s", base, i, ext)
		fullPath := filepath.Join(dir, newName)
		if _, err := os.Stat(fullPath); err != nil {
			// File doesn't exist, use this name
			return newName
		}
	}

	// Fallback: all 999 suffixes occupied, return empty string (error case)
	return ""
}
