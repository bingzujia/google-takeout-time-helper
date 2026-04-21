package screenshotter

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bingzujia/google-takeout-time-helper/internal/logutil"
)

// ─────────────────────────────────────────────────────────────────────────────
// TASK-07: TestGetFileModTime - Unit tests for GetFileModTime()
// ─────────────────────────────────────────────────────────────────────────────

func TestGetFileModTime_ValidFile(t *testing.T) {
	// Create a temp file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	
	// Write a file with specific modtime
	if err := os.WriteFile(testFile, []byte("test"), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Get modtime
	result, ok := GetFileModTime(testFile)
	if !ok {
		t.Fatal("GetFileModTime should succeed for existing file")
	}

	// Verify it's a valid time
	if result.IsZero() {
		t.Error("returned time should not be zero")
	}

	// Verify it's in UTC
	if result.Location() != time.UTC {
		t.Errorf("returned time should be in UTC, got %v", result.Location())
	}
}

func TestGetFileModTime_NonExistentFile(t *testing.T) {
	result, ok := GetFileModTime("/nonexistent/file/path")
	if ok {
		t.Fatal("GetFileModTime should fail for non-existent file")
	}
	
	if !result.IsZero() {
		t.Error("returned time should be zero on failure")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TASK-08: TestParseScreenshotTimestamp_Public - Unit tests for ParseScreenshotTimestamp() with 3 return values
// ─────────────────────────────────────────────────────────────────────────────

func TestParseScreenshotTimestamp_Public_Format1(t *testing.T) {
	// Format 1: YYYY-MM-DD-HH-MM-SS-MS (public version returns 3 values)
	filename := "Screenshot_2025-07-18-09-23-54-65.jpg"
	result, ok, source := ParseScreenshotTimestamp(filename)
	
	if !ok {
		t.Fatalf("ParseScreenshotTimestamp failed to parse format 1")
	}
	
	expected := time.Date(2025, 7, 18, 9, 23, 54, 65*10_000_000, time.UTC)
	if result.Sub(expected) != 0 {
		t.Errorf("expected %v, got %v", expected, result)
	}
	
	// Source should be empty for filename parsing (internal use only)
	if source != "" {
		t.Errorf("expected empty source, got %s", source)
	}
}

func TestParseScreenshotTimestamp_Public_Format2(t *testing.T) {
	// Format 2: YYYYMMDD_HHMMSS
	filename := "screenshot20250718_092354.png"
	result, ok, _ := ParseScreenshotTimestamp(filename)
	
	if !ok {
		t.Fatal("ParseScreenshotTimestamp failed to parse format 2")
	}
	
	expected := time.Date(2025, 7, 18, 9, 23, 54, 0, time.UTC)
	if result.Sub(expected) != 0 {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestParseScreenshotTimestamp_Public_Format3(t *testing.T) {
	// Format 3: YYYY-MM-DD_HH-MM-SS
	filename := "Screenshot_2025-07-18_09-23-54.jpg"
	result, ok, _ := ParseScreenshotTimestamp(filename)
	
	if !ok {
		t.Fatal("ParseScreenshotTimestamp failed to parse format 3")
	}
	
	expected := time.Date(2025, 7, 18, 9, 23, 54, 0, time.UTC)
	if result.Sub(expected) != 0 {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestParseScreenshotTimestamp_Public_NoMatch(t *testing.T) {
	// No matching format
	filename := "random_file.txt"
	result, ok, source := ParseScreenshotTimestamp(filename)
	
	if ok {
		t.Fatal("ParseScreenshotTimestamp should fail for non-matching filename")
	}
	
	if !result.IsZero() {
		t.Error("returned time should be zero on failure")
	}
	
	if source != "" {
		t.Error("source should be empty on failure")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TASK-09: TestResolveTimestamp - Unit tests for ResolveTimestamp() priority logic
// ─────────────────────────────────────────────────────────────────────────────

func TestResolveTimestamp_FilenameSuccess(t *testing.T) {
	// Filename has valid timestamp, modtime is irrelevant
	tmpDir := t.TempDir()
	filename := "Screenshot_2025-07-18-09-23-54-65.jpg"
	filepath := filepath.Join(tmpDir, filename)
	
	// Create a file with old modtime
	if err := os.WriteFile(filepath, []byte("test"), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	result, ok, source := ResolveTimestamp(filepath, filename)
	
	if !ok {
		t.Fatal("ResolveTimestamp should succeed (filename has timestamp)")
	}
	
	if source != "filename" {
		t.Errorf("expected source 'filename', got '%s'", source)
	}
	
	// Verify correct timestamp from filename
	expected := time.Date(2025, 7, 18, 9, 23, 54, 65*10_000_000, time.UTC)
	if result.Sub(expected) != 0 {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestResolveTimestamp_ModtimeFallback(t *testing.T) {
	// Filename has no timestamp, should fall back to modtime
	tmpDir := t.TempDir()
	filename := "random_file.jpg"
	filepath := filepath.Join(tmpDir, filename)
	
	if err := os.WriteFile(filepath, []byte("test"), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	result, ok, source := ResolveTimestamp(filepath, filename)
	
	if !ok {
		t.Fatal("ResolveTimestamp should succeed (modtime is available)")
	}
	
	if source != "modtime" {
		t.Errorf("expected source 'modtime', got '%s'", source)
	}
	
	// Verify time is from modtime (should be close to now)
	now := time.Now().UTC()
	diff := now.Sub(result).Abs()
	if diff > 2*time.Second { // Allow 2 seconds tolerance
		t.Errorf("expected modtime close to now, got %v (diff: %v)", result, diff)
	}
}

func TestResolveTimestamp_BothFail(t *testing.T) {
	// Nonexistent file (can't get modtime) and filename has no timestamp
	filepath := "/nonexistent/path/random.jpg"
	filename := "random.jpg"
	
	result, ok, source := ResolveTimestamp(filepath, filename)
	
	if ok {
		t.Fatal("ResolveTimestamp should fail when both methods fail")
	}
	
	if !result.IsZero() {
		t.Error("returned time should be zero on failure")
	}
	
	if source != "" {
		t.Error("source should be empty on failure")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TASK-12: TestRun_IntegrationWithModtime - Integration tests for Run() with modtime fallback
// ─────────────────────────────────────────────────────────────────────────────

func TestRun_IntegrationCompleteFlow(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create test files
	testFiles := map[string]bool{
		"photo_2025-07-18_09-23-54.jpg": true,  // Has timestamp
		"random.txt":                      false, // No timestamp - will use modtime
	}
	
	for name := range testFiles {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte("test"), 0o644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
	}
	
	// Run with dry-run to avoid actual file modifications
	cfg := Config{
		Dir:    tmpDir,
		DryRun: true,
		Logger: logutil.Nop(),
	}
	
	stats, err := Run(cfg)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	
	// Both files should be processed (1 by filename, 1 by modtime)
	if stats.Renamed != 2 {
		t.Errorf("expected 2 renamed, got %d", stats.Renamed)
	}
	
	if stats.Skipped != 0 {
		t.Errorf("expected 0 skipped, got %d", stats.Skipped)
	}
	
	if stats.Errors != 0 {
		t.Errorf("expected 0 errors, got %d", stats.Errors)
	}
}

func TestRun_IntegrationDryRunNoModification(t *testing.T) {
	tmpDir := t.TempDir()
	
	testFile := "photo_2025-07-18_09-23-54.jpg"
	if err := os.WriteFile(filepath.Join(tmpDir, testFile), []byte("test"), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	
	cfg := Config{
		Dir:    tmpDir,
		DryRun: true,
		Logger: logutil.Nop(),
	}
	
	stats, err := Run(cfg)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	
	if stats.Renamed != 1 {
		t.Errorf("expected 1 renamed, got %d", stats.Renamed)
	}
	
	// Verify file still has original name (no actual rename)
	if _, err := os.Stat(filepath.Join(tmpDir, testFile)); err != nil {
		t.Errorf("original file should still exist in dry-run mode: %v", err)
	}
}

func TestRun_IntegrationConflictHandling(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create files that will generate the same output name (same timestamp)
	files := []string{
		"photo1_2025-07-18_00-00-00.jpg",
		"photo2_2025-07-18_00-00-00.jpg",
	}
	
	for _, name := range files {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte("test"), 0o644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
	}
	
	cfg := Config{
		Dir:    tmpDir,
		DryRun: true,
		Logger: logutil.Nop(),
	}
	
	stats, err := Run(cfg)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	
	// Both should be renamed (one with _001 suffix)
	if stats.Renamed != 2 {
		t.Errorf("expected 2 renamed (conflict handled), got %d", stats.Renamed)
	}
}

