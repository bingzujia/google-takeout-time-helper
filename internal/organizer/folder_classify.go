package organizer

import (
	"os"
	"path/filepath"
	"regexp"
)

var yearFolderRegex = regexp.MustCompile(`^Photos from (20|19|18)\d{2}$`)

// FolderClass holds the classification result for a directory.
type FolderClass struct {
	IsYearFolder  bool
	IsAlbumFolder bool
}

// ClassifyFolder scans the given directory and classifies its subdirectories.
func ClassifyFolder(rootDir string) (yearFolders []string, albumFolders []string, err error) {
	entries, err := os.ReadDir(rootDir)
	if err != nil {
		return nil, nil, err
	}

	// First pass: find all year folders
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if isYearFolder(e.Name()) {
			yearFolders = append(yearFolders, filepath.Join(rootDir, e.Name()))
		}
	}

	hasYearFolder := len(yearFolders) > 0

	// Second pass: find album folders (non-year folders whose parent has year folders)
	if hasYearFolder {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			if isYearFolder(e.Name()) {
				continue
			}
			albumFolders = append(albumFolders, filepath.Join(rootDir, e.Name()))
		}
	}

	return yearFolders, albumFolders, nil
}

// isYearFolder checks if a directory name matches "Photos from XXXX" pattern.
func isYearFolder(name string) bool {
	return yearFolderRegex.MatchString(name)
}
