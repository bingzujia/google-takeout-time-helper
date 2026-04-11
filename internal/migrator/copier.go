package migrator

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// CopyAndHash copies src to dst (flat, in destDir) while computing SHA-256.
// Returns the destination path, SHA-256 hex string, and whether the file already existed.
// This is a single-pass operation: the file is read once and both written and hashed.
func CopyAndHash(src, destDir string) (dstPath, sha256Hex string, exists bool, err error) {
	name := filepath.Base(src)
	dstPath = filepath.Join(destDir, name)

	// Check if destination already exists
	if _, err := os.Stat(dstPath); err == nil {
		return dstPath, "", true, nil
	}

	srcF, err := os.Open(src)
	if err != nil {
		return dstPath, "", false, fmt.Errorf("open source: %w", err)
	}
	defer srcF.Close()

	dstF, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return dstPath, "", false, fmt.Errorf("create dest: %w", err)
	}
	defer dstF.Close()

	h := sha256.New()
	if _, err := io.Copy(dstF, io.TeeReader(srcF, h)); err != nil {
		os.Remove(dstPath) // clean up partial copy
		return dstPath, "", false, fmt.Errorf("copy: %w", err)
	}

	sha256Hex = hex.EncodeToString(h.Sum(nil))

	// Preserve original mtime/atime
	srcInfo, err := srcF.Stat()
	if err == nil {
		os.Chtimes(dstPath, srcInfo.ModTime(), srcInfo.ModTime())
	}

	return dstPath, sha256Hex, false, nil
}
