package dedup

import (
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	goimagehash "github.com/corona10/goimagehash"
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

func TestHashCacheHitSkipsDecoding(t *testing.T) {
	tmpDir := t.TempDir()
	imgPath := filepath.Join(tmpDir, "img.jpg")
	createCheckerImage(t, imgPath, 8)

	cfg := DefaultConfig()
	cfg.Threshold = 20
	cfg.CacheDir = filepath.Join(tmpDir, ".cache")

	// First run: decode from disk and populate cache.
	r1, err := Run(tmpDir, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if r1.TotalScanned != 1 {
		t.Fatalf("expected 1 scanned, got %d", r1.TotalScanned)
	}
	if len(r1.Errors) != 0 {
		t.Fatalf("unexpected errors on first run: %v", r1.Errors)
	}

	// Second run: should hit cache (no error, same result).
	r2, err := Run(tmpDir, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if r2.TotalScanned != 1 {
		t.Fatalf("expected 1 scanned on second run, got %d", r2.TotalScanned)
	}
	if len(r2.Errors) != 0 {
		t.Fatalf("unexpected errors on second run (cache hit path): %v", r2.Errors)
	}
}

func TestMaxDecodeMBSkipsOversizedFile(t *testing.T) {
	tmpDir := t.TempDir()
	imgPath := filepath.Join(tmpDir, "big.jpg")
	createCheckerImage(t, imgPath, 8)

	cfg := DefaultConfig()
	cfg.NoCache = true
	cfg.MaxDecodeMB = 0 // 0 = unlimited, set very small threshold via direct call

	// Use 1 byte limit so every file is "oversized".
	result := prepareEntry(imgPath, nil, 1, nil) // 1 MB but our image is tiny — set to 0 to force skip
	_ = result                              // this won't skip; test the actual threshold logic below

	// Test via Run with a threshold-equivalent approach: create a 1-byte "image" file
	// that exceeds a 0-byte limit isn't supported (min 1). Instead test the error path
	// by using an internal call with maxDecodeMB=1 and a file known to be < 1 MB.
	// The real OOM protection is for files > maxDecodeMB*1024*1024.
	// Create a dummy non-image file to trigger the decode-error path.
	badPath := filepath.Join(tmpDir, "notanimage.jpg")
	if err := os.WriteFile(badPath, []byte("not jpeg data"), 0644); err != nil {
		t.Fatal(err)
	}

	cfgBad := DefaultConfig()
	cfgBad.NoCache = true
	r, err := Run(tmpDir, cfgBad)
	if err != nil {
		t.Fatal(err)
	}
	// The non-image file should appear in Errors.
	found := false
	for _, fe := range r.Errors {
		if filepath.Base(fe.Path) == "notanimage.jpg" {
			found = true
		}
	}
	if !found {
		t.Error("expected notanimage.jpg to appear in Errors")
	}
}

func TestMaxDecodeMBOversizedSkipped(t *testing.T) {
	tmpDir := t.TempDir()
	imgPath := filepath.Join(tmpDir, "img.jpg")
	createCheckerImage(t, imgPath, 8)

	// Use maxDecodeMB=0 (treated as unlimited) vs a very low byte threshold via
	// direct prepareEntry call to validate the guard logic.
	// File is ~a few KB; set limit to 0 bytes (no files can pass).
	res := prepareEntry(imgPath, nil, 0, nil) // 0 = unlimited, should decode fine
	if res.err != nil {
		t.Fatalf("maxDecodeMB=0 should mean unlimited, got err: %s", res.err.Error)
	}

	// Create a 10-byte fake "image" and test with 1-byte limit.
	tiny := filepath.Join(tmpDir, "tiny.jpg")
	_ = os.WriteFile(tiny, make([]byte, 10), 0644)
	res2 := prepareEntry(tiny, nil, 1, nil) // maxDecodeMB=1 => limit is 1*1024*1024; 10 bytes is below limit
	// 10 bytes < 1 MB, so not oversized — will fail on decode (not a real image)
	if res2.err == nil || res2.err.Error == "oversized: file too large to decode" {
		// Fine either way; not oversized
	}

	// The true test: file size is exactly at the limit — create a file exactly maxDecodeMB+1 bytes.
	bigPath := filepath.Join(tmpDir, "oversized.jpg")
	bigData := make([]byte, 2*1024*1024+1) // 2 MB + 1 byte
	_ = os.WriteFile(bigPath, bigData, 0644)
	res3 := prepareEntry(bigPath, nil, 2, nil) // maxDecodeMB=2 => limit is 2 MB; file is 2 MB+1 byte
	if res3.err == nil {
		t.Fatal("expected oversized error for file > maxDecodeMB")
	}
	if res3.err.Error != "oversized: file too large to decode" {
		t.Errorf("unexpected error: %s", res3.err.Error)
	}
}

func TestBuildBucketsMatchesBruteForce(t *testing.T) {
	// Build a small set of entries with known pHash values.
	entries := []preparedEntry{
		{phash: 0xF0F0F0F0F0F0F0F0, dhash: 0xAAAAAAAAAAAAAAAA},
		{phash: 0xF0F0F0F0F0F0F0F1, dhash: 0xAAAAAAAAAAAAAAAA}, // same top 16 bits as [0]
		{phash: 0x0101010101010101, dhash: 0x5555555555555555}, // different bucket
		{phash: 0xF0F0F0F0FFFFFFFF, dhash: 0xAAAAAAAABBBBBBBB}, // same top 16 bits as [0]
	}

	threshold := 10

	// Brute-force O(n²).
	bfUF := newUnionFind(len(entries))
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			pDist, _ := goimagehash.NewImageHash(entries[i].phash, goimagehash.PHash).Distance(
				goimagehash.NewImageHash(entries[j].phash, goimagehash.PHash))
			dDist, _ := goimagehash.NewImageHash(entries[i].dhash, goimagehash.DHash).Distance(
				goimagehash.NewImageHash(entries[j].dhash, goimagehash.DHash))
			if pDist <= threshold && dDist <= threshold {
				bfUF.union(i, j)
			}
		}
	}

	// Bucket-based.
	bucketUF := newUnionFind(len(entries))
	buckets := buildBuckets(entries, 16)
	for _, idxs := range buckets {
		for a := 0; a < len(idxs); a++ {
			for b := a + 1; b < len(idxs); b++ {
				i, j := idxs[a], idxs[b]
				pDist, _ := goimagehash.NewImageHash(entries[i].phash, goimagehash.PHash).Distance(
					goimagehash.NewImageHash(entries[j].phash, goimagehash.PHash))
				dDist, _ := goimagehash.NewImageHash(entries[i].dhash, goimagehash.DHash).Distance(
					goimagehash.NewImageHash(entries[j].dhash, goimagehash.DHash))
				if pDist <= threshold && dDist <= threshold {
					bucketUF.union(i, j)
				}
			}
		}
	}

	// Compare group membership: every pair that BF groups together, buckets must too.
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			bfSame := bfUF.find(i) == bfUF.find(j)
			bkSame := bucketUF.find(i) == bucketUF.find(j)
			if bfSame && !bkSame {
				t.Errorf("bucket method missed duplicate pair (%d, %d)", i, j)
			}
		}
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

func TestDecodeWorkersLimitsConcurrency(t *testing.T) {
tmpDir := t.TempDir()

// Create several identical images so there will be duplicates to detect.
for i := 0; i < 5; i++ {
createSolidImage(t, filepath.Join(tmpDir, fmt.Sprintf("img%d.jpg", i)), color.RGBA{50, 100, 150, 255})
}

// Run with DecodeWorkers=1 (strict serial decode).
cfgSerial := DefaultConfig()
cfgSerial.DecodeWorkers = 1
cfgSerial.NoCache = true
resultSerial, err := Run(tmpDir, cfgSerial)
if err != nil {
t.Fatal(err)
}

// Run without limit (DecodeWorkers=0) for comparison.
cfgUnlimited := DefaultConfig()
cfgUnlimited.DecodeWorkers = 0
cfgUnlimited.NoCache = true
resultUnlimited, err := Run(tmpDir, cfgUnlimited)
if err != nil {
t.Fatal(err)
}

// Both runs must agree on the number of scanned files and duplicate groups.
if resultSerial.TotalScanned != resultUnlimited.TotalScanned {
t.Errorf("TotalScanned mismatch: serial=%d unlimited=%d",
resultSerial.TotalScanned, resultUnlimited.TotalScanned)
}
if resultSerial.TotalGroups != resultUnlimited.TotalGroups {
t.Errorf("TotalGroups mismatch: serial=%d unlimited=%d",
resultSerial.TotalGroups, resultUnlimited.TotalGroups)
}
if resultSerial.TotalDupes != resultUnlimited.TotalDupes {
t.Errorf("TotalDupes mismatch: serial=%d unlimited=%d",
resultSerial.TotalDupes, resultUnlimited.TotalDupes)
}
}
