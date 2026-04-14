package cmd

import (
	"testing"
	"time"
)

// resolveTimestamp is tested here because it encapsulates the EXIF-then-filename
// fallback logic. ParseEXIFTimestamp will return false for non-existent /
// non-image paths, which naturally exercises the filename fallback path.

func TestResolveTimestamp_FilenameOnly(t *testing.T) {
	// A path whose filename encodes a timestamp but has no real EXIF data.
	// ParseEXIFTimestamp will return false (no real file / no EXIF),
	// so the filename fallback must succeed.
	filePath := "/tmp/IMG20250409084814.jpg"

	got, src, ok := resolveTimestamp(filePath)
	if !ok {
		t.Fatal("expected resolveTimestamp to return ok=true via filename fallback")
	}
	if src != "filename" {
		t.Errorf("expected source=filename, got %q", src)
	}

	want := time.Date(2025, 4, 9, 8, 48, 14, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("expected timestamp %v, got %v", want, got)
	}
}

func TestResolveTimestamp_NoTimestamp(t *testing.T) {
	// A path whose filename contains no recognisable timestamp and has no EXIF.
	filePath := "/tmp/unknown_photo.jpg"

	_, _, ok := resolveTimestamp(filePath)
	if ok {
		t.Error("expected resolveTimestamp to return ok=false when both EXIF and filename yield no timestamp")
	}
}
