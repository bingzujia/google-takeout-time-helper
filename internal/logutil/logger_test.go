package logutil

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestOpenLog_FirstRun(t *testing.T) {
	dir := t.TempDir()
	logger, err := OpenLog(dir, "migrate", false)
	if err != nil {
		t.Fatalf("OpenLog: %v", err)
	}
	defer logger.Close()

	if !strings.HasSuffix(logger.Path(), "migrate-") &&
		!strings.Contains(logger.Path(), "migrate-") {
		t.Errorf("unexpected path: %s", logger.Path())
	}
	if !strings.HasSuffix(logger.Path(), "-001.log") {
		t.Errorf("expected -001.log suffix, got: %s", logger.Path())
	}
}

func TestOpenLog_SecondRun(t *testing.T) {
	dir := t.TempDir()

	l1, err := OpenLog(dir, "migrate", false)
	if err != nil {
		t.Fatalf("first OpenLog: %v", err)
	}
	l1.Close()

	l2, err := OpenLog(dir, "migrate", false)
	if err != nil {
		t.Fatalf("second OpenLog: %v", err)
	}
	defer l2.Close()

	if !strings.HasSuffix(l2.Path(), "-002.log") {
		t.Errorf("expected -002.log suffix, got: %s", l2.Path())
	}
}

func TestOpenLog_DryRun(t *testing.T) {
	dir := t.TempDir()
	logger, err := OpenLog(dir, "migrate", true)
	if err != nil {
		t.Fatalf("OpenLog dry-run: %v", err)
	}
	defer logger.Close()

	if logger.Path() != "" {
		t.Errorf("dry-run should return empty path, got: %s", logger.Path())
	}

	logDir := filepath.Join(dir, "gtoh-log")
	if _, err := os.Stat(logDir); !os.IsNotExist(err) {
		t.Errorf("dry-run should not create gtoh-log dir")
	}
}

func TestOpenLog_DifferentCommandsIndependentIndex(t *testing.T) {
	dir := t.TempDir()

	l1, _ := OpenLog(dir, "fix-exif", false)
	l1.Close()

	l2, err := OpenLog(dir, "dedup", false)
	if err != nil {
		t.Fatalf("second command OpenLog: %v", err)
	}
	defer l2.Close()

	if !strings.HasSuffix(l2.Path(), "-001.log") {
		t.Errorf("dedup first run should be -001.log, got: %s", l2.Path())
	}
}

func TestLogger_ConcurrentWrites(t *testing.T) {
	dir := t.TempDir()
	logger, err := OpenLog(dir, "test", false)
	if err != nil {
		t.Fatalf("OpenLog: %v", err)
	}
	defer logger.Close()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			logger.Info("processed", "/some/file.jpg")
			logger.Skip("duplicate", "/other/file.jpg")
			logger.Fail("write-exif", "/bad/file.jpg", "exit status 1")
		}(i)
	}
	wg.Wait()

	// Read back the log and verify no partial lines.
	content, err := os.ReadFile(logger.Path())
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(content), "\n"), "\n")
	if len(lines) != 60 {
		t.Errorf("expected 60 lines (20×3), got %d", len(lines))
	}
	for _, line := range lines {
		if !strings.HasPrefix(line, "[") {
			t.Errorf("malformed log line: %q", line)
		}
	}
}

func TestNop(t *testing.T) {
	logger := Nop()
	// Should not panic or error.
	logger.Info("test", "/file.jpg")
	logger.Skip("already-done", "/file.jpg")
	logger.Fail("error", "/file.jpg", "detail")
	if err := logger.Close(); err != nil {
		t.Errorf("Nop.Close: %v", err)
	}
	if logger.Path() != "" {
		t.Errorf("Nop path should be empty")
	}
}
