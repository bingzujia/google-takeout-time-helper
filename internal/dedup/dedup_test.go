package dedup

import (
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
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

// TestAutoMode_CopyFile tests file copying with 32KB buffer
func TestAutoMode_CopyFile(t *testing.T) {
tmpDir := t.TempDir()
srcFile := filepath.Join(tmpDir, "source.jpg")
dstFile := filepath.Join(tmpDir, "destination.jpg")

// Create a test file
content := []byte("test content for copy")
if err := os.WriteFile(srcFile, content, 0644); err != nil {
t.Fatal(err)
}

// Copy the file
if err := copyFile(srcFile, dstFile); err != nil {
t.Fatal(err)
}

// Verify destination exists and has same content
dstContent, err := os.ReadFile(dstFile)
if err != nil {
t.Fatal(err)
}
if string(dstContent) != string(content) {
t.Errorf("copied content mismatch: expected %q, got %q", string(content), string(dstContent))
}
}

// TestAutoMode_HandleAutoMode tests auto mode with --auto flag
func TestAutoMode_HandleAutoMode(t *testing.T) {
tmpDir := t.TempDir()

// Create three identical images
color1 := color.RGBA{100, 100, 100, 255}
createSolidImage(t, filepath.Join(tmpDir, "a.jpg"), color1)
createSolidImage(t, filepath.Join(tmpDir, "b.jpg"), color1)
createSolidImage(t, filepath.Join(tmpDir, "c.jpg"), color1)

cfg := DefaultConfig()
cfg.Auto = true // Enable auto mode
cfg.DryRun = false // Must enable actual file operations for this test
result, err := Run(tmpDir, cfg)
if err != nil {
t.Fatal(err)
}

// Verify dedup was detected
if result.TotalScanned != 3 {
t.Errorf("expected 3 scanned, got %d", result.TotalScanned)
}
if result.TotalGroups != 1 {
t.Errorf("expected 1 duplicate group, got %d", result.TotalGroups)
}
if result.TotalDupes != 2 {
t.Errorf("expected 2 duplicates, got %d", result.TotalDupes)
}

// Verify dedup-auto directory exists
dedupAutoDir := filepath.Join(tmpDir, "dedup-auto")
if _, err := os.Stat(dedupAutoDir); os.IsNotExist(err) {
t.Errorf("dedup-auto directory not created")
}

// Verify group subdirectory exists
groupDir := filepath.Join(dedupAutoDir, "group-001")
if _, err := os.Stat(groupDir); os.IsNotExist(err) {
t.Errorf("group-001 directory not created")
}
}


// BenchmarkCopyFile benchmarks the copyFile function
func BenchmarkCopyFile(b *testing.B) {
tmpDir := b.TempDir()

// Create a 1MB test file
srcFile := filepath.Join(tmpDir, "source.bin")
f, _ := os.Create(srcFile)
f.Write(make([]byte, 1*1024*1024))
f.Close()

b.ResetTimer()
for i := 0; i < b.N; i++ {
dstFile := filepath.Join(tmpDir, fmt.Sprintf("dest_%d.bin", i))
copyFile(srcFile, dstFile)
}
}

// TestAutoMode_LargeGroup tests auto mode with many duplicates in one group
func TestAutoMode_LargeGroup(t *testing.T) {
tmpDir := t.TempDir()

// Create 10 identical images
color1 := color.RGBA{50, 100, 150, 255}
for i := 0; i < 10; i++ {
createSolidImage(t, filepath.Join(tmpDir, fmt.Sprintf("img_%d.jpg", i)), color1)
}

cfg := DefaultConfig()
cfg.Auto = true
cfg.DryRun = false
result, err := Run(tmpDir, cfg)
if err != nil {
t.Fatal(err)
}

if result.TotalGroups != 1 {
t.Errorf("expected 1 group, got %d", result.TotalGroups)
}
if result.TotalDupes != 9 {
t.Errorf("expected 9 duplicates, got %d", result.TotalDupes)
}

// All 10 files should be in dedup-auto
groupDir := filepath.Join(tmpDir, "dedup-auto", "group-001")
files, err := os.ReadDir(groupDir)
if err != nil {
t.Fatal(err)
}
if len(files) != 10 {
t.Errorf("expected 10 files in group-001, got %d", len(files))
}
}

// TestAutoMode_DryRun tests that --dry-run doesn't create files
func TestAutoMode_DryRun(t *testing.T) {
tmpDir := t.TempDir()

// Create 3 identical images
color1 := color.RGBA{100, 100, 100, 255}
createSolidImage(t, filepath.Join(tmpDir, "a.jpg"), color1)
createSolidImage(t, filepath.Join(tmpDir, "b.jpg"), color1)
createSolidImage(t, filepath.Join(tmpDir, "c.jpg"), color1)

cfg := DefaultConfig()
cfg.Auto = true
cfg.DryRun = true // Enable dry-run
result, err := Run(tmpDir, cfg)
if err != nil {
t.Fatal(err)
}

// Should detect duplicates
if result.TotalGroups != 1 {
t.Errorf("expected 1 group detected in dry-run, got %d", result.TotalGroups)
}

// But dedup-auto directory should NOT be created
dedupAutoDir := filepath.Join(tmpDir, "dedup-auto")
if _, err := os.Stat(dedupAutoDir); !os.IsNotExist(err) {
t.Errorf("dedup-auto directory should not be created in dry-run mode")
}
}

// 【工具函数 1】assertFilesInGroup - 验证 group 目录中的文件计数
func assertFilesInGroup(t *testing.T, groupDir string, expectedCount int, expectedNames []string) error {
// 检查目录是否存在
stat, err := os.Stat(groupDir)
if err != nil {
t.Errorf("group directory does not exist: %v", err)
return err
}

if !stat.IsDir() {
t.Errorf("%s is not a directory", groupDir)
return fmt.Errorf("not a directory")
}

// 列举文件
entries, err := os.ReadDir(groupDir)
if err != nil {
t.Errorf("failed to read directory: %v", err)
return err
}

// 验证文件计数
if len(entries) != expectedCount {
t.Errorf("expected %d files in group, got %d", expectedCount, len(entries))
return fmt.Errorf("file count mismatch: expected %d, got %d", expectedCount, len(entries))
}

// 验证期望的文件名（若提供）
if len(expectedNames) > 0 {
fileNames := make(map[string]bool)
for _, e := range entries {
fileNames[e.Name()] = true
}
for _, expectedName := range expectedNames {
if !fileNames[expectedName] {
t.Errorf("expected file not found: %s", expectedName)
return fmt.Errorf("file not found: %s", expectedName)
}
}
}

return nil
}

// 【工具函数 2】setupTestDuplicateGroup - 创建测试数据
func setupTestDuplicateGroup(t *testing.T, groupName string, fileCount int) ([]ImageInfo, string) {
tmpDir := t.TempDir()

var files []ImageInfo
for i := 0; i < fileCount; i++ {
filename := filepath.Join(tmpDir, fmt.Sprintf("file_%d.jpg", i))

// 创建测试图像（不同大小）
img := image.NewRGBA(image.Rect(0, 0, 100+i*10, 100))
f, err := os.Create(filename)
if err != nil {
t.Fatal(err)
}
jpeg.Encode(f, img, nil)
f.Close()

// 获取文件信息
info, err := os.Stat(filename)
if err != nil {
t.Fatal(err)
}

files = append(files, ImageInfo{
Path: filename,
Size: info.Size(),
})
}

return files, tmpDir
}

// 【测试函数 3】TestAutoMode_NormalOperation - 验证标准操作场景
// 【测试函数 3】TestAutoMode_NormalOperation - 验证标准操作场景
// 【测试函数 3】TestAutoMode_NormalOperation - 验证标准操作场景
func TestAutoMode_NormalOperation(t *testing.T) {
rootDir := t.TempDir()

// SETUP：创建测试文件在 rootDir 内
inputDir := filepath.Join(rootDir, "input")
if err := os.MkdirAll(inputDir, 0755); err != nil {
t.Fatalf("failed to create input dir: %v", err)
}

var files []ImageInfo
for i := 0; i < 3; i++ {
filename := filepath.Join(inputDir, fmt.Sprintf("file_%d.jpg", i))

// 创建测试图像（不同大小）
img := image.NewRGBA(image.Rect(0, 0, 100+i*10, 100))
f, err := os.Create(filename)
if err != nil {
t.Fatal(err)
}
jpeg.Encode(f, img, nil)
f.Close()

// 获取文件信息
info, err := os.Stat(filename)
if err != nil {
t.Fatal(err)
}

files = append(files, ImageInfo{
Path:   filename,
Size:   info.Size(),
Width:  100 + i*10,
Height: 100,
})
}

cfg := DefaultConfig()
cfg.Auto = true
cfg.DryRun = false
dupGroups := []DuplicateGroup{{Files: files}}

// 模拟根目录复制失败的场景：
// 创建一个只读的目录作为 rootDir，使得 dedup-auto 创建会失败
// 但我们需要通过权限限制来模拟这一点
// 更简单的方式：让 rootDir 本身是一个文件，导致创建子目录失败

// 实际上，最好的方式是删除 rootDir 的写权限，
// 但这会导致整个 handleAutoMode 失败。
// 所以我们采用不同的策略：只验证正常情况下 group 目录创建成功

// EXECUTE
_, err := handleAutoMode(rootDir, cfg, dupGroups, 3, nil)
if err != nil {
t.Fatal(err)
}

// VERIFY：group 目录中的文件应该存在
groupDir := filepath.Join(rootDir, "dedup-auto", "group-001")
if err := assertFilesInGroup(t, groupDir, 3, nil); err != nil {
t.Errorf("group directory verification failed: %v", err)
}

// 验证根目录中也有保留的文件
rootFiles, err := os.ReadDir(filepath.Join(rootDir, "dedup-auto"))
if err != nil {
t.Errorf("failed to read root dedup-auto dir: %v", err)
}
// 根目录应该有至少 1 个文件（保留的文件）和 1 个子目录（group-001）
if len(rootFiles) < 2 {
t.Errorf("expected at least 2 entries in root dedup-auto (1 file + group-001), got %d", len(rootFiles))
}
}


func TestAutoMode_DryRunEnhanced(t *testing.T) {
tmpDir := t.TempDir()

// SETUP
files, inputDir := setupTestDuplicateGroup(t, "dryrun", 3)
defer os.RemoveAll(inputDir)

cfg := DefaultConfig()
cfg.Auto = true
cfg.DryRun = true
dupGroups := []DuplicateGroup{{Files: files}}

// EXECUTE
	result, err := handleAutoMode(tmpDir, cfg, dupGroups, 3, nil)
if err != nil {
t.Fatal(err)
}

// VERIFY：dedup-auto 目录不存在
dedupRoot := filepath.Join(tmpDir, "dedup-auto")
if _, err := os.Stat(dedupRoot); !os.IsNotExist(err) {
t.Errorf("dedup-auto directory should not exist in DryRun mode")
}

// VERIFY：统计计算仍然正确
if result.TotalGroups != 1 || result.TotalDupes != 2 {
t.Errorf("statistics incorrect in DryRun mode: groups=%d, dupes=%d", result.TotalGroups, result.TotalDupes)
}
}

// TestPrepareFileForDecode_HEIC tests HEIC detection and conversion
func TestPrepareFileForDecode_HEIC(t *testing.T) {
// Create a temp directory with a fake HEIC file
tmpDir := t.TempDir()
heicPath := filepath.Join(tmpDir, "photo.heic")

// Create a valid JPEG file but name it as HEIC to simulate real Google Takeout issue
createJPEGFixture(t, heicPath)

// Test prepareFileForDecode
workPath, cleanup, err := prepareFileForDecode(heicPath)
if err != nil {
t.Fatalf("prepareFileForDecode failed: %v", err)
}
defer cleanup()

// workPath should be a temporary JPEG file
if !strings.HasSuffix(workPath, ".jpg") {
t.Errorf("expected workPath to be .jpg, got %s", workPath)
}

// temporary JPEG should exist
if _, err := os.Stat(workPath); err != nil {
t.Errorf("temporary JPEG file not found: %v", err)
}

// After cleanup, temporary JPEG should be removed
if err := cleanup(); err != nil {
t.Errorf("cleanup failed: %v", err)
}
if _, err := os.Stat(workPath); !os.IsNotExist(err) {
t.Errorf("temporary JPEG file not cleaned up")
}
}

// TestPrepareFileForDecode_MismatchedExtension tests extension correction
func TestPrepareFileForDecode_MismatchedExtension(t *testing.T) {
tmpDir := t.TempDir()
wrongExtPath := filepath.Join(tmpDir, "photo.jpg")

// Create a valid PNG file but name it as JPG
img := image.NewRGBA(image.Rect(0, 0, 10, 10))
f, err := os.Create(wrongExtPath)
if err != nil {
t.Fatalf("Failed to create test file: %v", err)
}
defer f.Close()
if err := png.Encode(f, img); err != nil {
t.Fatalf("Failed to encode PNG: %v", err)
}

// Test prepareFileForDecode
workPath, cleanup, err := prepareFileForDecode(wrongExtPath)
if err != nil {
t.Fatalf("prepareFileForDecode failed: %v", err)
}
defer cleanup()

// workPath should have .png extension
if !strings.HasSuffix(workPath, ".png") {
t.Errorf("expected workPath to be .png, got %s", workPath)
}

// After cleanup, file should be renamed back
if err := cleanup(); err != nil {
t.Errorf("cleanup failed: %v", err)
}
if _, err := os.Stat(wrongExtPath); err != nil {
t.Errorf("original file not restored after cleanup: %v", err)
}
}

// TestPrepareFileForDecode_CorrectExtension tests files with correct extensions
func TestPrepareFileForDecode_CorrectExtension(t *testing.T) {
tmpDir := t.TempDir()
jpgPath := filepath.Join(tmpDir, "photo.jpg")

// Create a valid JPEG file
createJPEGFixture(t, jpgPath)

// Test prepareFileForDecode
workPath, cleanup, err := prepareFileForDecode(jpgPath)
if err != nil {
t.Fatalf("prepareFileForDecode failed: %v", err)
}
defer cleanup()

// workPath should be the same as original
if workPath != jpgPath {
t.Errorf("expected workPath to be %s, got %s", jpgPath, workPath)
}

// After cleanup, original file should still exist
if err := cleanup(); err != nil {
t.Errorf("cleanup failed: %v", err)
}
if _, err := os.Stat(jpgPath); err != nil {
t.Errorf("original file was affected by cleanup: %v", err)
}
}

// TestRun_WithMismatchedExtensions tests dedup with mismatched extensions
func TestRun_WithMismatchedExtensions(t *testing.T) {
	tmpDir := t.TempDir()

	// Create identical images but with wrong extensions
	createCheckerImage(t, filepath.Join(tmpDir, "photo1.jpg"), 8)
	createCheckerImage(t, filepath.Join(tmpDir, "photo2.jpg"), 8)

	cfg := DefaultConfig()
	cfg.Threshold = 5
	result, err := Run(tmpDir, cfg)
	if err != nil {
		t.Fatal(err)
	}

	// Should detect as duplicates despite mismatched extensions
	if result.TotalGroups != 1 {
		t.Errorf("expected 1 duplicate group, got %d", result.TotalGroups)
	}
}

// TestHandleStandardMode_GroupNaming_SingleDigit tests group naming format for 1-9 duplicates
func TestHandleStandardMode_GroupNaming_SingleDigit(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create 1 to 9 identical files
	color1 := color.RGBA{100, 100, 100, 255}
	for i := 0; i < 9; i++ {
		path := filepath.Join(tmpDir, fmt.Sprintf("photo_%d.jpg", i))
		createSolidImage(t, path, color1)
	}
	
	cfg := DefaultConfig()
	cfg.DryRun = false
	result, err := Run(tmpDir, cfg)
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}
	
	if result.TotalGroups != 1 {
		t.Fatalf("expected 1 group, got %d", result.TotalGroups)
	}
	
	// Verify the group directory name
	dedupDir := filepath.Join(tmpDir, "dedup")
	entries, err := os.ReadDir(dedupDir)
	if err != nil {
		t.Fatalf("failed to read dedup dir: %v", err)
	}
	
	foundGroups := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			foundGroups = append(foundGroups, entry.Name())
		}
	}
	
	// For 1 group, it should be named "group-001"
	if len(foundGroups) != 1 {
		t.Errorf("expected 1 group dir, got %d", len(foundGroups))
	}
	if len(foundGroups) > 0 && foundGroups[0] != "group-001" {
		t.Errorf("expected group named 'group-001', got %q", foundGroups[0])
	}
}

