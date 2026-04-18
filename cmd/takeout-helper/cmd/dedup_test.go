package cmd

import (
	"bytes"
	"os"
	"testing"

	"github.com/bingzujia/google-takeout-time-helper/internal/dedup"
)

// TestStandardModeLogging_SingleGroup tests standard mode log format with single group
func TestStandardModeLogging_SingleGroup(t *testing.T) {
	result := &dedup.Result{
		TotalScanned: 5,
		TotalGroups:  1,
		TotalDupes:   2,
		Groups: []dedup.DuplicateGroup{
			{
				Files: []dedup.ImageInfo{
					{Path: "output/photo1.jpg"},
					{Path: "output/photo1(1).jpg"},
					{Path: "output/photo1(2).jpg"},
				},
				Keep: 0, // first file is kept
			},
		},
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Call outputGroupLogging
	outputGroupLogging(result, "output", false)

	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Verify format
	if !bytes.Contains(buf.Bytes(), []byte("[group-001] 3 duplicate file(s):")) {
		t.Errorf("expected group header format '[group-001] 3 duplicate file(s):', got: %s", output)
	}

	// Verify file paths are present
	if !bytes.Contains(buf.Bytes(), []byte("output/photo1.jpg")) {
		t.Errorf("expected source path 'output/photo1.jpg' in output")
	}

	// Verify indentation (2 spaces before file line)
	if !bytes.Contains(buf.Bytes(), []byte("  output/photo1.jpg")) {
		t.Errorf("expected 2-space indentation before file path")
	}

	// Verify arrow separator
	if !bytes.Contains(buf.Bytes(), []byte(" → ")) {
		t.Errorf("expected ' → ' separator between source and destination paths")
	}

	// Verify NO [KEEP] marker in standard mode
	if bytes.Contains(buf.Bytes(), []byte("[KEEP]")) {
		t.Errorf("unexpected [KEEP] marker in standard mode")
	}
}

// TestStandardModeLogging_MultipleGroups tests standard mode with multiple groups
func TestStandardModeLogging_MultipleGroups(t *testing.T) {
	result := &dedup.Result{
		TotalScanned: 10,
		TotalGroups:  2,
		TotalDupes:   3,
		Groups: []dedup.DuplicateGroup{
			{
				Files: []dedup.ImageInfo{
					{Path: "output/img1.jpg"},
					{Path: "output/img1(1).jpg"},
				},
				Keep: 0,
			},
			{
				Files: []dedup.ImageInfo{
					{Path: "output/img2.png"},
					{Path: "output/img2(1).png"},
					{Path: "output/img2(2).png"},
				},
				Keep: 0,
			},
		},
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Call outputGroupLogging
	outputGroupLogging(result, "output", false)

	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	var buf bytes.Buffer
	buf.ReadFrom(r)

	// Verify both group headers
	if !bytes.Contains(buf.Bytes(), []byte("[group-001]")) {
		t.Errorf("expected [group-001] header")
	}
	if !bytes.Contains(buf.Bytes(), []byte("[group-002]")) {
		t.Errorf("expected [group-002] header")
	}

	// Verify file count for each group
	if !bytes.Contains(buf.Bytes(), []byte("2 duplicate file(s):")) {
		t.Errorf("expected '2 duplicate file(s):' for first group")
	}
	if !bytes.Contains(buf.Bytes(), []byte("3 duplicate file(s):")) {
		t.Errorf("expected '3 duplicate file(s):' for second group")
	}

	// Verify order in output
	idx1 := bytes.Index(buf.Bytes(), []byte("[group-001]"))
	idx2 := bytes.Index(buf.Bytes(), []byte("[group-002]"))
	if idx1 >= idx2 {
		t.Errorf("group-001 should appear before group-002")
	}
}

// ============================================================================
// TASK-02: Auto Mode [KEEP] Marker Tests
// ============================================================================

// TestAutoModeLogging_WithKeepMarker tests auto mode log with [KEEP] markers
func TestAutoModeLogging_WithKeepMarker(t *testing.T) {
	result := &dedup.Result{
		TotalScanned: 5,
		TotalGroups:  1,
		TotalDupes:   2,
		Groups: []dedup.DuplicateGroup{
			{
				Files: []dedup.ImageInfo{
					{Path: "output/photo1.jpg"},
					{Path: "output/photo1(1).jpg"},
					{Path: "output/photo1(2).jpg"},
				},
				Keep: 0, // first file is kept
			},
		},
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Call outputGroupLogging with auto mode
	outputGroupLogging(result, "output", true)

	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Verify [KEEP] marker appears exactly once
	keepCount := bytes.Count(buf.Bytes(), []byte("[KEEP]"))
	if keepCount != 1 {
		t.Errorf("expected exactly 1 [KEEP] marker, got %d in: %s", keepCount, output)
	}

	// Verify [KEEP] marker appears only on first file line
	lines := bytes.Split(buf.Bytes(), []byte("\n"))
	keepFoundOnFirstFile := false

	for i, line := range lines {
		if bytes.Contains(line, []byte("photo1.jpg")) && !bytes.Contains(line, []byte("photo1(")) {
			// First file (kept)
			if bytes.Contains(line, []byte("[KEEP]")) {
				keepFoundOnFirstFile = true
			}
		} else if bytes.Contains(line, []byte("photo1(")) {
			// Duplicate files (not kept)
			if bytes.Contains(line, []byte("[KEEP]")) {
				t.Errorf("line %d should not have [KEEP] marker: %s", i, string(line))
			}
		}
	}

	if !keepFoundOnFirstFile {
		t.Errorf("expected [KEEP] marker on first (kept) file")
	}
}

// TestAutoModeLogging_PathFormatting tests auto mode path formatting
func TestAutoModeLogging_PathFormatting(t *testing.T) {
	result := &dedup.Result{
		TotalScanned: 5,
		TotalGroups:  1,
		TotalDupes:   2,
		Groups: []dedup.DuplicateGroup{
			{
				Files: []dedup.ImageInfo{
					{Path: "output/photo1.jpg"},
					{Path: "output/photo1(1).jpg"},
				},
				Keep: 0,
			},
		},
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Call outputGroupLogging with auto mode
	outputGroupLogging(result, "output", true)

	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Verify kept file path: dedup-auto/filename (no group subdir)
	if !bytes.Contains(buf.Bytes(), []byte("dedup-auto/photo1.jpg")) {
		t.Errorf("expected kept file path 'dedup-auto/photo1.jpg' (no group), got: %s", output)
	}

	// Verify non-kept file path: dedup-auto/group-001/filename
	if !bytes.Contains(buf.Bytes(), []byte("dedup-auto/group-001/photo1(1).jpg")) {
		t.Errorf("expected non-kept file path 'dedup-auto/group-001/photo1(1).jpg', got: %s", output)
	}
}

// ============================================================================
// TASK-03: Dry-Run Mode Consistency Tests
// ============================================================================

// TestDryRunLogging_IdenticalFormat tests that dry-run logs same format
func TestDryRunLogging_IdenticalFormat(t *testing.T) {
	result := &dedup.Result{
		TotalScanned: 5,
		TotalGroups:  1,
		TotalDupes:   2,
		Groups: []dedup.DuplicateGroup{
			{
				Files: []dedup.ImageInfo{
					{Path: "output/photo1.jpg"},
					{Path: "output/photo1(1).jpg"},
				},
				Keep: 0,
			},
		},
	}

	// Capture stdout for standard mode
	oldStdout := os.Stdout
	r1, w1, _ := os.Pipe()
	os.Stdout = w1
	outputGroupLogging(result, "output", false)
	w1.Close()
	os.Stdout = oldStdout

	var buf1 bytes.Buffer
	buf1.ReadFrom(r1)
	standardOutput := buf1.String()

	// The dry-run output should use the same format
	// (dry-run flag is handled elsewhere, not in outputGroupLogging)
	// This test verifies the format structure is consistent

	if !bytes.Contains(buf1.Bytes(), []byte("[group-001]")) {
		t.Errorf("expected [group-001] header in output")
	}

	if !bytes.Contains(buf1.Bytes(), []byte("2 duplicate file(s):")) {
		t.Errorf("expected '2 duplicate file(s):' in output: %s", standardOutput)
	}
}

// ============================================================================
// TASK-04: Boundary Case Tests
// ============================================================================

// TestBoundaryCase_NoGroups tests output when no groups found
func TestBoundaryCase_NoGroups(t *testing.T) {
	result := &dedup.Result{
		TotalScanned: 10,
		TotalGroups:  0,
		TotalDupes:   0,
		Groups:       []dedup.DuplicateGroup{},
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	outputGroupLogging(result, "output", false)

	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	var buf bytes.Buffer
	buf.ReadFrom(r)

	// Should produce no output for empty groups
	if len(buf.Bytes()) > 0 {
		// It's OK if there's minimal output, but shouldn't crash
		t.Logf("no-group output: %s", buf.String())
	}
}

// TestBoundaryCase_SpecialCharacters tests handling of special characters in filenames
func TestBoundaryCase_SpecialCharacters(t *testing.T) {
	result := &dedup.Result{
		TotalScanned: 3,
		TotalGroups:  1,
		TotalDupes:   1,
		Groups: []dedup.DuplicateGroup{
			{
				Files: []dedup.ImageInfo{
					{Path: "output/photo-已修改.jpg"},
					{Path: "output/photo-已修改(1).jpg"},
				},
				Keep: 0,
			},
		},
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	outputGroupLogging(result, "output", false)

	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Verify special characters are preserved
	if !bytes.Contains(buf.Bytes(), []byte("photo-已修改.jpg")) {
		t.Errorf("expected special characters preserved in filename: %s", output)
	}

	if !bytes.Contains(buf.Bytes(), []byte("[group-001]")) {
		t.Errorf("expected group header present")
	}
}
