package qqmedia

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GenerateNewName creates a standardized filename for QQ media
// Format: {Image|Video}_{unix-ms}.{ext}
func GenerateNewName(mediaType string, timestamp int64, sourceExt string) string {
	prefix := "Image_"
	if mediaType == "video" {
		prefix = "Video_"
	}

	filename := fmt.Sprintf("%s%d", prefix, timestamp)
	if sourceExt != "" {
		// Ensure extension starts with dot
		if !strings.HasPrefix(sourceExt, ".") {
			sourceExt = "." + sourceExt
		}
		filename = filename + sourceExt
	}

	return filename
}

// ResolveConflict handles file naming conflicts by appending numeric suffixes
// Returns the final filename to use (or original if no conflict)
func ResolveConflict(targetName string, dir string) (string, error) {
	targetPath := filepath.Join(dir, targetName)

	// Check if target already exists
	_, err := os.Stat(targetPath)
	if err == nil {
		// File exists, need to resolve conflict
		return appendConflictSuffix(targetName, dir)
	}

	if !os.IsNotExist(err) {
		// Some other error occurred
		return "", err
	}

	// File doesn't exist, no conflict
	return targetName, nil
}

// appendConflictSuffix tries numeric suffixes until finding an available name
func appendConflictSuffix(targetName string, dir string) (string, error) {
	// Split filename and extension
	ext := filepath.Ext(targetName)
	nameWithoutExt := strings.TrimSuffix(targetName, ext)

	// Try suffixes from _001 to _999
	for i := 1; i <= 999; i++ {
		suffixedName := fmt.Sprintf("%s_%03d%s", nameWithoutExt, i, ext)
		suffixedPath := filepath.Join(dir, suffixedName)

		_, err := os.Stat(suffixedPath)
		if err != nil && os.IsNotExist(err) {
			// Found an available name
			return suffixedName, nil
		} else if err != nil {
			// Some other error
			return "", err
		}
		// File exists, continue to next suffix
	}

	// All 999 suffixes are taken, return error
	return "", fmt.Errorf("unable to resolve filename conflict after 999 attempts: %s", targetName)
}
