package dedup

import (
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
)

func TestRun_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	result, err := Run(tmpDir, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalScanned != 0 {
		t.Errorf("expected 0 scanned, got %d", result.TotalScanned)
	}
}

func TestRun_NoDuplicates(t *testing.T) {
	tmpDir := t.TempDir()

	// Create two visually very different images
	createCheckerImage(t, filepath.Join(tmpDir, "a.jpg"), 8)
	createStripedImage(t, filepath.Join(tmpDir, "b.jpg"), 8)

	cfg := DefaultConfig()
	cfg.Threshold = 5
	result, err := Run(tmpDir, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalScanned != 2 {
		t.Errorf("expected 2 scanned, got %d", result.TotalScanned)
	}
	if result.TotalGroups != 0 {
		t.Errorf("expected 0 duplicate groups, got %d", result.TotalGroups)
	}
}

func TestRun_IdenticalDuplicates(t *testing.T) {
	tmpDir := t.TempDir()

	// Create three identical images
	createSolidImage(t, filepath.Join(tmpDir, "a.jpg"), color.RGBA{100, 100, 100, 255})
	createSolidImage(t, filepath.Join(tmpDir, "b.jpg"), color.RGBA{100, 100, 100, 255})
	createSolidImage(t, filepath.Join(tmpDir, "c.jpg"), color.RGBA{100, 100, 100, 255})

	result, err := Run(tmpDir, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalScanned != 3 {
		t.Errorf("expected 3 scanned, got %d", result.TotalScanned)
	}
	if result.TotalGroups != 1 {
		t.Errorf("expected 1 duplicate group, got %d", result.TotalGroups)
	}
	if result.TotalDupes != 2 {
		t.Errorf("expected 2 duplicates, got %d", result.TotalDupes)
	}
}

func TestRun_NonRecursive(t *testing.T) {
	tmpDir := t.TempDir()

	// Create image in root
	createSolidImage(t, filepath.Join(tmpDir, "a.jpg"), color.RGBA{50, 50, 50, 255})

	// Create identical image in subdirectory
	subDir := filepath.Join(tmpDir, "sub")
	os.MkdirAll(subDir, 0755)
	createSolidImage(t, filepath.Join(subDir, "b.jpg"), color.RGBA{50, 50, 50, 255})

	// Non-recursive should only find 1 image
	cfg := DefaultConfig()
	cfg.Recursive = false
	result, err := Run(tmpDir, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalScanned != 1 {
		t.Errorf("expected 1 scanned (non-recursive), got %d", result.TotalScanned)
	}
}

func TestCollectImagePaths_NonRecursive(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "sub")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	createSolidImage(t, filepath.Join(tmpDir, "a.jpg"), color.RGBA{10, 10, 10, 255})
	createSolidImage(t, filepath.Join(subDir, "b.jpg"), color.RGBA{20, 20, 20, 255})
	if err := os.WriteFile(filepath.Join(tmpDir, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	paths, err := collectImagePaths(tmpDir, false)
	if err != nil {
		t.Fatalf("collectImagePaths() error = %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("len(paths) = %d, want 1", len(paths))
	}
	if got := filepath.Base(paths[0]); got != "a.jpg" {
		t.Fatalf("paths[0] = %q, want a.jpg", got)
	}
}

func TestRun_CorruptImageCollectedAsError(t *testing.T) {
	tmpDir := t.TempDir()
	createSolidImage(t, filepath.Join(tmpDir, "ok.jpg"), color.RGBA{30, 30, 30, 255})
	if err := os.WriteFile(filepath.Join(tmpDir, "bad.jpg"), []byte("not an image"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := DefaultConfig()
	cfg.ShowProgress = false
	result, err := Run(tmpDir, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalScanned != 1 {
		t.Fatalf("TotalScanned = %d, want 1", result.TotalScanned)
	}
	if len(result.Errors) != 1 {
		t.Fatalf("len(Errors) = %d, want 1", len(result.Errors))
	}
	if filepath.Base(result.Errors[0].Path) != "bad.jpg" {
		t.Fatalf("error path = %q, want bad.jpg", result.Errors[0].Path)
	}
}

func createSolidImage(t *testing.T, path string, c color.Color) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for x := 0; x < 64; x++ {
		for y := 0; y < 64; y++ {
			img.Set(x, y, c)
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

// createCheckerImage creates a black/white checkerboard pattern.
func createCheckerImage(t *testing.T, path string, cellSize int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for x := 0; x < 64; x++ {
		for y := 0; y < 64; y++ {
			if (x/cellSize+y/cellSize)%2 == 0 {
				img.Set(x, y, color.White)
			} else {
				img.Set(x, y, color.Black)
			}
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

// createStripedImage creates horizontal black/white stripes.
func createStripedImage(t *testing.T, path string, stripeHeight int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for x := 0; x < 64; x++ {
		for y := 0; y < 64; y++ {
			if (y/stripeHeight)%2 == 0 {
				img.Set(x, y, color.White)
			} else {
				img.Set(x, y, color.Black)
			}
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
