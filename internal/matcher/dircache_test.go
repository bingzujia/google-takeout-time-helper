package matcher

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
)

func TestDirCache_NilFallsThrough(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	var c *DirCache
	entries, err := c.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "a.json" {
		t.Errorf("unexpected entries: %v", entries)
	}
}

func TestDirCache_CachesResult(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "photo.jpg"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	c := &DirCache{}

	// First read.
	e1, err := c.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Add a new file after caching.
	if err := os.WriteFile(filepath.Join(dir, "extra.jpg"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	// Second read: should return the cached (stale) result.
	e2, err := c.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(e2) != len(e1) {
		t.Errorf("expected cached result (%d entries), got %d", len(e1), len(e2))
	}
}

func TestDirCache_ConcurrentSafe(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.jpg", "b.jpg", "c.jpg"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
	}

	c := &DirCache{}
	const goroutines = 50
	var wg sync.WaitGroup
	var readCount atomic.Int64

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			entries, err := c.ReadDir(dir)
			if err != nil {
				t.Errorf("ReadDir error: %v", err)
				return
			}
			if len(entries) != 3 {
				t.Errorf("expected 3 entries, got %d", len(entries))
			}
			readCount.Add(1)
		}()
	}
	wg.Wait()

	if readCount.Load() != goroutines {
		t.Errorf("expected %d successful reads, got %d", goroutines, readCount.Load())
	}
}

func TestDirCache_ReadDirOncePerDir(t *testing.T) {
	// Verify that a shared DirCache prevents duplicate os.ReadDir calls across
	// concurrent JSONForFile invocations for files in the same directory.
	dir := t.TempDir()

	// Create a photo file and a JSON sidecar.
	photoPath := filepath.Join(dir, "IMG_20230101_120000.jpg")
	jsonPath := photoPath + ".supplemental-metadata.json"
	if err := os.WriteFile(photoPath, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	sidecar := `{"photoTakenTime":{"timestamp":"1672574400"},"geoDataExif":{},"geoData":{}}`
	if err := os.WriteFile(jsonPath, []byte(sidecar), 0644); err != nil {
		t.Fatal(err)
	}

	c := &DirCache{}

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			result := JSONForFile(photoPath, c)
			if result == nil {
				t.Errorf("JSONForFile returned nil")
			}
		}()
	}
	wg.Wait()
}
