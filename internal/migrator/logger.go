package migrator

import (
	"fmt"
	"os"
	"time"
)

// Logger writes structured log entries to a file.
type Logger struct {
	f *os.File
}

// NewLogger creates a log file at the given path.
func NewLogger(path string) (*Logger, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("create log file: %w", err)
	}
	return &Logger{f: f}, nil
}

// Close closes the underlying file.
func (l *Logger) Close() error {
	return l.f.Close()
}

// Skip logs a skipped file entry.
func (l *Logger) Skip(reason, path string) {
	l.write("SKIP", reason, path, "")
}

// Fail logs a failed file entry.
func (l *Logger) Fail(reason, path, detail string) {
	l.write("FAIL", reason, path, detail)
}

// Info logs an informational entry (e.g. no JSON sidecar).
func (l *Logger) Info(reason, path string) {
	l.write("INFO", reason, path, "")
}

func (l *Logger) write(level, reason, path, detail string) {
	ts := time.Now().Format("2006-01-02 15:04:05")
	if detail != "" {
		fmt.Fprintf(l.f, "[%s] %s %s: %s (%s)\n", ts, level, reason, path, detail)
	} else {
		fmt.Fprintf(l.f, "[%s] %s %s: %s\n", ts, level, reason, path)
	}
}
