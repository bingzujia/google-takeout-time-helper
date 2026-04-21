package renamer

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ── buildName ─────────────────────────────────────────────────────────────────

func TestBuildName(t *testing.T) {
	tm := time.Date(2023, 1, 23, 10, 47, 7, 0, time.UTC)
	cases := []struct {
		ext  string
		want string
	}{
		{"heic", "IMG20230123104707.heic"},
		{"heif", "IMG20230123104707.heif"},
		{"jpg", "IMG_20230123_104707.jpg"},
		{"jpeg", "IMG_20230123_104707.jpeg"},
		{"png", "IMG_20230123_104707.png"},
		{"mp4", "VID20230123104707.mp4"},
		{"mov", "VID20230123104707.mov"},
	}
	for _, c := range cases {
		got := buildName(c.ext, tm)
		if got != c.want {
			t.Errorf("buildName(%q): got %q, want %q", c.ext, got, c.want)
		}
	}
}

// ── buildBurstName ────────────────────────────────────────────────────────────

func TestBuildBurstName(t *testing.T) {
	cases := []struct {
		ext      string
		dateTime string
		idx      int
		want     string
	}{
		{"heic", "20190207_184125", 0, "IMG20190207184125_BURST000.heic"},
		{"heic", "20190207_184125", 1, "IMG20190207184125_BURST001.heic"},
		{"jpg", "20190207_184125", 0, "IMG_20190207_184125_BURST000.jpg"},
		{"jpg", "20190207_184125", 1, "IMG_20190207_184125_BURST001.jpg"},
	}
	for _, c := range cases {
		got := buildBurstName(c.ext, c.dateTime, c.idx)
		if got != c.want {
			t.Errorf("buildBurstName(%q, %q, %d): got %q, want %q", c.ext, c.dateTime, c.idx, got, c.want)
		}
	}
}

// ── detectBurstGroups ─────────────────────────────────────────────────────────

type fakeDirEntry struct {
	name string
}

func (f fakeDirEntry) Name() string               { return f.name }
func (f fakeDirEntry) IsDir() bool                { return false }
func (f fakeDirEntry) Type() os.FileMode          { return 0 }
func (f fakeDirEntry) Info() (os.FileInfo, error) { return nil, nil }

func fakeEntries(names ...string) []os.DirEntry {
	out := make([]os.DirEntry, len(names))
	for i, n := range names {
		out[i] = fakeDirEntry{n}
	}
	return out
}

func TestDetectBurstGroups(t *testing.T) {
	entries := fakeEntries(
		"20190207_184125_007.heic",
		"20190207_184125_009.heic",
		"20190207_184125_007.mp4", // video – must be ignored
		"20230101_120000_001.jpg", // single burst-like → not a group
		"photo.jpg",
	)
	groups := detectBurstGroups(entries)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	g, ok := groups["20190207_184125"]
	if !ok {
		t.Fatal("expected group key 20190207_184125")
	}
	if len(g) != 2 {
		t.Errorf("expected 2 files in group, got %d", len(g))
	}
}

// ── detectMp4Pairs ────────────────────────────────────────────────────────────

func TestDetectMp4Pairs(t *testing.T) {
	entries := fakeEntries(
		"photo.heic",
		"photo.mp4",  // paired
		"clip.mp4",   // no image companion → not paired
		"other.jpg",
	)
	pairs := detectMp4Pairs(entries)
	if len(pairs) != 1 {
		t.Fatalf("expected 1 pair, got %d", len(pairs))
	}
	if pairs["photo"] != "photo.mp4" {
		t.Errorf("expected pairs[photo]=photo.mp4, got %q", pairs["photo"])
	}
}

// ── Run (integration) ─────────────────────────────────────────────────────────

func writeFile(t *testing.T, dir, name string, mtime time.Time) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}
}

