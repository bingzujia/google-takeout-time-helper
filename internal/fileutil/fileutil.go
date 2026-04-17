package fileutil

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CopyFile copies src to dst, preserving the source file's mtime.
// For cross-device moves, it performs a copy+delete.
func CopyFile(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat src: %w", err)
	}

	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open src: %w", err)
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
	if err != nil {
		return fmt.Errorf("create dst: %w", err)
	}
	defer out.Close()

	if _, err = io.Copy(out, in); err != nil {
		os.Remove(dst)
		return fmt.Errorf("copy: %w", err)
	}
	if err = out.Close(); err != nil {
		return fmt.Errorf("close dst: %w", err)
	}

	return os.Chtimes(dst, info.ModTime(), info.ModTime())
}

// ResolveDestPath returns filepath.Join(destDir, name). If that path already
// exists, a timestamp suffix (YYYYMMDDHHMMSS) is appended to the stem to avoid
// overwriting the existing file.
func ResolveDestPath(destDir, name string) string {
	target := filepath.Join(destDir, name)
	if _, err := os.Stat(target); os.IsNotExist(err) {
		return target
	}
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	suffix := time.Now().Format("20060102150405")
	return filepath.Join(destDir, fmt.Sprintf("%s_%s%s", stem, suffix, ext))
}