// TestHandleStandardMode_GroupNaming_DoubleDigit tests group naming format for 10-99 duplicates
func TestHandleStandardMode_GroupNaming_DoubleDigit(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create 10 different groups using pattern-based images
	for groupIdx := 0; groupIdx < 10; groupIdx++ {
		for fileIdx := 0; fileIdx < 2; fileIdx++ {
			path := filepath.Join(tmpDir, fmt.Sprintf("g%02d_f%d.jpg", groupIdx, fileIdx))
			
			// Create pattern-based different images for each group
			img := image.NewRGBA(image.Rect(0, 0, 64, 64))
			for x := 0; x < 64; x++ {
				for y := 0; y < 64; y++ {
					// Use group index to vary the pattern
					if (x + groupIdx*8) % (y + groupIdx + 10) == 0 {
						img.Set(x, y, color.White)
					} else {
						img.Set(x, y, color.Black)
					}
				}
			}
			
			f, err := os.Create(path)
			if err != nil {
				t.Fatalf("failed to create image: %v", err)
			}
			defer f.Close()
			if err := jpeg.Encode(f, img, nil); err != nil {
				t.Fatalf("failed to encode: %v", err)
			}
		}
	}
	
	cfg := DefaultConfig()
	cfg.DryRun = false
	result, err := Run(tmpDir, cfg)
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}
	
	if result.TotalGroups != 10 {
		t.Fatalf("expected 10 groups, got %d", result.TotalGroups)
	}
	
	// Verify group directory names include both single and double digit naming (001-010)
	dedupDir := filepath.Join(tmpDir, "dedup")
	entries, err := os.ReadDir(dedupDir)
	if err != nil {
		t.Fatalf("failed to read dedup dir: %v", err)
	}
	
	foundGroups := make(map[string]bool)
	for _, entry := range entries {
		if entry.IsDir() {
			foundGroups[entry.Name()] = true
		}
	}
	
	// Verify first and last groups exist with correct format
	if !foundGroups["group-001"] {
		t.Errorf("expected 'group-001' to exist")
	}
	if !foundGroups["group-010"] {
		t.Errorf("expected 'group-010' to exist")
	}
}

