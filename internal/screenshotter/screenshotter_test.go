package screenshotter

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bingzujia/google-takeout-time-helper/internal/logutil"
)

// ── Test Helpers ──────────────────────────────────────────────────────────────

type fakeDirEntry struct {
	name string
	isDir bool
}

func (f fakeDirEntry) Name() string               { return f.name }
func (f fakeDirEntry) IsDir() bool                { return f.isDir }
func (f fakeDirEntry) Type() os.FileMode          { return 0 }
func (f fakeDirEntry) Info() (os.FileInfo, error) { return nil, nil }

func fakeEntries(entries map[string]bool) []os.DirEntry {
	out := make([]os.DirEntry, 0, len(entries))
	for name, isDir := range entries {
		out = append(out, fakeDirEntry{name, isDir})
	}
	return out
}

func createTempDir(t *testing.T) string {
	dir, err := os.MkdirTemp("", "screenshotter-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})
	return dir
}

// ── TASK-RS-001-TEST: screenshotter main test structure ──────────────────────

func TestRun_DryRun(t *testing.T) {
	dir := createTempDir(t)

	// Create test screenshot file (matches Format 5: YYYY-MM-DD)
	testFile := "screenshot_2025-07-18.png"
	if err := os.WriteFile(filepath.Join(dir, testFile), []byte("test"), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	cfg := Config{
		Dir:    dir,
		DryRun: true,
		Logger: logutil.Nop(),
	}

	stats, err := Run(cfg)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Dry-run should report renamed but not actually rename
	if stats.Renamed != 1 {
		t.Errorf("expected 1 renamed, got %d", stats.Renamed)
	}

	// File should still have original name
	if _, err := os.Stat(filepath.Join(dir, testFile)); err != nil {
		t.Fatalf("original file should still exist in dry-run mode: %v", err)
	}
}

func TestRun_LiveMode(t *testing.T) {
	dir := createTempDir(t)

	// Create test screenshot file (matches Format 5: YYYY-MM-DD)
	testFile := "screenshot_2025-07-18.png"
	if err := os.WriteFile(filepath.Join(dir, testFile), []byte("test"), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	cfg := Config{
		Dir:    dir,
		DryRun: false,
		Logger: logutil.Nop(),
	}

	stats, err := Run(cfg)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if stats.Renamed != 1 {
		t.Errorf("expected 1 renamed, got %d", stats.Renamed)
	}

	// Original file should be gone
	if _, err := os.Stat(filepath.Join(dir, testFile)); err == nil {
		t.Error("original file should be renamed in live mode")
	}
}

// ── TASK-RS-002-TEST: TestDetectScreenshots ───────────────────────────────────

func TestDetectScreenshots_LooseMatching(t *testing.T) {
	entries := fakeEntries(map[string]bool{
		"screenshot_2025_07_18.png": false,
		"Screenshot.jpg":            false,
		"SCREENSHOT_test.png":       false,
		"photo.jpg":                 false,
		"screen.png":                false,
	})

	result := detectScreenshots(entries)

	if _, ok := result["screenshot_2025_07_18.png"]; !ok {
		t.Error("expected screenshot_2025_07_18.png to be detected")
	}
	if _, ok := result["Screenshot.jpg"]; !ok {
		t.Error("expected Screenshot.jpg to be detected")
	}
	if _, ok := result["SCREENSHOT_test.png"]; !ok {
		t.Error("expected SCREENSHOT_test.png to be detected")
	}
	if _, ok := result["photo.jpg"]; ok {
		t.Error("expected photo.jpg to not be detected")
	}
	if _, ok := result["screen.png"]; ok {
		t.Error("expected screen.png to not be detected")
	}
}

func TestDetectScreenshots_Variants(t *testing.T) {
	entries := fakeEntries(map[string]bool{
		"mmscreenshot1727421404387":    false,
		"wxscreenshot_IMG_001.jpg":     false,
		"screenshot_2025_7_18_9_23_54": false,
	})

	result := detectScreenshots(entries)

	if len(result) != 3 {
		t.Errorf("expected 3 files detected, got %d", len(result))
	}
}

func TestDetectScreenshots_ExcludeNonFiles(t *testing.T) {
	entries := fakeEntries(map[string]bool{
		"screenshot_2025": false,
		"screenshots_dir": true,
	})

	result := detectScreenshots(entries)

	if _, ok := result["screenshot_2025"]; !ok {
		t.Error("expected screenshot_2025 to be detected")
	}
	if _, ok := result["screenshots_dir"]; ok {
		t.Error("expected screenshots_dir (directory) to be excluded")
	}
}

// ── TASK-RS-003-TEST: TestParseFormat 1-5 ────────────────────────────────────

func TestParseFormat1_FullTimestamp(t *testing.T) {
	// Format 1: YYYY-MM-DD-HH-MM-SS-MS
	// Example: "Screenshot_2025-07-18-09-23-54-65.png"
	filename := "Screenshot_2025-07-18-09-23-54-65.png"
	tm, ok := parseScreenshotTimestamp(filename)
	if !ok {
		t.Errorf("failed to parse Format 1")
	}
	expected := time.Date(2025, 7, 18, 9, 23, 54, 650_000_000, time.UTC)
	if tm != expected {
		t.Errorf("Format 1: got %v, expected %v", tm, expected)
	}
}

func TestParseFormat2_YYYYMMDDHHmmss(t *testing.T) {
	// Format 2: YYYYMMDD_HHMMSS
	// Example: "screenshot20250718_092354.jpg"
	filename := "screenshot20250718_092354.jpg"
	tm, ok := parseScreenshotTimestamp(filename)
	if !ok {
		t.Errorf("failed to parse Format 2")
	}
	expected := time.Date(2025, 7, 18, 9, 23, 54, 0, time.UTC)
	if tm != expected {
		t.Errorf("Format 2: got %v, expected %v", tm, expected)
	}
}

func TestParseFormat3_YYYY_MM_DD_HH_MM_SS(t *testing.T) {
	// Format 3: YYYY-MM-DD_HH-MM-SS
	// Example: "Screenshot_2025-07-18_09-23-54.png"
	filename := "Screenshot_2025-07-18_09-23-54.png"
	tm, ok := parseScreenshotTimestamp(filename)
	if !ok {
		t.Errorf("failed to parse Format 3")
	}
	expected := time.Date(2025, 7, 18, 9, 23, 54, 0, time.UTC)
	if tm != expected {
		t.Errorf("Format 3: got %v, expected %v", tm, expected)
	}
}

func TestParseFormat4_UnpadAutoFill(t *testing.T) {
	// Format 4: YYYY_M_D_H_M_S (unpadded, auto-fill)
	// Example: "screenshot_2025_7_18_9_23_54.png"
	filename := "screenshot_2025_7_18_9_23_54.png"
	tm, ok := parseScreenshotTimestamp(filename)
	if !ok {
		t.Errorf("failed to parse Format 4")
	}
	expected := time.Date(2025, 7, 18, 9, 23, 54, 0, time.UTC)
	if tm != expected {
		t.Errorf("Format 4: got %v, expected %v", tm, expected)
	}
}

func TestParseFormat5_DateOnly(t *testing.T) {
	// Format 5: YYYY-MM-DD (date-only)
	// Example: "screenshot_2025-07-18.png"
	filename := "screenshot_2025-07-18.png"
	tm, ok := parseScreenshotTimestamp(filename)
	if !ok {
		t.Errorf("failed to parse Format 5")
	}
	expected := time.Date(2025, 7, 18, 0, 0, 0, 0, time.UTC)
	if tm != expected {
		t.Errorf("Format 5: got %v, expected %v", tm, expected)
	}
}

// ── TASK-RS-004-TEST: TestParseFormat 6a/6b ──────────────────────────────────

func TestParseFormat6a_UnixSeconds(t *testing.T) {
	// Format 6a: 10-digit Unix seconds
	// Example: "screenshot1634560000.jpg"
	// Expected: 2021-10-18 12:53:20 UTC
	filename := "screenshot1634560000.jpg"
	tm, ok := parseScreenshotTimestamp(filename)
	if !ok {
		t.Errorf("failed to parse Format 6a")
	}
	expected := time.Unix(1634560000, 0).UTC()
	if tm != expected {
		t.Errorf("Format 6a: got %v, expected %v", tm, expected)
	}
}

func TestParseFormat6a_WithExtension(t *testing.T) {
	// Format 6a with extension
	filename := "wxscreenshot1634560000.png"
	tm, ok := parseScreenshotTimestamp(filename)
	if !ok {
		t.Errorf("failed to parse Format 6a with extension")
	}
	expected := time.Unix(1634560000, 0).UTC()
	if tm != expected {
		t.Errorf("Format 6a with ext: got %v, expected %v", tm, expected)
	}
}

func TestParseFormat6b_UnixMilliseconds(t *testing.T) {
	// Format 6b: 13-digit Unix milliseconds
	// Example: "mmscreenshot1727421404387.jpg"
	// Expected: 2024-09-27 02:30:04.387 UTC (with ms precision)
	filename := "mmscreenshot1727421404387.jpg"
	tm, ok := parseScreenshotTimestamp(filename)
	if !ok {
		t.Errorf("failed to parse Format 6b")
	}
	// 1727421404387 ms = 1727421404 sec + 387 ms
	expected := time.Unix(1727421404, 387_000_000).UTC()
	if tm != expected {
		t.Errorf("Format 6b: got %v, expected %v", tm, expected)
	}
}

func TestParseFormat6b_NoExtension(t *testing.T) {
	// Format 6b without extension
	// Example: "mmscreenshot1727421404387"
	filename := "mmscreenshot1727421404387"
	tm, ok := parseScreenshotTimestamp(filename)
	if !ok {
		t.Errorf("failed to parse Format 6b no extension")
	}
	expected := time.Unix(1727421404, 387_000_000).UTC()
	if tm != expected {
		t.Errorf("Format 6b no ext: got %v, expected %v", tm, expected)
	}
}

// ── TASK-RS-005-TEST: TestGenerateNewName ────────────────────────────────────

func TestGenerateNewName_WithExtension(t *testing.T) {
	// Generate name with file extension
	// 65 * 10_000_000 ns = 650_000_000 ns = 65ms in 0-99 range
	tm := time.Date(2025, 7, 18, 9, 23, 54, 65*10_000_000, time.UTC)
	result := generateNewName(tm, ".jpg")
	expected := "Screenshot_2025-07-18-09-23-54-65.jpg"
	if result != expected {
		t.Errorf("WithExtension: got %q, expected %q", result, expected)
	}
}

func TestGenerateNewName_NoExtension(t *testing.T) {
	// Generate name without extension
	tm := time.Date(2025, 7, 18, 9, 23, 54, 0, time.UTC)
	result := generateNewName(tm, "")
	expected := "Screenshot_2025-07-18-09-23-54-00"
	if result != expected {
		t.Errorf("NoExtension: got %q, expected %q", result, expected)
	}
}

func TestGenerateNewName_VariousMs(t *testing.T) {
	// Test various 2-digit millisecond values (0, 50, 99)
	// Note: 2-digit ms is in 0-99 range where each unit = 10ms
	cases := []struct {
		ns       int // nanoseconds
		expected string
	}{
		{0, "Screenshot_2025-07-18-09-23-54-00"},
		{50 * 10_000_000, "Screenshot_2025-07-18-09-23-54-50"},
		{99 * 10_000_000, "Screenshot_2025-07-18-09-23-54-99"},
	}
	for _, c := range cases {
		tm := time.Date(2025, 7, 18, 9, 23, 54, c.ns, time.UTC)
		result := generateNewName(tm, "")
		if result != c.expected {
			t.Errorf("VariousMs: got %q, expected %q", result, c.expected)
		}
	}
}

func TestGenerateNewName_DateOnly(t *testing.T) {
	// Test date-only case (Format 5 parsed timestamp)
	tm := time.Date(2025, 7, 18, 0, 0, 0, 0, time.UTC)
	result := generateNewName(tm, ".png")
	expected := "Screenshot_2025-07-18-00-00-00-00.png"
	if result != expected {
		t.Errorf("DateOnly: got %q, expected %q", result, expected)
	}
}

// ── TASK-RS-006-TEST: TestResolveConflict ────────────────────────────────────

func TestResolveConflict_NoConflict(t *testing.T) {
	dir := createTempDir(t)
	targetName := "Screenshot_2025-07-18-09-23-54-650.jpg"
	
	result := resolveConflict(dir, targetName)
	if result != targetName {
		t.Errorf("NoConflict: got %q, expected %q", result, targetName)
	}
}

func TestResolveConflict_SingleConflict(t *testing.T) {
	dir := createTempDir(t)
	targetName := "Screenshot_2025-07-18-09-23-54-650.jpg"
	
	// Create file with target name
	if err := os.WriteFile(filepath.Join(dir, targetName), []byte("test"), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	
	result := resolveConflict(dir, targetName)
	expected := "Screenshot_2025-07-18-09-23-54-650_001.jpg"
	if result != expected {
		t.Errorf("SingleConflict: got %q, expected %q", result, expected)
	}
}

func TestResolveConflict_MultipleConflicts(t *testing.T) {
	dir := createTempDir(t)
	targetName := "Screenshot_2025-07-18-09-23-54-650.jpg"
	
	// Create files with conflicting names
	os.WriteFile(filepath.Join(dir, targetName), []byte("test"), 0o644)
	os.WriteFile(filepath.Join(dir, "Screenshot_2025-07-18-09-23-54-650_001.jpg"), []byte("test"), 0o644)
	os.WriteFile(filepath.Join(dir, "Screenshot_2025-07-18-09-23-54-650_002.jpg"), []byte("test"), 0o644)
	
	result := resolveConflict(dir, targetName)
	expected := "Screenshot_2025-07-18-09-23-54-650_003.jpg"
	if result != expected {
		t.Errorf("MultipleConflicts: got %q, expected %q", result, expected)
	}
}
