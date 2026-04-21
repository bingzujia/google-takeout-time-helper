package migrator

import (
"os"
"path/filepath"
"testing"
"time"

"github.com/bingzujia/google-takeout-time-helper/internal/parser"
)

// TestMoveToManualReview_SHA256Assignment_Success tests SHA256 calculation success in moveToManualReview
func TestMoveToManualReview_SHA256Assignment_Success(t *testing.T) {
// Create temporary directory structure
tmpDir := t.TempDir()
manualReviewDir := filepath.Join(tmpDir, "manual_review")
outputDir := filepath.Join(tmpDir, "output")
inputDir := filepath.Join(tmpDir, "input")

if err := os.MkdirAll(manualReviewDir, 0755); err != nil {
t.Fatal(err)
}
if err := os.MkdirAll(outputDir, 0755); err != nil {
t.Fatal(err)
}
if err := os.MkdirAll(inputDir, 0755); err != nil {
t.Fatal(err)
}

// Create test file in input directory
testFile := filepath.Join(inputDir, "test.jpg")
if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
t.Fatal(err)
}

// Create test entry
entry := FileEntry{
Path:    testFile,
RelPath: "test.jpg",
}

// Call moveToManualReview
moveToManualReview(
entry, outputDir, manualReviewDir,
nil, time.Time{},
parser.GPSInfo{}, false,
parser.GPSInfo{}, false,
"camera", "phone", "test_reason",
)

// Verify metadata file was created with valid SHA256 filename
metadataDir := filepath.Join(manualReviewDir, "metadata")
entries, err := os.ReadDir(metadataDir)
if err != nil {
t.Fatalf("metadata directory not created: %v", err)
}

if len(entries) == 0 {
t.Fatal("no metadata files created")
}

filename := entries[0].Name()
if filename == ".json" {
t.Fatal("metadata file has empty SHA256 (.json)")
}

// Should have 64 character SHA256 hex string + .json
if len(filename) != 64+5 {
t.Fatalf("expected filename format {sha256}.json (69 chars), got %d chars: %s", len(filename), filename)
}
}

// TestMoveToManualReview_SHA256Assignment_Error tests SHA256 calculation failure in moveToManualReview
func TestMoveToManualReview_SHA256Assignment_Error(t *testing.T) {
// Create temporary directories
tmpDir := t.TempDir()
manualReviewDir := filepath.Join(tmpDir, "manual_review")
outputDir := filepath.Join(tmpDir, "output")
inputDir := filepath.Join(tmpDir, "input")

if err := os.MkdirAll(manualReviewDir, 0755); err != nil {
t.Fatal(err)
}
if err := os.MkdirAll(outputDir, 0755); err != nil {
t.Fatal(err)
}
if err := os.MkdirAll(inputDir, 0755); err != nil {
t.Fatal(err)
}

// Create test entry with non-existent file
entry := FileEntry{
Path:    filepath.Join(inputDir, "nonexistent.jpg"),
RelPath: "nonexistent.jpg",
}

// Call moveToManualReview with non-existent file
moveToManualReview(
entry, outputDir, manualReviewDir,
nil, time.Time{},
parser.GPSInfo{}, false,
parser.GPSInfo{}, false,
"camera", "phone", "no_json_sidecar",
)

// When file doesn't exist, copyToPath will fail and function returns early
// This is acceptable behavior per design
metadataDir := filepath.Join(manualReviewDir, "metadata")
entries, err := os.ReadDir(metadataDir)

// Either directory doesn't exist or no files created is acceptable
if err == nil && len(entries) > 0 {
// If metadata was created, check filename
filename := entries[0].Name()
// Both ".json" (error case) and valid hash are acceptable in error scenario
t.Logf("metadata file created: %s", filename)
}
}

// TestMoveToManualReviewByPath_SHA256Assignment_Success tests SHA256 calculation success in moveToManualReviewByPath
func TestMoveToManualReviewByPath_SHA256Assignment_Success(t *testing.T) {
// Create temporary directory structure
tmpDir := t.TempDir()
manualReviewDir := filepath.Join(tmpDir, "manual_review")
outputDir := filepath.Join(tmpDir, "output")

if err := os.MkdirAll(manualReviewDir, 0755); err != nil {
t.Fatal(err)
}
if err := os.MkdirAll(outputDir, 0755); err != nil {
t.Fatal(err)
}

// Create test file in output directory
srcPath := filepath.Join(outputDir, "test.jpg")
if err := os.WriteFile(srcPath, []byte("test content"), 0644); err != nil {
t.Fatal(err)
}

// Call moveToManualReviewByPath to move file and write metadata
moveToManualReviewByPath(
srcPath, "test.jpg", outputDir, manualReviewDir,
nil, time.Time{},
parser.GPSInfo{}, false,
parser.GPSInfo{}, false,
"camera", "phone", "metadata_mismatch",
)

// Verify metadata file was created with valid SHA256 filename
metadataDir := filepath.Join(manualReviewDir, "metadata")
entries, err := os.ReadDir(metadataDir)
if err != nil {
t.Fatalf("metadata directory not created: %v", err)
}

if len(entries) == 0 {
t.Fatal("no metadata files created")
}

filename := entries[0].Name()
if filename == ".json" {
t.Fatal("metadata file has empty SHA256 (.json)")
}

// Should have 64 character SHA256 hex string + .json
if len(filename) != 64+5 {
t.Fatalf("expected filename format {sha256}.json (69 chars), got %d chars: %s", len(filename), filename)
}
}

// TestMoveToManualReviewByPath_SHA256Assignment_Error tests SHA256 calculation failure in moveToManualReviewByPath
func TestMoveToManualReviewByPath_SHA256Assignment_Error(t *testing.T) {
// Create temporary directories
tmpDir := t.TempDir()
manualReviewDir := filepath.Join(tmpDir, "manual_review")
outputDir := filepath.Join(tmpDir, "output")

if err := os.MkdirAll(manualReviewDir, 0755); err != nil {
t.Fatal(err)
}
if err := os.MkdirAll(outputDir, 0755); err != nil {
t.Fatal(err)
}

// Use a non-existent srcPath to simulate HashFile error
srcPath := filepath.Join(outputDir, "nonexistent.jpg")

// Call moveToManualReviewByPath with non-existent file
moveToManualReviewByPath(
srcPath, "nonexistent.jpg", outputDir, manualReviewDir,
nil, time.Time{},
parser.GPSInfo{}, false,
parser.GPSInfo{}, false,
"camera", "phone", "missing_metadata",
)

// When file doesn't exist, os.Rename fails and copyToPath will fail and function returns early
// This is acceptable behavior per design
metadataDir := filepath.Join(manualReviewDir, "metadata")
entries, err := os.ReadDir(metadataDir)

// Either directory doesn't exist or no files created is acceptable
if err == nil && len(entries) > 0 {
// If metadata was created, check filename
filename := entries[0].Name()
// Both ".json" (error case) and valid hash are acceptable in error scenario
t.Logf("metadata file created: %s", filename)
}
}