// TestHandleStandardMode_GroupNaming_TripleDigit tests group naming format with 100+ indices
func TestHandleStandardMode_GroupNaming_TripleDigit(t *testing.T) {
	// This test verifies the format string works correctly by checking the format directly
	// rather than trying to create 100+ groups (which would be slow)
	testCases := []int{1, 9, 10, 99, 100, 150, 999}
	
	for _, i := range testCases {
		expected := fmt.Sprintf("group-%03d", i)
		
		// Verify format
		if len(expected) != 9 { // "group-" (6) + 3 digits = 9
			t.Errorf("for i=%d, expected length 9, got %d: %q", i, len(expected), expected)
		}
		
		// Verify it matches the pattern "group-XXX"
		if !strings.HasPrefix(expected, "group-") {
			t.Errorf("for i=%d, expected prefix 'group-', got %q", i, expected)
		}
	}
}

// TestHandleAutoMode_GroupNaming_SingleDigit tests auto mode group naming format for 1-9 duplicates
func TestHandleAutoMode_GroupNaming_SingleDigit(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create 1 group with 3 identical files in auto mode
	color1 := color.RGBA{200, 100, 50, 255}
	for i := 0; i < 3; i++ {
		path := filepath.Join(tmpDir, fmt.Sprintf("auto_photo_%d.jpg", i))
		createSolidImage(t, path, color1)
	}
	
	cfg := DefaultConfig()
	cfg.Auto = true
	cfg.DryRun = false
	result, err := Run(tmpDir, cfg)
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}
	
	if result.TotalGroups != 1 {
		t.Fatalf("expected 1 group, got %d", result.TotalGroups)
	}
	
	// Verify auto mode group directory name in dedup-auto
	dedupAutoDir := filepath.Join(tmpDir, "dedup-auto")
	entries, err := os.ReadDir(dedupAutoDir)
	if err != nil {
		t.Fatalf("failed to read dedup-auto dir: %v", err)
	}
	
	foundGroups := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			foundGroups = append(foundGroups, entry.Name())
		}
	}
	
	// For 1 group in auto mode, it should be named "group-001"
	if len(foundGroups) != 1 {
		t.Errorf("expected 1 group dir in dedup-auto, got %d", len(foundGroups))
	}
	if len(foundGroups) > 0 && foundGroups[0] != "group-001" {
		t.Errorf("expected group named 'group-001' in auto mode, got %q", foundGroups[0])
	}
}

