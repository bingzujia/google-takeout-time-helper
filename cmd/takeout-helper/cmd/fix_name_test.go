package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bingzujia/google-takeout-time-helper/internal/parser"
)

func TestRunFixNameFiles_NoFilenameDate(t *testing.T) {
	// Files without a parseable datetime in their name must be counted as skipped.
	_, _, skipped := runFixNameFiles([]string{"/tmp/unknown_photo.jpg"}, fixNameRunOptions{
		WriteAll:     func(string, time.Time) error { return nil },
		WorkerCount:  1,
		ShowProgress: false,
	})
	if skipped != 1 {
		t.Fatalf("skipped = %d, want 1", skipped)
	}
}

func TestRunFixNameFiles_WriteSuccess(t *testing.T) {
	// Any file with a parseable filename datetime is always written, regardless of EXIF.
	var written []string
	processed, failed, skipped := runFixNameFiles([]string{"/tmp/IMG20240102030405.jpg"}, fixNameRunOptions{
		WriteAll: func(filePath string, _ time.Time) error {
			written = append(written, filepath.Base(filePath))
			return nil
		},
		WorkerCount:  1,
		ShowProgress: false,
	})
	if processed != 1 || failed != 0 || skipped != 0 {
		t.Fatalf("processed=%d failed=%d skipped=%d, want 1 0 0", processed, failed, skipped)
	}
	if len(written) != 1 || written[0] != "IMG20240102030405.jpg" {
		t.Fatalf("written = %v", written)
	}
}

func TestRunFixNameFiles_WriteFailed(t *testing.T) {
	var logs []string
	processed, failed, skipped := runFixNameFiles([]string{"/tmp/IMG20240102030405.jpg"}, fixNameRunOptions{
		WriteAll: func(string, time.Time) error {
			return errors.New("exiftool error")
		},
		WriteLog: func(filePath, detail string) {
			logs = append(logs, filepath.Base(filePath)+":"+detail)
		},
		WorkerCount:  1,
		ShowProgress: false,
	})
	if processed != 0 || failed != 1 || skipped != 0 {
		t.Fatalf("processed=%d failed=%d skipped=%d, want 0 1 0", processed, failed, skipped)
	}
	if len(logs) != 1 || logs[0] != "IMG20240102030405.jpg:exiftool error" {
		t.Fatalf("logs = %v", logs)
	}
}

