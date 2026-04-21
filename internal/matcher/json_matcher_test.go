package matcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// --- methodIdentity ---

func TestMethodIdentity(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"photo.jpg", "photo.jpg"},
		{"IMG_20230302_112040.jpg", "IMG_20230302_112040.jpg"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := methodIdentity(tt.input); got != tt.expected {
			t.Errorf("methodIdentity(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// --- methodShortenName ---

func TestMethodShortenName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "short filename unchanged",
			input:    "photo.jpg",
			expected: "photo.jpg",
		},
		{
			name:     "exactly 46 chars unchanged",
			input:    "abcdefghijklmnopqrstuvwxyz0123456789012345.jpg", // 46 chars
			expected: "abcdefghijklmnopqrstuvwxyz0123456789012345.jpg",
		},
		{
			name:     "47 chars truncated to 46",
			input:    "abcdefghijklmnopqrstuvwxyz01234567890123456.jpg", // 47 chars
			expected: "abcdefghijklmnopqrstuvwxyz01234567890123456.jp",
		},
		{
			name:     "very long filename truncated",
			input:    "very_long_filename_that_exceeds_fifty_one_characters_limit.jpg",
			expected: "very_long_filename_that_exceeds_fifty_one_char",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := methodShortenName(tt.input); got != tt.expected {
				t.Errorf("methodShortenName(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// --- methodBracketSwap ---

func TestMethodBracketSwap(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no bracket unchanged",
			input:    "normal.jpg",
			expected: "normal.jpg",
		},
		{
			name:     "single bracket swap",
			input:    "image(11).jpg",
			expected: "image.jpg(11)",
		},
		{
			name:     "single digit bracket",
			input:    "photo(1).png",
			expected: "photo.png(1)",
		},
		{
			name:     "multiple brackets uses last",
			input:    "image(3).(2)(3).jpg",
			expected: "image(3).(2).jpg(3)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := methodBracketSwap(tt.input); got != tt.expected {
				t.Errorf("methodBracketSwap(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// --- methodRemoveExtra ---

func TestMethodRemoveExtra(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no extra unchanged",
			input:    "normal.jpg",
			expected: "normal.jpg",
		},
		{
			name:     "english edited",
			input:    "photo-edited.jpg",
			expected: "photo.jpg",
		},
		{
			name:     "german bearbeitet",
			input:    "urlaub-bearbeitet.jpg",
			expected: "urlaub.jpg",
		},
		{
			name:     "french modifié",
			input:    "photo-modifié.jpg",
			expected: "photo.jpg",
		},
		{
			name:     "chinese 已修改",
			input:    "photo-已修改.jpg",
			expected: "photo.jpg",
		},
		{
			name:     "chinese 编辑",
			input:    "photo-编辑.jpg",
			expected: "photo.jpg",
		},
		{
			name:     "chinese 修改",
			input:    "photo-修改.jpg",
			expected: "photo.jpg",
		},
		{
			name:     "japanese 編集済み",
			input:    "photo-編集済み.jpg",
			expected: "photo.jpg",
		},
		{
			name:     "spanish ha editado",
			input:    "photo-ha editado.jpg",
			expected: "photo.jpg",
		},
		{
			name:     "multiple occurrences removes last",
			input:    "my-edited-photo-edited.jpg",
			expected: "my-edited-photo.jpg",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := methodRemoveExtra(tt.input); got != tt.expected {
				t.Errorf("methodRemoveExtra(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// --- methodNoExtension ---

func TestMethodNoExtension(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"20030616.jpg", "20030616"},
		{"photo.png", "photo"},
		{"archive.tar.gz", "archive.tar"},
		{"noext", "noext"},
	}
	for _, tt := range tests {
		if got := methodNoExtension(tt.input); got != tt.expected {
			t.Errorf("methodNoExtension(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// --- replaceLast ---

func TestReplaceLast(t *testing.T) {
	tests := []struct {
		s, old, new string
		expected    string
	}{
		{"my-edited-photo-edited.jpg", "-edited", "", "my-edited-photo.jpg"},
		{"normal.jpg", "-edited", "", "normal.jpg"},
		{"a-b-c.jpg", "-", "_", "a-b_c.jpg"},
		{"", "x", "y", ""},
	}
	for _, tt := range tests {
		if got := replaceLast(tt.s, tt.old, tt.new); got != tt.expected {
			t.Errorf("replaceLast(%q, %q, %q) = %q, want %q", tt.s, tt.old, tt.new, got, tt.expected)
		}
	}
}

// --- Integration: JSONForFile ---

func TestJSONForFile(t *testing.T) {
	// Create a temporary directory with test files
	tmpDir := t.TempDir()

	// Test case 1: exact match
	writeFile(t, tmpDir, "photo.jpg", "fake image content")
	writeJSON(t, tmpDir, "photo.jpg.json", `{"photoTakenTime":{"timestamp":"1683012040"}}`)

	// Test case 2: edited suffix
	writeFile(t, tmpDir, "vacation-edited.jpg", "fake image content")
	writeJSON(t, tmpDir, "vacation.jpg.json", `{"photoTakenTime":{"timestamp":"1683012040"}}`)

	// Test case 3: bracket swap
	writeFile(t, tmpDir, "image(11).jpg", "fake image content")
	writeJSON(t, tmpDir, "image.jpg(11).json", `{"photoTakenTime":{"timestamp":"1683012040"}}`)

	// Test case 4: no extension
	writeFile(t, tmpDir, "20030616.jpg", "fake image content")
	writeJSON(t, tmpDir, "20030616.json", `{"photoTakenTime":{"timestamp":"1683012040"}}`)

	// Test case 5: chinese suffix
	writeFile(t, tmpDir, "photo-已修改.jpg", "fake image content")
	writeJSON(t, tmpDir, "photo.jpg.json", `{"photoTakenTime":{"timestamp":"1683012040"}}`)

	// Test case 6: supplemental-metadata suffix
	writeFile(t, tmpDir, "144xeigf2qsw9f42ruvsjc5ii.jpg", "fake image content")
	writeJSON(t, tmpDir, "144xeigf2qsw9f42ruvsjc5ii.jpg.supplemental-met.json", `{"photoTakenTime":{"timestamp":"1683012040"}}`)

	// Test case 7: supplemental regex fallback (arbitrary truncation)
	writeFile(t, tmpDir, "3EC9B98B2A283DED25AD727147740EFE.png", "fake image content")
	writeJSON(t, tmpDir, "3EC9B98B2A283DED25AD727147740EFE.png.s.json", `{"photoTakenTime":{"timestamp":"1683012040"}}`)

	// Test case 8: supplemental with numbered duplicate — (N) moves from photo to JSON suffix
	writeFile(t, tmpDir, "IMG20240405102259(1).heic", "fake image content")
	writeJSON(t, tmpDir, "IMG20240405102259.heic.supplemental-metadata(1).json", `{"photoTakenTime":{"timestamp":"1683012040"}}`)

	// Test case 9: supplemental without numbered duplicate
	writeFile(t, tmpDir, "IMG20240405102259.heic", "fake image content")
	writeJSON(t, tmpDir, "IMG20240405102259.heic.supplemental-metadata.json", `{"photoTakenTime":{"timestamp":"1683012040"}}`)

	// Test case 10: chinese suffix with IMG prefix and timestamp
	writeFile(t, tmpDir, "IMG_20210629_114736-已修改.jpg", "fake image content")
	writeJSON(t, tmpDir, "IMG_20210629_114736.jpg.json", `{"photoTakenTime":{"timestamp":"1683012040"}}`)

	// Test case 11: chinese suffix + supplemental-metadata (combined)
	writeFile(t, tmpDir, "IMG_20210630_120000-已修改.jpg", "fake image content")
	writeJSON(t, tmpDir, "IMG_20210630_120000.jpg.supplemental-metadata.json", `{"photoTakenTime":{"timestamp":"1683012040"}}`)

	// Test case 12: double-dot JSON naming
	writeFile(t, tmpDir, "v2-60f3fcfa38e5d175d410eb7180efc9ad_r-01.jpeg", "fake image content")
	writeJSON(t, tmpDir, "v2-60f3fcfa38e5d175d410eb7180efc9ad_r-01.jpeg..json", `{"photoTakenTime":{"timestamp":"1683012040"}}`)

	// Test case 13: no matching JSON
	writeFile(t, tmpDir, "unknown.jpg", "fake image content")

	tests := []struct {
		name        string
		photoName   string
		expectFound bool
	}{
		{"exact match", "photo.jpg", true},
		{"edited suffix", "vacation-edited.jpg", true},
		{"bracket swap", "image(11).jpg", true},
		{"no extension", "20030616.jpg", true},
		{"chinese suffix", "photo-已修改.jpg", true},
		{"supplemental-met suffix", "144xeigf2qsw9f42ruvsjc5ii.jpg", true},
		{"supplemental regex fallback", "3EC9B98B2A283DED25AD727147740EFE.png", true},
		{"supplemental numbered duplicate", "IMG20240405102259(1).heic", true},
		{"supplemental no duplicate", "IMG20240405102259.heic", true},
		{"IMG with chinese suffix", "IMG_20210629_114736-已修改.jpg", true},
		{"IMG chinese suffix + supplemental", "IMG_20210630_120000-已修改.jpg", true},
		{"double-dot JSON", "v2-60f3fcfa38e5d175d410eb7180efc9ad_r-01.jpeg", true},
		{"no matching json", "unknown.jpg", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			photoPath := filepath.Join(tmpDir, tt.photoName)
			result := JSONForFile(photoPath, nil)

			if tt.expectFound {
				if result == nil {
					t.Errorf("JSONForFile(%q) = nil, want non-nil", tt.photoName)
				} else if result.Timestamp.IsZero() {
					t.Errorf("JSONForFile(%q) timestamp is zero", tt.photoName)
				}
			} else {
				if result != nil {
					t.Errorf("JSONForFile(%q) = %+v, want nil", tt.photoName, result)
				}
			}
		})
	}
}

// --- ResolveTimestamp ---

func TestResolveTimestamp(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a photo with a parseable filename
	photoPath := filepath.Join(tmpDir, "IMG_20230302_112040.jpg")
	writeFile(t, tmpDir, "IMG_20230302_112040.jpg", "fake content")

	gp := &GooglePhoto{}
	gp.PhotoTakenTime.Timestamp = "1683012040" // 2023-05-02 10:20:40 UTC

	ts := ResolveTimestamp(photoPath, gp)

	// Filename timestamp should take priority: 2023-03-02 11:20:40 UTC
	expected := time.Date(2023, 3, 2, 11, 20, 40, 0, time.UTC)
	if !ts.Equal(expected) {
		t.Errorf("ResolveTimestamp() = %v, want %v", ts, expected)
	}

	// Test JSON fallback (unparseable filename)
	photoPath2 := filepath.Join(tmpDir, "unknown_name.jpg")
	writeFile(t, tmpDir, "unknown_name.jpg", "fake content")
	ts2 := ResolveTimestamp(photoPath2, gp)
	expected2 := time.Unix(1683012040, 0).UTC()
	if !ts2.Equal(expected2) {
		t.Errorf("ResolveTimestamp() fallback = %v, want %v", ts2, expected2)
	}

	// Test zero time (both fail)
	gp2 := &GooglePhoto{}
	ts3 := ResolveTimestamp(photoPath2, gp2)
	if !ts3.IsZero() {
		t.Errorf("ResolveTimestamp() = %v, want zero time", ts3)
	}
}

// --- Helpers ---

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write file %s: %v", name, err)
	}
}

func writeJSON(t *testing.T, dir, name, content string) {
	t.Helper()
	writeFile(t, dir, name, content)
}

// TestCreationTimeExtraction tests that creationTime is properly extracted from JSON sidecars
func TestCreationTimeExtraction(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name           string
		jsonContent    string
		expectPhotoTs  int64
		expectCreatTs  int64
	}{
		{
			name: "both photoTakenTime and creationTime present",
			jsonContent: `{
				"photoTakenTime":{"timestamp":"1683012040"},
				"creationTime":{"timestamp":"1683015000"}
			}`,
			expectPhotoTs: 1683012040,
			expectCreatTs: 1683015000,
		},
		{
			name: "only photoTakenTime present",
			jsonContent: `{
				"photoTakenTime":{"timestamp":"1683012040"}
			}`,
			expectPhotoTs: 1683012040,
			expectCreatTs: 0,
		},
		{
			name: "only creationTime present",
			jsonContent: `{
				"creationTime":{"timestamp":"1683015000"}
			}`,
			expectPhotoTs: 0,
			expectCreatTs: 1683015000,
		},
		{
			name:          "neither timestamp present",
			jsonContent:   `{}`,
			expectPhotoTs: 0,
			expectCreatTs: 0,
		},
		{
			name: "invalid photoTakenTime timestamp",
			jsonContent: `{
				"photoTakenTime":{"timestamp":"invalid"},
				"creationTime":{"timestamp":"1683015000"}
			}`,
			expectPhotoTs: 0,
			expectCreatTs: 1683015000,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			photoFile := "photo.jpg"
			writeFile(t, tmpDir, photoFile, "fake image")
			writeJSON(t, tmpDir, photoFile+".json", tc.jsonContent)

			result := JSONForFile(filepath.Join(tmpDir, photoFile), &DirCache{})
			if result == nil {
				t.Fatal("JSONForFile returned nil")
			}

			if result.PhotoTakenTimeUnix != tc.expectPhotoTs {
				t.Errorf("PhotoTakenTimeUnix = %d, want %d", result.PhotoTakenTimeUnix, tc.expectPhotoTs)
			}

			if result.CreationTimeUnix != tc.expectCreatTs {
				t.Errorf("CreationTimeUnix = %d, want %d", result.CreationTimeUnix, tc.expectCreatTs)
			}
		})
	}
}

