package exifrunner

import (
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
)

// writeTestJPEG writes a minimal valid JPEG to path.
func writeTestJPEG(t *testing.T, path string) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for x := 0; x < 4; x++ {
		for y := 0; y < 4; y++ {
			img.Set(x, y, color.White)
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := jpeg.Encode(f, img, nil); err != nil {
		t.Fatal(err)
	}
}

func TestBatchQueryEmptyPaths(t *testing.T) {
	result, err := BatchQuery(nil, []string{"Make"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d", len(result))
	}
}

func TestBatchQueryNoExiftool(t *testing.T) {
	// Force LookupPath to run so we know the current state.
	_, found := LookupPath()
	if found {
		// Can't simulate "not found" when exiftool is installed.
		// Verify instead that BatchQuery still works with real exiftool.
		t.Skip("exiftool is installed; skipping unavailable-exiftool simulation")
	}

	// When exiftool is not installed, BatchQuery should return empty maps without error.
	paths := []string{"/nonexistent.jpg"}
	results, err := BatchQuery(paths, []string{"Make"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0] == nil {
		t.Error("expected non-nil empty map")
	}
}

func TestBatchQuerySingleFile(t *testing.T) {
	_, ok := LookupPath()
	if !ok {
		t.Skip("exiftool not installed")
	}

	dir := t.TempDir()
	imgPath := filepath.Join(dir, "test.jpg")
	writeTestJPEG(t, imgPath)

	results, err := BatchQuery([]string{imgPath}, []string{"Make", "Model"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	// A tiny synthesized JPEG has no Make/Model; just verify the map is non-nil.
	if results[0] == nil {
		t.Error("expected non-nil result map")
	}
}

func TestBatchQueryMultipleFiles(t *testing.T) {
	_, ok := LookupPath()
	if !ok {
		t.Skip("exiftool not installed")
	}

	dir := t.TempDir()
	paths := make([]string, 3)
	for i := range paths {
		paths[i] = filepath.Join(dir, "img"+string(rune('0'+i))+".jpg")
		writeTestJPEG(t, paths[i])
	}

	results, err := BatchQuery(paths, []string{"Make"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != len(paths) {
		t.Fatalf("expected %d results, got %d", len(paths), len(results))
	}
	for i, m := range results {
		if m == nil {
			t.Errorf("result[%d] is nil", i)
		}
	}
}

func TestBatchQueryMissingTag(t *testing.T) {
	_, ok := LookupPath()
	if !ok {
		t.Skip("exiftool not installed")
	}

	dir := t.TempDir()
	imgPath := filepath.Join(dir, "test.jpg")
	writeTestJPEG(t, imgPath)

	results, err := BatchQuery([]string{imgPath}, []string{"GPSLatitude"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	// Plain JPEG has no GPS; the key should be absent.
	if _, found := results[0]["GPSLatitude"]; found {
		t.Error("expected GPSLatitude to be absent for a plain JPEG")
	}
}