func TestRunFixNameFiles_ExtensionMismatch(t *testing.T) {
	dir := t.TempDir()
	// File whose stem encodes a timestamp but extension is wrong.
	orig := filepath.Join(dir, "IMG20240102030405.png") // actually a JPEG
	if err := os.WriteFile(orig, []byte("fake jpeg bytes"), 0644); err != nil {
		t.Fatal(err)
	}

	var infos []string
	var writtenPaths []string

	processed, failed, skipped := runFixNameFiles([]string{orig}, fixNameRunOptions{
		PrepareFile: func(filePath string) (string, func() error, string, error) {
			tmpPath := strings.TrimSuffix(filePath, filepath.Ext(filePath)) + ".jpg"
			if err := os.Rename(filePath, tmpPath); err != nil {
				return "", nil, "", err
			}
			return tmpPath, func() error { return os.Rename(tmpPath, filePath) }, "extension mismatch: .png→.jpg", nil
		},
		WriteAll: func(filePath string, _ time.Time) error {
			writtenPaths = append(writtenPaths, filepath.Base(filePath))
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
	// WriteAll must have been called with the .jpg path.
	if len(writtenPaths) != 1 || writtenPaths[0] != "IMG20240102030405.jpg" {
		t.Fatalf("writtenPaths = %v", writtenPaths)
	}
	// Original .png must be restored.
	if _, err := os.Stat(orig); err != nil {
		t.Fatalf("original file not restored: %v", err)
	}
	// Mismatch must be logged via LogInfo.
	if len(infos) != 1 || infos[0] != "IMG20240102030405.png:extension mismatch: .png→.jpg" {
		t.Fatalf("infos = %v", infos)
	}
}

func TestRunFixNameFiles_PXLConvertsUTCToLocal(t *testing.T) {
	// PXL_ filenames embed UTC. The written timestamp must be the local-time
	// representation of that UTC instant; no EXIF comparison is done.
	var writtenTime time.Time
	processed, failed, skipped := runFixNameFiles([]string{"/tmp/PXL_20240101_060000123.jpg"}, fixNameRunOptions{
		WriteAll: func(_ string, ts time.Time) error {
			writtenTime = ts
			return nil
		},
		WorkerCount:  1,
		ShowProgress: false,
	})
	if processed != 1 || failed != 0 || skipped != 0 {
		t.Fatalf("processed=%d failed=%d skipped=%d, want 1 0 0", processed, failed, skipped)
	}

	// ParseFilenameTimestamp returns UTC; isPXLFile converts to local.
	// Verify the written time is in time.Local and has the same instant as the UTC parse.
	parsedUTC, ok := parser.ParseFilenameTimestamp("PXL_20240101_060000123.jpg")
	if !ok {
		t.Fatal("parser did not recognise PXL_ filename")
	}
	want := parsedUTC.In(time.Local)
	if !writtenTime.Equal(want) {
		t.Errorf("writtenTime = %v, want %v", writtenTime, want)
	}
	if writtenTime.Location() != time.Local {
		t.Errorf("writtenTime.Location() = %v, want Local", writtenTime.Location())
	}
}

func TestRunFixNameFiles_PrepareFileError_Skips(t *testing.T) {
	var skipLogs []string
	processed, failed, skipped := runFixNameFiles([]string{"/tmp/IMG20240102030405.png"}, fixNameRunOptions{
		PrepareFile: func(filePath string) (string, func() error, string, error) {
			return "", nil, "", errors.New("unknown file type: application/octet-stream")
		},
		WriteAll: func(string, time.Time) error { return nil },
		LogSkip: func(filePath, reason string) {
			skipLogs = append(skipLogs, filepath.Base(filePath)+":"+reason)
		},
		WorkerCount:  1,
		ShowProgress: false,
	})
	if processed != 0 || failed != 0 || skipped != 1 {
		t.Fatalf("processed=%d failed=%d skipped=%d, want 0 0 1", processed, failed, skipped)
	}
	if len(skipLogs) != 1 || skipLogs[0] != "IMG20240102030405.png:unknown file type: application/octet-stream" {
		t.Fatalf("skipLogs = %v", skipLogs)
	}
}

// TestFixName_mmexport_TimezoneConversion tests that mmexport filenames
// (Unix timestamp format) are converted from UTC to local timezone.
func TestFixName_mmexport_TimezoneConversion(t *testing.T) {
	// mmexport1491013330299 → 1491013330 seconds → 2017-04-01 09:15:30 UTC
	// When converted to local timezone, it should reflect the local time
	var writtenTime time.Time
	processed, failed, skipped := runFixNameFiles([]string{"/tmp/mmexport1491013330299.jpg"}, fixNameRunOptions{
		WriteAll: func(_ string, ts time.Time) error {
			writtenTime = ts
			return nil
		},
		WorkerCount:  1,
		ShowProgress: false,
	})

	if processed != 1 || failed != 0 || skipped != 0 {
		t.Fatalf("processed=%d failed=%d skipped=%d, want 1 0 0", processed, failed, skipped)
	}

	// Parse the filename to get the expected UTC time
	parsedUTC, ok := parser.ParseFilenameTimestamp("mmexport1491013330299.jpg")
	if !ok {
		t.Fatal("parser did not recognize mmexport filename")
	}

	// For Unix timestamp formats, the time should be converted to local timezone
	want := parsedUTC.In(time.Local)
	if !writtenTime.Equal(want) {
		t.Errorf("writtenTime = %v, want %v (both represent same instant in different timezones)", writtenTime, want)
	}
}

// TestFixName_album_temp_TimezoneConversion tests that album_temp filenames
// (Unix timestamp format) are converted from UTC to local timezone.
func TestFixName_album_temp_TimezoneConversion(t *testing.T) {
	// album_temp__ss6f5323f071bd7f7b6f521e8ss_1769347547.jpg → 1769347547 seconds → 2025-11-24 08:25:47 UTC
	// When converted to local timezone, it should reflect the local time
	var writtenTime time.Time
	processed, failed, skipped := runFixNameFiles([]string{"/tmp/album_temp__ss6f5323f071bd7f7b6f521e8ss_1769347547.jpg"}, fixNameRunOptions{
		WriteAll: func(_ string, ts time.Time) error {
			writtenTime = ts
			return nil
		},
		WorkerCount:  1,
		ShowProgress: false,
	})

	if processed != 1 || failed != 0 || skipped != 0 {
		t.Fatalf("processed=%d failed=%d skipped=%d, want 1 0 0", processed, failed, skipped)
	}

	// Parse the filename to get the expected UTC time
	parsedUTC, ok := parser.ParseFilenameTimestamp("album_temp__ss6f5323f071bd7f7b6f521e8ss_1769347547.jpg")
	if !ok {
		t.Fatal("parser did not recognize album_temp filename")
	}

	// For Unix timestamp formats, the time should be converted to local timezone
	want := parsedUTC.In(time.Local)
	if !writtenTime.Equal(want) {
		t.Errorf("writtenTime = %v, want %v (both represent same instant in different timezones)", writtenTime, want)
	}
}

// TestFixName_explicit_datetime_no_conversion tests that explicit datetime formats
// (e.g., IMG_20250415_144530) are NOT converted to local timezone (kept as UTC).
func TestFixName_explicit_datetime_no_conversion(t *testing.T) {
	// IMG_20250415_144530 → 2025-04-15 14:45:30 UTC
	// This format should NOT be converted; it should remain UTC
	var writtenTime time.Time
	processed, failed, skipped := runFixNameFiles([]string{"/tmp/IMG_20250415_144530.jpg"}, fixNameRunOptions{
		WriteAll: func(_ string, ts time.Time) error {
			writtenTime = ts
			return nil
		},
		WorkerCount:  1,
		ShowProgress: false,
	})

	if processed != 1 || failed != 0 || skipped != 0 {
		t.Fatalf("processed=%d failed=%d skipped=%d, want 1 0 0", processed, failed, skipped)
	}

	// Parse the filename to get the expected UTC time
	parsedUTC, ok := parser.ParseFilenameTimestamp("IMG_20250415_144530.jpg")
	if !ok {
		t.Fatal("parser did not recognize IMG_ filename")
	}

	// For explicit datetime formats, the time should remain UTC (no conversion)
	if !writtenTime.Equal(parsedUTC) {
		t.Errorf("writtenTime = %v, want %v (UTC, no conversion)", writtenTime, parsedUTC)
	}
	if writtenTime.Location() != time.UTC {
		t.Errorf("writtenTime.Location() = %v, want UTC", writtenTime.Location())
	}
}

// TestFixName_pxl_file_unchanged tests that PXL files continue to use the existing
// timezone conversion logic (unchanged from original behavior).
func TestFixName_pxl_file_unchanged(t *testing.T) {
	// PXL_20240101_060000123 → 2024-01-01 06:00:00 UTC
	// Existing behavior: convert UTC to local timezone
	var writtenTime time.Time
	processed, failed, skipped := runFixNameFiles([]string{"/tmp/PXL_20240101_060000123.jpg"}, fixNameRunOptions{
		WriteAll: func(_ string, ts time.Time) error {
			writtenTime = ts
			return nil
		},
		WorkerCount:  1,
		ShowProgress: false,
	})

	if processed != 1 || failed != 0 || skipped != 0 {
		t.Fatalf("processed=%d failed=%d skipped=%d, want 1 0 0", processed, failed, skipped)
	}

	// Parse the filename to get the expected UTC time
	parsedUTC, ok := parser.ParseFilenameTimestamp("PXL_20240101_060000123.jpg")
	if !ok {
		t.Fatal("parser did not recognize PXL_ filename")
	}

	// PXL files should be converted to local timezone (existing behavior preserved)
	want := parsedUTC.In(time.Local)
	if !writtenTime.Equal(want) {
		t.Errorf("writtenTime = %v, want %v", writtenTime, want)
	}
	if writtenTime.Location() != time.Local {
		t.Errorf("writtenTime.Location() = %v, want Local", writtenTime.Location())
	}
}