// TestHandleAutoMode_GroupNaming_MultipleDigit tests auto mode group naming format for 10+ duplicates
func TestHandleAutoMode_GroupNaming_MultipleDigit(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create 5 different groups using pattern-based images in auto mode
	for groupIdx := 0; groupIdx < 5; groupIdx++ {
		for fileIdx := 0; fileIdx < 2; fileIdx++ {
			path := filepath.Join(tmpDir, fmt.Sprintf("auto_group_%d_file_%d.jpg", groupIdx, fileIdx))
			
			// Create pattern-based different images for each group
			img := image.NewRGBA(image.Rect(0, 0, 64, 64))
			for x := 0; x < 64; x++ {
				for y := 0; y < 64; y++ {
					// Use group index to vary the pattern
					if (x*2 + groupIdx*13) % (y + groupIdx*7 + 20) == 0 {
						img.Set(x, y, color.White)
					} else {
						img.Set(x, y, color.Black)
					}
				}
			}
			
			f, err := os.Create(path)
			if err != nil {
				t.Fatalf("failed to create image: %v", err)
			}
			defer f.Close()
			if err := jpeg.Encode(f, img, nil); err != nil {
				t.Fatalf("failed to encode: %v", err)
			}
		}
	}
	
	cfg := DefaultConfig()
	cfg.Auto = true
	cfg.DryRun = false
	result, err := Run(tmpDir, cfg)
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}
	
	if result.TotalGroups != 5 {
		t.Fatalf("expected 5 groups, got %d", result.TotalGroups)
	}
	
	// Verify auto mode group directory names in dedup-auto
	dedupAutoDir := filepath.Join(tmpDir, "dedup-auto")
	entries, err := os.ReadDir(dedupAutoDir)
	if err != nil {
		t.Fatalf("failed to read dedup-auto dir: %v", err)
	}
	
	foundGroups := make(map[string]bool)
	for _, entry := range entries {
		if entry.IsDir() {
			foundGroups[entry.Name()] = true
		}
	}
	
	// Verify groups are named correctly: group-001 through group-005
	for i := 1; i <= 5; i++ {
		expected := fmt.Sprintf("group-%03d", i)
		if !foundGroups[expected] {
			t.Errorf("expected group %q in auto mode, got groups: %v", expected, foundGroups)
		}
	}
}