func TestRunNormalRename(t *testing.T) {
	dir := t.TempDir()
	tm := time.Date(2023, 1, 23, 10, 47, 7, 0, time.Local)
	writeFile(t, dir, "shot.heic", tm)
	writeFile(t, dir, "photo.jpg", tm)

	res, err := Run(Config{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if res.Renamed != 2 {
		t.Errorf("expected 2 renamed, got %d", res.Renamed)
	}
	if _, err := os.Stat(filepath.Join(dir, "IMG20230123104707.heic")); err != nil {
		t.Error("IMG20230123104707.heic not found")
	}
	if _, err := os.Stat(filepath.Join(dir, "IMG_20230123_104707.jpg")); err != nil {
		t.Error("IMG_20230123_104707.jpg not found")
	}
}

func TestRunBurstRename(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "20190207_184125_007.heic", time.Now())
	writeFile(t, dir, "20190207_184125_009.heic", time.Now())

	res, err := Run(Config{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if res.Renamed != 2 {
		t.Errorf("expected 2 renamed, got %d", res.Renamed)
	}
	if _, err := os.Stat(filepath.Join(dir, "IMG20190207184125_BURST000.heic")); err != nil {
		t.Error("BURST000 not found")
	}
	if _, err := os.Stat(filepath.Join(dir, "IMG20190207184125_BURST001.heic")); err != nil {
		t.Error("BURST001 not found")
	}
}

func TestRunMtimeConflict(t *testing.T) {
	dir := t.TempDir()
	tm := time.Date(2019, 4, 3, 16, 51, 10, 0, time.Local)
	writeFile(t, dir, "a.jpg", tm)
	writeFile(t, dir, "b.jpg", tm)

	res, err := Run(Config{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if res.Renamed != 2 {
		t.Errorf("expected 2 renamed, got %d", res.Renamed)
	}
	if _, err := os.Stat(filepath.Join(dir, "IMG_20190403_165110.jpg")); err != nil {
		t.Error("IMG_20190403_165110.jpg not found")
	}
	if _, err := os.Stat(filepath.Join(dir, "IMG_20190403_165110_001.jpg")); err != nil {
		t.Error("IMG_20190403_165110_001.jpg not found")
	}
}

func TestRunMp4Companion(t *testing.T) {
	dir := t.TempDir()
	tm := time.Date(2023, 1, 23, 10, 47, 7, 0, time.Local)
	writeFile(t, dir, "photo.heic", tm)
	writeFile(t, dir, "photo.mp4", tm)

	res, err := Run(Config{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if res.Renamed != 2 {
		t.Errorf("expected 2 renamed, got %d (heic+mp4)", res.Renamed)
	}
	if _, err := os.Stat(filepath.Join(dir, "IMG20230123104707.heic")); err != nil {
		t.Error("IMG20230123104707.heic not found")
	}
	if _, err := os.Stat(filepath.Join(dir, "IMG20230123104707.mp4")); err != nil {
		t.Error("IMG20230123104707.mp4 not found")
	}
}

// ── screenshot rename tests ─────────────────────────────────────────────────────

// mockDirEntry is a mock implementation of os.DirEntry for testing
type mockDirEntry struct {
	name  string
	isDir bool
}

func (m *mockDirEntry) Name() string               { return m.name }
func (m *mockDirEntry) IsDir() bool                { return m.isDir }
func (m *mockDirEntry) Type() os.FileMode          { return 0 }
func (m *mockDirEntry) Info() (os.FileInfo, error) { return nil, nil }

// ── TestDetectScreenshots ──────────────────────────────────────────────────────

func TestDetectScreenshots(t *testing.T) {
	cases := []struct {
		name     string
		entries  []os.DirEntry
		expected map[string]bool
		desc     string
	}{
		{
			name: "basic screenshot detection",
			entries: []os.DirEntry{
				&mockDirEntry{name: "Screenshot_2025-07-18-09-23-54-65.png", isDir: false},
				&mockDirEntry{name: "IMG_20230123_104707.jpg", isDir: false},
			},
			expected: map[string]bool{
				"Screenshot_2025-07-18-09-23-54-65.png": true,
			},
			desc: "Should detect .png screenshot and ignore IMG file",
		},
		{
			name: "case insensitive detection",
			entries: []os.DirEntry{
				&mockDirEntry{name: "screenshot_20250718_092354.jpg", isDir: false},
				&mockDirEntry{name: "SCREENSHOT_2025-07-18_09-23-54.jpeg", isDir: false},
			},
			expected: map[string]bool{
				"screenshot_20250718_092354.jpg":        true,
				"SCREENSHOT_2025-07-18_09-23-54.jpeg": true,
			},
			desc: "Should detect screenshots regardless of case",
		},
		{
			name: "ignore directories, detect any filename with screenshot prefix",
			entries: []os.DirEntry{
				&mockDirEntry{name: "screenshot", isDir: false},                          // No extension
				&mockDirEntry{name: "mmscreenshot1727421404387", isDir: false},            // No extension, Unix timestamp
				&mockDirEntry{name: "screenshot.txt", isDir: false},                      // Text file with screenshot prefix
				&mockDirEntry{name: "screenshot", isDir: true},                           // Directory
				&mockDirEntry{name: "Screenshot_2025-07-18-09-23-54-65.png", isDir: false}, // Image file
			},
			expected: map[string]bool{
				"screenshot":                            true,
				"mmscreenshot1727421404387":             true,
				"screenshot.txt":                        true,
				"Screenshot_2025-07-18-09-23-54-65.png": true,
			},
			desc: "Should detect any file starting with 'screenshot' (loose matching), ignore directories",
		},
		{
			name: "mixed file types",
			entries: []os.DirEntry{
				&mockDirEntry{name: "IMG_20230123_104707.jpg", isDir: false},
				&mockDirEntry{name: "Screenshot_2025-07-18-09-23-54-65.png", isDir: false},
				&mockDirEntry{name: "VID_20230123_104707.mp4", isDir: false},
				&mockDirEntry{name: "screenshot_20250718_092354.jpg", isDir: false},
			},
			expected: map[string]bool{
				"Screenshot_2025-07-18-09-23-54-65.png": true,
				"screenshot_20250718_092354.jpg":        true,
			},
			desc: "Should identify only screenshot files in mixed list",
		},
		{
			name:     "empty input",
			entries:  []os.DirEntry{},
			expected: map[string]bool{},
			desc:     "Should return empty map for no entries",
		},
		{
			name: "various image formats",
			entries: []os.DirEntry{
				&mockDirEntry{name: "screenshot_2025-07-18_09-23-54.png", isDir: false},
				&mockDirEntry{name: "screenshot_2025-07-18_09-23-54.jpg", isDir: false},
				&mockDirEntry{name: "screenshot_2025-07-18_09-23-54.jpeg", isDir: false},
				&mockDirEntry{name: "screenshot_2025-07-18_09-23-54.heic", isDir: false},
				&mockDirEntry{name: "screenshot_2025-07-18_09-23-54.webp", isDir: false},
			},
			expected: map[string]bool{
				"screenshot_2025-07-18_09-23-54.png":   true,
				"screenshot_2025-07-18_09-23-54.jpg":   true,
				"screenshot_2025-07-18_09-23-54.jpeg":  true,
				"screenshot_2025-07-18_09-23-54.heic":  true,
				"screenshot_2025-07-18_09-23-54.webp":  true,
			},
			desc: "Should detect screenshots with various image extensions",
		},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			result := detectScreenshots(tc.entries)
			if len(result) != len(tc.expected) {
				t.Errorf("detectScreenshots() returned %d entries, expected %d", len(result), len(tc.expected))
			}
			for name := range tc.expected {
				if !result[name] {
					t.Errorf("detectScreenshots() missing expected file: %s", name)
				}
			}
			for name := range result {
				if !tc.expected[name] {
					t.Errorf("detectScreenshots() returned unexpected file: %s", name)
				}
			}
		})
	}
}

// ── TestParseScreenshotTimestamp ──────────────────────────────────────────────

func TestParseScreenshotTimestamp(t *testing.T) {
	cases := []struct {
		filename     string
		expectedTime time.Time
		expectedOK   bool
		desc         string
	}{
		// Format 1: YYYY-MM-DD-HH-MM-SS-MS
		{
			filename:     "Screenshot_2025-07-18-09-23-54-65.png",
			expectedTime: time.Date(2025, 7, 18, 9, 23, 54, 650_000_000, time.UTC),
			expectedOK:   true,
			desc:         "Format 1: Complete timestamp with milliseconds",
		},
		{
			filename:     "Screenshot_2024-12-31-23-59-59-99.jpg",
			expectedTime: time.Date(2024, 12, 31, 23, 59, 59, 990_000_000, time.UTC),
			expectedOK:   true,
			desc:         "Format 1: Boundary values (end of year)",
		},
		{
			filename:     "screenshot_2025-01-01-00-00-00-00.jpeg",
			expectedTime: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			expectedOK:   true,
			desc:         "Format 1: Boundary values (start of year)",
		},

		// Format 2: YYYYMMDD_HHMMSS
		{
			filename:     "screenshot20250718_092354.jpg",
			expectedTime: time.Date(2025, 7, 18, 9, 23, 54, 0, time.UTC),
			expectedOK:   true,
			desc:         "Format 2: Compact format without milliseconds",
		},
		{
			filename:     "Screenshot20240101_000000.png",
			expectedTime: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			expectedOK:   true,
			desc:         "Format 2: Boundary (start of year)",
		},

		// Format 3: YYYY-MM-DD_HH-MM-SS
		{
			filename:     "Screenshot_2025-07-18_09-23-54.png",
			expectedTime: time.Date(2025, 7, 18, 9, 23, 54, 0, time.UTC),
			expectedOK:   true,
			desc:         "Format 3: Mixed separator format",
		},
		{
			filename:     "screenshot_2024-06-15_14-30-22.jpg",
			expectedTime: time.Date(2024, 6, 15, 14, 30, 22, 0, time.UTC),
			expectedOK:   true,
			desc:         "Format 3: Mid-year timestamp",
		},

		// Format 4: YYYY_M_D_H_M_S (auto zero-padding)
		{
			filename:     "screenshot_2025_7_18_9_23_54.png",
			expectedTime: time.Date(2025, 7, 18, 9, 23, 54, 0, time.UTC),
			expectedOK:   true,
			desc:         "Format 4: Auto zero-padding (single digits)",
		},
		{
			filename:     "screenshot_2024_1_1_0_0_0.jpg",
			expectedTime: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			expectedOK:   true,
			desc:         "Format 4: Boundary with auto zero-padding",
		},
		{
			filename:     "screenshot_2025_12_31_23_59_59.jpeg",
			expectedTime: time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC),
			expectedOK:   true,
			desc:         "Format 4: End of year",
		},

		// Format 5: YYYY-MM-DD (date only, incomplete)
		{
			filename:     "screenshot_2025-07-18.png",
			expectedTime: time.Date(2025, 7, 18, 0, 0, 0, 0, time.UTC),
			expectedOK:   false,
			desc:         "Format 5: Date only (incomplete, OK=false)",
		},
		{
			filename:     "Screenshot_2024-12-31.jpg",
			expectedTime: time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC),
			expectedOK:   false,
			desc:         "Format 5: Year end date only",
		},

		// Format 6a: Unix timestamp seconds (10 digits)
		{
			filename:     "screenshot1634560000.jpg",
			expectedTime: time.Date(2021, 10, 18, 12, 26, 40, 0, time.UTC),
			expectedOK:   true,
			desc:         "Format 6a: Unix timestamp seconds (10 digits)",
		},
		{
			filename:     "screenshot1634560000.png",
			expectedTime: time.Date(2021, 10, 18, 12, 26, 40, 0, time.UTC),
			expectedOK:   true,
			desc:         "Format 6a: Unix timestamp seconds with different extension",
		},

		// Format 6b: Unix timestamp milliseconds (13 digits)
		{
			filename:     "mmscreenshot1727421404387.jpg",
			expectedTime: time.Date(2024, 9, 27, 7, 16, 44, 387_000_000, time.UTC),
			expectedOK:   true,
			desc:         "Format 6b: WeChat-style Unix timestamp milliseconds (13 digits)",
		},
		{
			filename:     "mmscreenshot1727421404387",
			expectedTime: time.Date(2024, 9, 27, 7, 16, 44, 387_000_000, time.UTC),
			expectedOK:   true,
			desc:         "Format 6b: Unix timestamp milliseconds without extension",
		},
		{
			filename:     "wxscreenshot1727421404387.png",
			expectedTime: time.Date(2024, 9, 27, 7, 16, 44, 387_000_000, time.UTC),
			expectedOK:   true,
			desc:         "Format 6b: wx-style Unix timestamp milliseconds",
		},
		{
			filename:     "screenshot1634560000000.jpg",
			expectedTime: time.Date(2021, 10, 18, 12, 26, 40, 0, time.UTC),
			expectedOK:   true,
			desc:         "Format 6b: Unix timestamp milliseconds (round second)",
		},

		// Invalid formats
		{
			filename:     "screenshot.png",
			expectedTime: time.Time{},
			expectedOK:   false,
			desc:         "Invalid: No timestamp",
		},
		{
			filename:     "invalid_2025-07-18-09-23-54.jpg",
			expectedTime: time.Time{},
			expectedOK:   false,
			desc:         "Invalid: Wrong filename prefix",
		},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			result, ok := parseScreenshotTimestamp(tc.filename)
			if ok != tc.expectedOK {
				t.Errorf("parseScreenshotTimestamp(%q) ok=%v, expected ok=%v", tc.filename, ok, tc.expectedOK)
			}
			// Compare time values (allow small differences due to nanosecond precision)
			if ok && !result.Equal(tc.expectedTime) {
				t.Errorf("parseScreenshotTimestamp(%q) returned %v, expected %v", tc.filename, result, tc.expectedTime)
			}
			if !ok && tc.expectedOK && !result.Equal(tc.expectedTime) {
				// For incomplete formats (ok=false), we still check the date portion matches
				if result.Year() != tc.expectedTime.Year() || result.Month() != tc.expectedTime.Month() || result.Day() != tc.expectedTime.Day() {
					t.Errorf("parseScreenshotTimestamp(%q) date mismatch: %v vs %v", tc.filename, result, tc.expectedTime)
				}
			}
		})
	}
}

// ── TestBuildScreenshotName ────────────────────────────────────────────────────

func TestBuildScreenshotName(t *testing.T) {
	cases := []struct {
		ext      string
		t        time.Time
		expected string
		desc     string
	}{
		{
			ext:      "png",
			t:        time.Date(2025, 7, 18, 9, 23, 54, 654_321_000, time.UTC),
			expected: "Screenshot_2025-07-18-09-23-54-65.png",
			desc:     "Standard format with milliseconds calculation",
		},
		{
			ext:      "jpg",
			t:        time.Date(2024, 1, 15, 14, 30, 22, 500_000_000, time.UTC),
			expected: "Screenshot_2024-01-15-14-30-22-50.jpg",
			desc:     "JPG format with 50 milliseconds",
		},
		{
			ext:      "jpeg",
			t:        time.Date(2025, 12, 31, 23, 59, 59, 999_000_000, time.UTC),
			expected: "Screenshot_2025-12-31-23-59-59-99.jpeg",
			desc:     "Year boundary with max milliseconds",
		},
		{
			ext:      "heic",
			t:        time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
			expected: "Screenshot_2024-06-01-00-00-00-00.heic",
			desc:     "HEIC format with zero milliseconds",
		},
		{
			ext:      "PNG",
			t:        time.Date(2025, 7, 18, 9, 23, 54, 654_321_000, time.UTC),
			expected: "Screenshot_2025-07-18-09-23-54-65.png",
			desc:     "Uppercase extension should be converted to lowercase",
		},
		{
			ext:      "JPEG",
			t:        time.Date(2024, 3, 15, 12, 30, 45, 123_000_000, time.UTC),
			expected: "Screenshot_2024-03-15-12-30-45-12.jpeg",
			desc:     "Uppercase JPEG extension",
		},
		{
			ext:      "webp",
			t:        time.Date(2025, 2, 28, 8, 15, 30, 12_000_000, time.UTC),
			expected: "Screenshot_2025-02-28-08-15-30-01.webp",
			desc:     "WEBP format",
		},
		{
			ext:      "jpg",
			t:        time.Date(2024, 5, 5, 5, 5, 5, 5_000_000, time.UTC),
			expected: "Screenshot_2024-05-05-05-05-05-00.jpg",
			desc:     "Nanosecond value under 10ms threshold",
		},
		{
			ext:      "png",
			t:        time.Date(2025, 1, 1, 0, 0, 0, 1_000_000, time.UTC),
			expected: "Screenshot_2025-01-01-00-00-00-00.png",
			desc:     "Very small nanosecond value (1ms)",
		},
		{
			ext:      "jpg",
			t:        time.Date(2024, 12, 25, 20, 30, 15, 999_999_999, time.UTC),
			expected: "Screenshot_2024-12-25-20-30-15-99.jpg",
			desc:     "Max nanosecond value (should clamp to 99)",
		},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			result := buildScreenshotName(tc.ext, tc.t)
			if result != tc.expected {
				t.Errorf("buildScreenshotName(%q, %v) = %q, expected %q", tc.ext, tc.t, result, tc.expected)
			}
		})
	}
}
