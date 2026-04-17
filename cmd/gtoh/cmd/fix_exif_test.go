package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
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

func TestCollectMediaFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "photo.jpg"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "nested"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "nested", "inside.jpg"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	files, skipped, err := collectMediaFiles(dir)
	if err != nil {
		t.Fatalf("collectMediaFiles() error = %v", err)
	}
	if skipped != 1 {
		t.Fatalf("skipped = %d, want 1", skipped)
	}
	if len(files) != 1 {
		t.Fatalf("len(files) = %d, want 1", len(files))
	}
	if got, want := filepath.Base(files[0]), "photo.jpg"; got != want {
		t.Fatalf("files[0] = %q, want %q", got, want)
	}
}

func TestCollectMediaFiles_UppercaseExtension(t *testing.T) {
	dir := t.TempDir()
	// Uppercase extensions must be recognised the same as lowercase.
	for _, name := range []string{"PHOTO.JPG", "video.MP4", "image.HEIC"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "doc.PDF"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	files, skipped, err := collectMediaFiles(dir)
	if err != nil {
		t.Fatalf("collectMediaFiles() error = %v", err)
	}
	if skipped != 1 {
		t.Fatalf("skipped = %d, want 1 (doc.PDF)", skipped)
	}
	if len(files) != 3 {
		t.Fatalf("len(files) = %d, want 3", len(files))
	}
}

func TestRunFixExifFiles_CountsAndLogging(t *testing.T) {
	mediaFiles := []string{
		"/tmp/exif-ok.jpg",
		"/tmp/filename-ok.jpg",
		"/tmp/missing.jpg",
		"/tmp/write-fail.jpg",
	}

	var logs []string
	var failures []string

	processed, failed := runFixExifFiles(mediaFiles, fixExifRunOptions{
		ResolveTimestamp: func(filePath string) (time.Time, string, bool) {
			switch filepath.Base(filePath) {
			case "exif-ok.jpg":
				return time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC), "exif", true
			case "filename-ok.jpg":
				return time.Date(2024, 2, 3, 4, 5, 6, 0, time.UTC), "filename", true
			case "write-fail.jpg":
				return time.Date(2024, 3, 4, 5, 6, 7, 0, time.UTC), "exif", true
			default:
				return time.Time{}, "", false
			}
		},
		WriteTimestamp: func(filePath string, _ time.Time) error {
			if filepath.Base(filePath) == "write-fail.jpg" {
				return errors.New("write failed")
			}
			return nil
		},
		WriteLog: func(filePath, detail string) {
			logs = append(logs, filepath.Base(filePath)+":"+detail)
		},
		ReportFailure: func(filePath, detail string) {
			failures = append(failures, filepath.Base(filePath)+":"+detail)
		},
		WorkerCount:  2,
		ShowProgress: false,
	})

	if processed != 2 || failed != 2 {
		t.Fatalf("processed=%d failed=%d, want 2 and 2", processed, failed)
	}

	sort.Strings(logs)
	wantLogs := []string{
		"filename-ok.jpg:no DateTimeOriginal; timestamp from filename",
		"missing.jpg:no DateTimeOriginal and no filename timestamp",
		"write-fail.jpg:write failed",
	}
	sort.Strings(wantLogs)
	if len(logs) != len(wantLogs) {
		t.Fatalf("log count = %d, want %d (%v)", len(logs), len(wantLogs), logs)
	}
	for i := range wantLogs {
		if logs[i] != wantLogs[i] {
			t.Fatalf("logs[%d] = %q, want %q", i, logs[i], wantLogs[i])
		}
	}

	sort.Strings(failures)
	wantFailures := []string{
		"missing.jpg:no DateTimeOriginal and no filename timestamp",
		"write-fail.jpg:write failed",
	}
	for i := range wantFailures {
		if failures[i] != wantFailures[i] {
			t.Fatalf("failures[%d] = %q, want %q", i, failures[i], wantFailures[i])
		}
	}
}

func TestRunFixExifFiles_DryRunDoesNotWrite(t *testing.T) {
	mediaFiles := []string{"/tmp/a.jpg", "/tmp/b.jpg"}
	reported := 0
	writes := 0

	processed, failed := runFixExifFiles(mediaFiles, fixExifRunOptions{
		DryRun: true,
		ResolveTimestamp: func(filePath string) (time.Time, string, bool) {
			if filepath.Base(filePath) == "a.jpg" {
				return time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC), "filename", true
			}
			return time.Time{}, "", false
		},
		WriteTimestamp: func(string, time.Time) error {
			writes++
			return nil
		},
		ReportDryRun: func(string, time.Time, string, bool) {
			reported++
		},
		WorkerCount:  2,
		ShowProgress: false,
	})

	if processed != 0 || failed != 0 {
		t.Fatalf("processed=%d failed=%d, want 0 and 0", processed, failed)
	}
	if reported != 2 {
		t.Fatalf("reported = %d, want 2", reported)
	}
	if writes != 0 {
		t.Fatalf("writes = %d, want 0", writes)
	}
}
