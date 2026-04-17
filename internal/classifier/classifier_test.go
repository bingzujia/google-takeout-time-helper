package classifier

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// --- classifyFile tests (task 6.1) ---

func TestClassifyFile_Camera(t *testing.T) {
	cases := []string{
		"IMG_20230401_120000.jpg",
		"VID_20230401_120000.mp4",
		"PXL_20230401_120000.jpg",
		"IMG_001.jpeg",
		"20230401_120000.jpg",
	}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			cat, ok := classifyFile(name)
			if !ok {
				t.Fatalf("classifyFile(%q) returned ok=false, want true", name)
			}
			if cat != CategoryCamera {
				t.Errorf("classifyFile(%q) = %q, want %q", name, cat, CategoryCamera)
			}
		})
	}
}

func TestClassifyFile_Screenshot(t *testing.T) {
	cases := []string{
		"Screenshot_2023-04-01.png",
		"screenshot_001.jpg",
		"my_screenshot.png",
	}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			cat, ok := classifyFile(name)
			if !ok {
				t.Fatalf("classifyFile(%q) returned ok=false, want true", name)
			}
			if cat != CategoryScreenshot {
				t.Errorf("classifyFile(%q) = %q, want %q", name, cat, CategoryScreenshot)
			}
		})
	}
}

func TestClassifyFile_Wechat(t *testing.T) {
	cases := []string{
		"mmexport1680000000000.jpg",
		"mmexport1680000000000.mp4",
	}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			cat, ok := classifyFile(name)
			if !ok {
				t.Fatalf("classifyFile(%q) returned ok=false, want true", name)
			}
			if cat != CategoryWechat {
				t.Errorf("classifyFile(%q) = %q, want %q", name, cat, CategoryWechat)
			}
		})
	}
}

func TestClassifyFile_NoMatch(t *testing.T) {
	cases := []string{
		"random_file.jpg",
		"document.pdf",
		"notes.txt",
		"DCIM_photo.jpg", // not a recognised prefix
	}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			_, ok := classifyFile(name)
			if ok {
				t.Errorf("classifyFile(%q) returned ok=true, want false", name)
			}
		})
	}
}

// --- exifMapFallback tests (task 6.2) ---

func TestExifMapFallback_Missing(t *testing.T) {
	exifMap := map[string]exifDeviceOutput{}
	if got := exifMapFallback("/some/file.jpg", exifMap); got {
		t.Error("expected false for missing key")
	}
}

func TestExifMapFallback_EmptyFields(t *testing.T) {
	exifMap := map[string]exifDeviceOutput{
		"/some/file.jpg": {Make: "", Model: ""},
	}
	if got := exifMapFallback("/some/file.jpg", exifMap); got {
		t.Error("expected false when Make and Model are empty")
	}
}

func TestExifMapFallback_HasMake(t *testing.T) {
	exifMap := map[string]exifDeviceOutput{
		"/some/file.jpg": {Make: "Apple", Model: ""},
	}
	if got := exifMapFallback("/some/file.jpg", exifMap); !got {
		t.Error("expected true when Make is non-empty")
	}
}

// --- Run integration test (task 6.3) ---

func TestRun_Integration(t *testing.T) {
	tmpDir := t.TempDir()

	// Input layout:
	//   tmpDir/input/IMG_20230401_120000.jpg   → camera
	//   tmpDir/input/Screenshot_001.png        → screenshot
	//   tmpDir/input/mmexport1234567890.jpg    → wechat
	//   tmpDir/input/random_file.jpg           → skipped (no exif, txt content)
	//   tmpDir/input/album1/nested.jpg         → ignored (subdir)
	inputDir := filepath.Join(tmpDir, "input")
	album1 := filepath.Join(inputDir, "album1")
	if err := os.MkdirAll(album1, 0o755); err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		"IMG_20230401_120000.jpg": "fake jpeg",
		"Screenshot_001.png":      "fake png",
		"mmexport1234567890.jpg":  "fake wechat",
		"random_file.jpg":         "fake random",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(inputDir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	// Subdirectory file — should be ignored.
	if err := os.WriteFile(filepath.Join(album1, "nested.jpg"), []byte("nested"), 0644); err != nil {
		t.Fatal(err)
	}

	outputDir := filepath.Join(tmpDir, "output")

	result, err := Run(Config{InputDir: inputDir, OutputDir: outputDir, DryRun: false, ShowProgress: false})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if result.Camera != 1 {
		t.Errorf("Camera = %d, want 1", result.Camera)
	}
	if result.Screenshot != 1 {
		t.Errorf("Screenshot = %d, want 1", result.Screenshot)
	}
	if result.Wechat != 1 {
		t.Errorf("Wechat = %d, want 1", result.Wechat)
	}
	// random_file.jpg: no filename match; exiftool either skips gracefully or finds no device → Skipped.
	// seemsCamera could be >0 if exiftool finds something (unlikely for fake content).
	total := result.Camera + result.Screenshot + result.Wechat + result.SeemsCamera + result.Skipped
	if total != 4 {
		t.Errorf("total processed = %d, want 4", total)
	}

	// Verify files landed in the right directories.
	assertExists(t, filepath.Join(outputDir, "camera", "IMG_20230401_120000.jpg"))
	assertExists(t, filepath.Join(outputDir, "screenshot", "Screenshot_001.png"))
	assertExists(t, filepath.Join(outputDir, "wechat", "mmexport1234567890.jpg"))

	// Subdirectory file must still be in place — not touched.
	assertExists(t, filepath.Join(album1, "nested.jpg"))
}

func TestRun_DryRun(t *testing.T) {
	tmpDir := t.TempDir()
	inputDir := filepath.Join(tmpDir, "input")
	if err := os.MkdirAll(inputDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inputDir, "IMG_001.jpg"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	outputDir := filepath.Join(tmpDir, "output")

	result, err := Run(Config{InputDir: inputDir, OutputDir: outputDir, DryRun: true, ShowProgress: false})
	if err != nil {
		t.Fatalf("Run(DryRun) returned error: %v", err)
	}
	if result.Camera != 1 {
		t.Errorf("DryRun Camera = %d, want 1", result.Camera)
	}
	// Output directory must NOT have been created in dry-run.
	if _, err := os.Stat(outputDir); !os.IsNotExist(err) {
		t.Error("output directory should not exist after dry-run")
	}
	// Source file must still exist.
	assertExists(t, filepath.Join(inputDir, "IMG_001.jpg"))
}

func TestScanEligibleFiles(t *testing.T) {
	tmpDir := t.TempDir()
	inputDir := filepath.Join(tmpDir, "input")
	album1 := filepath.Join(inputDir, "album1")
	nested := filepath.Join(album1, "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inputDir, "root.jpg"), []byte("root"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(album1, "a.jpg"), []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inputDir, "b.png"), []byte("b"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "c.jpg"), []byte("c"), 0644); err != nil {
		t.Fatal(err)
	}

	files, err := scanEligibleFiles(inputDir)
	if err != nil {
		t.Fatalf("scanEligibleFiles() error = %v", err)
	}

	var names []string
	for _, f := range files {
		names = append(names, f.Name)
	}
	sort.Strings(names)

	want := []string{"b.png", "root.jpg"}
	if len(names) != len(want) {
		t.Fatalf("len(names) = %d, want %d (%v)", len(names), len(want), names)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("names[%d] = %q, want %q", i, names[i], want[i])
		}
	}
}

func assertExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("expected file to exist: %s", path)
	}
}
