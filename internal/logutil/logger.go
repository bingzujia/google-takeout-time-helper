package logutil

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	logDirNameOnce sync.Once
	logDirNameVal  string
)

// LogDirName returns the log directory name derived from the running binary:
// filepath.Base(os.Args[0]) + "-log" (e.g. "takeout-helper-log").
// The result is computed once and cached.
func LogDirName() string {
	logDirNameOnce.Do(func() {
		bin := filepath.Base(os.Args[0])
		// Strip any OS-specific extension (e.g. ".exe" on Windows).
		if ext := filepath.Ext(bin); ext != "" {
			bin = strings.TrimSuffix(bin, ext)
		}
		logDirNameVal = bin + "-log"
	})
	return logDirNameVal
}

// Logger writes structured log entries to a file.
// It is safe to use from multiple goroutines concurrently.
type Logger struct {
	f    *os.File // nil for no-op logger
	path string
	mu   sync.Mutex
}

// Path returns the resolved log file path (empty for no-op logger).
func (l *Logger) Path() string { return l.path }

// Info logs an informational entry.
func (l *Logger) Info(reason, path string) {
	l.write("INFO", reason, path, "")
}

// Skip logs a skipped-file entry.
func (l *Logger) Skip(reason, path string) {
	l.write("SKIP", reason, path, "")
}

// Fail logs a failure entry with an optional detail string.
func (l *Logger) Fail(reason, path, detail string) {
	l.write("FAIL", reason, path, detail)
}

// Close closes the underlying log file. Safe to call on a no-op logger.
func (l *Logger) Close() error {
	if l.f == nil {
		return nil
	}
	return l.f.Close()
}

func (l *Logger) write(level, reason, path, detail string) {
	if l.f == nil {
		return
	}
	ts := time.Now().Format("2006-01-02 15:04:05")
	var line string
	if detail != "" {
		line = fmt.Sprintf("[%s] %s %s: %s (%s)\n", ts, level, reason, path, detail)
	} else {
		line = fmt.Sprintf("[%s] %s %s: %s\n", ts, level, reason, path)
	}
	l.mu.Lock()
	fmt.Fprint(l.f, line)
	l.mu.Unlock()
}

// Nop returns a no-op Logger that discards all writes and creates no files.
func Nop() *Logger {
	return &Logger{}
}

// indexRe matches "command-YYYY-MM-DD-NNN.log" and captures NNN.
var indexRe = regexp.MustCompile(`-(\d{3})\.log$`)

// OpenLog opens (or creates) a log file at:
//
//	<baseDir>/<LogDirName()>/<command>-<date>-<NNN>.log
//
// where date is today's date in YYYY-MM-DD and NNN is auto-incremented per-day.
// The log directory name is derived from the running binary name (e.g. "takeout-helper-log").
// If dryRun is true, a no-op logger is returned without touching the filesystem.
func OpenLog(baseDir, command string, dryRun bool) (*Logger, error) {
	if dryRun {
		return Nop(), nil
	}

	logDir := filepath.Join(baseDir, LogDirName())
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("create log dir %q: %w", logDir, err)
	}

	date := time.Now().UTC().Format("2006-01-02")
	prefix := command + "-" + date + "-"
	next := nextIndex(logDir, prefix)

	filename := fmt.Sprintf("%s%03d.log", prefix, next)
	logPath := filepath.Join(logDir, filename)

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("open log file %q: %w", logPath, err)
	}
	return &Logger{f: f, path: logPath}, nil
}

// nextIndex scans logDir for files starting with prefix and returns max(existing)+1.
func nextIndex(logDir, prefix string) int {
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return 1
	}
	max := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		m := indexRe.FindStringSubmatch(name)
		if m == nil {
			continue
		}
		n, err := strconv.Atoi(m[1])
		if err == nil && n > max {
			max = n
		}
	}
	return max + 1
}
