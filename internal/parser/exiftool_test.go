package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseEXIFTimestamp(t *testing.T) {
	// Create a temp JPEG with known EXIF DateTimeOriginal
	// We use a real test image if available, or skip if exiftool can't read it.
	// For now, test that the function returns false for a non-image file.

	tmpDir := t.TempDir()
	txtFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(txtFile, []byte("not an image"), 0644); err != nil {
		t.Fatal(err)
	}

	_, ok := ParseEXIFTimestamp(txtFile)
	if ok {
		t.Error("expected false for non-image file")
	}

	// Test non-existent file
	_, ok = ParseEXIFTimestamp("/nonexistent/file.jpg")
	if ok {
		t.Error("expected false for non-existent file")
	}
}

func TestParseEXIFTimestamp_RealImage(t *testing.T) {
	// Try to find a real image in the testdata directory
	testFiles := []string{
		"testdata/test.jpg",
		"testdata/test.jpeg",
	}

	for _, f := range testFiles {
		if _, err := os.Stat(f); err != nil {
			continue
		}

		ts, ok := ParseEXIFTimestamp(f)
		if !ok {
			t.Logf("no EXIF DateTimeOriginal in %s (may be expected)", f)
			return
		}

		// Sanity check: EXIF date should be reasonable (1970-2100)
		if ts.Year() < 1970 || ts.Year() > 2100 {
			t.Errorf("EXIF timestamp %v out of reasonable range", ts)
		}
		return
	}

	t.Log("no testdata images found, skipping EXIF real-image test")
}
