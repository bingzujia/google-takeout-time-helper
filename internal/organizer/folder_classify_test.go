package organizer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsYearFolder(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"Photos from 2024", true},
		{"Photos from 1999", true},
		{"Photos from 1850", true},
		{"Photos from 2024 (1)", false},
		{"My Photos", false},
		{"Photos from 202", false},
		{"Photos from 20245", false},
		{"photos from 2024", false},
		{"Photos from 1700", false},
		{"", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isYearFolder(tc.name)
			if got != tc.want {
				t.Errorf("isYearFolder(%q) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

func TestClassifyFolder(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test structure
	dirs := []string{
		"Photos from 2024",
		"Photos from 2023",
		"Album A",
		"Album B",
		"Random Folder",
	}
	for _, d := range dirs {
		if err := createDir(tmpDir, d); err != nil {
			t.Fatal(err)
		}
	}

	yearFolders, albumFolders, err := ClassifyFolder(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	if len(yearFolders) != 2 {
		t.Errorf("expected 2 year folders, got %d", len(yearFolders))
	}
	if len(albumFolders) != 3 {
		t.Errorf("expected 3 album folders, got %d", len(albumFolders))
	}
}

func TestClassifyFolder_NoYearFolder(t *testing.T) {
	tmpDir := t.TempDir()

	dirs := []string{"Album A", "Album B", "Random"}
	for _, d := range dirs {
		if err := createDir(tmpDir, d); err != nil {
			t.Fatal(err)
		}
	}

	yearFolders, albumFolders, err := ClassifyFolder(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	if len(yearFolders) != 0 {
		t.Errorf("expected 0 year folders, got %d", len(yearFolders))
	}
	if len(albumFolders) != 0 {
		t.Errorf("expected 0 album folders when no year folder exists, got %d", len(albumFolders))
	}
}

func createDir(parent, name string) error {
	return os.MkdirAll(filepath.Join(parent, name), 0755)
}
