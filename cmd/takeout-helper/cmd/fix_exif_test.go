package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// resolveTimestamp is tested here to verify it reads only EXIF DateTimeOriginal
// and does NOT fall back to the filename.

func TestResolveTimestamp_NoEXIF(t *testing.T) {
	// A path whose filename encodes a timestamp but has no real EXIF data.
	// fix-exif must NOT use the filename — resolveTimestamp should return false.
	filePath := "/tmp/IMG20250409084814.jpg"

	_, ok := resolveTimestamp(filePath)
	if ok {
		t.Fatal("expected resolveTimestamp to return ok=false (no EXIF on non-existent file; filename fallback must not apply)")
	}
}

func TestResolveTimestamp_NoTimestamp(t *testing.T) {
	// A path whose filename contains no recognisable timestamp and has no EXIF.
	filePath := "/tmp/unknown_photo.jpg"

	_, ok := resolveTimestamp(filePath)
	if ok {
		t.Error("expected resolveTimestamp to return ok=false when there is no EXIF DateTimeOriginal")
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
		"/tmp/no-exif.jpg",
		"/tmp/write-fail.jpg",
	}

	var writeLogs []string
	var skipLogs []string
	var failures []string

	processed, failed, skipped := runFixExifFiles(mediaFiles, fixExifRunOptions{
		ResolveTimestamp: func(filePath string) (time.Time, bool) {
			switch filepath.Base(filePath) {
			case "exif-ok.jpg":
				return time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC), true
			case "write-fail.jpg":
				return time.Date(2024, 3, 4, 5, 6, 7, 0, time.UTC), true
			default:
				return time.Time{}, false
			}
		},
		WriteTimestamp: func(filePath string, _ time.Time) error {
			if filepath.Base(filePath) == "write-fail.jpg" {
				return errors.New("write failed")
			}
			return nil
		},
		WriteLog: func(filePath, detail string) {
			writeLogs = append(writeLogs, filepath.Base(filePath)+":"+detail)
		},
		LogSkip: func(filePath, reason string) {
			skipLogs = append(skipLogs, filepath.Base(filePath)+":"+reason)
		},
		ReportFailure: func(filePath, detail string) {
			failures = append(failures, filepath.Base(filePath)+":"+detail)
		},
		WorkerCount:  2,
		ShowProgress: false,
	})

	if processed != 1 || failed != 1 || skipped != 1 {
		t.Fatalf("processed=%d failed=%d skipped=%d, want 1 1 1", processed, failed, skipped)
	}

	sort.Strings(writeLogs)
	wantWriteLogs := []string{"write-fail.jpg:write failed"}
	if len(writeLogs) != len(wantWriteLogs) {
		t.Fatalf("writeLogs = %v, want %v", writeLogs, wantWriteLogs)
	}
	for i := range wantWriteLogs {
		if writeLogs[i] != wantWriteLogs[i] {
			t.Fatalf("writeLogs[%d] = %q, want %q", i, writeLogs[i], wantWriteLogs[i])
		}
	}

	sort.Strings(skipLogs)
	wantSkipLogs := []string{"no-exif.jpg:no DateTimeOriginal"}
	if len(skipLogs) != len(wantSkipLogs) {
		t.Fatalf("skipLogs = %v, want %v", skipLogs, wantSkipLogs)
	}
	for i := range wantSkipLogs {
		if skipLogs[i] != wantSkipLogs[i] {
			t.Fatalf("skipLogs[%d] = %q, want %q", i, skipLogs[i], wantSkipLogs[i])
		}
	}

	sort.Strings(failures)
	wantFailures := []string{"write-fail.jpg:write failed"}
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

	processed, failed, _ := runFixExifFiles(mediaFiles, fixExifRunOptions{
		DryRun: true,
		ResolveTimestamp: func(filePath string) (time.Time, bool) {
			if filepath.Base(filePath) == "a.jpg" {
				return time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC), true
			}
			return time.Time{}, false
		},
		WriteTimestamp: func(string, time.Time) error {
			writes++
			return nil
		},
		ReportDryRun: func(string, time.Time, bool) {
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

func TestRunFixExifFiles_ExtensionMismatch(t *testing.T) {
	dir := t.TempDir()
	orig := filepath.Join(dir, "photo.png") // actually a JPEG
	if err := os.WriteFile(orig, []byte("fake jpeg bytes"), 0644); err != nil {
		t.Fatal(err)
	}

	var infos []string

	processed, failed, skipped := runFixExifFiles([]string{orig}, fixExifRunOptions{
		PrepareFile: func(filePath string) (string, func() error, string, error) {
			// Simulate: actual type is JPEG, needs rename to .jpg
			tmpPath := strings.TrimSuffix(filePath, filepath.Ext(filePath)) + ".jpg"
			if err := os.Rename(filePath, tmpPath); err != nil {
				return "", nil, "", err
			}
			return tmpPath, func() error { return os.Rename(tmpPath, filePath) }, "extension mismatch: .png→.jpg", nil
		},
		ResolveTimestamp: func(filePath string) (time.Time, bool) {
			if strings.HasSuffix(filePath, ".jpg") {
				return time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC), true
			}
			return time.Time{}, false
		},
		WriteTimestamp: func(filePath string, _ time.Time) error {
			if !strings.HasSuffix(filePath, ".jpg") {
				return errors.New("wrong extension")
			}
			return nil
		},
		LogInfo: func(filePath, detail string) {
			infos = append(infos, filepath.Base(filePath)+":"+detail)
		},
		WorkerCount:  1,
		ShowProgress: false,
	})

	if processed != 1 || failed != 0 || skipped != 0 {
		t.Fatalf("processed=%d failed=%d skipped=%d, want 1 0 0", processed, failed, skipped)
	}
	// Original filename must be restored after processing.
	if _, err := os.Stat(orig); err != nil {
		t.Fatalf("original file not restored: %v", err)
	}
	if len(infos) != 1 || infos[0] != "photo.png:extension mismatch: .png→.jpg" {
		t.Fatalf("infos = %v, want [photo.png:extension mismatch: .png→.jpg]", infos)
	}
}

func TestRunFixExifFiles_PrepareFileError_Skips(t *testing.T) {
	var skipLogs []string

	processed, failed, skipped := runFixExifFiles([]string{"/tmp/bad.png"}, fixExifRunOptions{
		PrepareFile: func(filePath string) (string, func() error, string, error) {
			return "", nil, "", errors.New("unknown file type: application/octet-stream")
		},
		ResolveTimestamp: func(filePath string) (time.Time, bool) {
			return time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), true
		},
		LogSkip: func(filePath, reason string) {
			skipLogs = append(skipLogs, filepath.Base(filePath)+":"+reason)
		},
		WorkerCount:  1,
		ShowProgress: false,
	})

	if processed != 0 || failed != 0 || skipped != 1 {
		t.Fatalf("processed=%d failed=%d skipped=%d, want 0 0 1", processed, failed, skipped)
	}
	if len(skipLogs) != 1 || skipLogs[0] != "bad.png:unknown file type: application/octet-stream" {
		t.Fatalf("skipLogs = %v", skipLogs)
	}
}