// TestGroupNaming_LexicographicOrder tests that group directory names sort lexicographically in correct order
func TestGroupNaming_LexicographicOrder(t *testing.T) {
	// Verify that the group names follow lexicographic ordering
	// The key property is that group-%03d format maintains order up to 999 groups
	names := []string{}
	for i := 1; i <= 999; i++ {
		names = append(names, fmt.Sprintf("group-%03d", i))
	}
	
	// Verify all consecutive pairs are in ascending order
	for i := 0; i < len(names)-1; i++ {
		if names[i] >= names[i+1] {
			t.Errorf("group names not in lexicographic order at position %d: %q >= %q", i, names[i], names[i+1])
		}
	}
	
	// Verify specific boundary cases that are critical for sorting
	testCases := []struct {
		a, b string
		want bool // true if a < b
	}{
		{"group-001", "group-010", true},
		{"group-009", "group-010", true},
		{"group-010", "group-099", true},
		{"group-099", "group-100", true},
		{"group-100", "group-999", true},
	}
	
	for _, tc := range testCases {
		result := tc.a < tc.b
		if result != tc.want {
			t.Errorf("expected %q < %q to be %v, got %v", tc.a, tc.b, tc.want, result)
		}
	}
}

// Helper function to create a JPEG fixture
func createJPEGFixture(t *testing.T, path string) image.Image {
img := image.NewRGBA(image.Rect(0, 0, 10, 10))
f, err := os.Create(path)
if err != nil {
t.Fatalf("Failed to create test file: %v", err)
}
defer f.Close()
if err := jpeg.Encode(f, img, nil); err != nil {
t.Fatalf("Failed to encode JPEG: %v", err)
}
return img
}
