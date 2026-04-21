package heicconv

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Error variables for heif-convert related errors
var (
	ErrHeifConvertNotFound    = errors.New("heif-convert not found")
	ErrHeifConvertVersionOld  = errors.New("heif-convert version too old")
	ErrHeifConvertDecodeError = errors.New("heif-convert decode error")
	ErrHeifConvertIOError     = errors.New("heif-convert io error")
)

const (
	// decodeTimeout is the context deadline for a single HEIC decode operation
	decodeTimeout = 30 * time.Second
	// versionCheckTimeout is the timeout for checking heif-convert version
	versionCheckTimeout = 5 * time.Second
)

// LookupPath checks if heif-convert tool is available and meets minimum version requirement (1.1+).
// Returns (found bool, error).
// If tool is found and version is correct, returns (true, nil).
// If tool is not found, returns (false, ErrHeifConvertNotFound).
// If tool version is too old, returns (false, ErrHeifConvertVersionOld).
func LookupPath() (found bool, err error) {
	// Check if heif-convert is in PATH
	path, err := exec.LookPath("heif-convert")
	if err != nil {
		return false, fmt.Errorf("%w: %v", ErrHeifConvertNotFound, err)
	}

	// Check version
	major, minor, err := detectHeifConvertVersion()
	if err != nil {
		// If we can't detect version, assume tool is available but version check failed
		return false, fmt.Errorf("failed to detect heif-convert version: %v", err)
	}

	// Verify version >= 1.1
	if major < 1 || (major == 1 && minor < 1) {
		return false, fmt.Errorf("%w: version %d.%d < required 1.1", ErrHeifConvertVersionOld, major, minor)
	}

	_ = path // path is available
	return true, nil
}

// detectHeifConvertVersion detects the version of heif-convert tool.
// Returns (major, minor, error).
// Note: Some versions of heif-convert don't support --version flag,
// in which case we assume version 1.1+ (safe assumption for modern systems)
func detectHeifConvertVersion() (major, minor int, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), versionCheckTimeout)
	defer cancel()

	// Try --version first
	cmd := exec.CommandContext(ctx, "heif-convert", "--version")
	out, err := cmd.CombinedOutput()
	if err == nil {
		// Parse version string: "heif-convert X.Y.Z ..." or "X.Y"
		versionStr := strings.TrimSpace(string(out))
		major, minor, err := parseVersionString(versionStr)
		if err == nil {
			return major, minor, nil
		}
		// Fall through to assume version 1.1
	}

	// If --version fails or tool doesn't support it,
	// assume version 1.1+ (modern heif-convert has this)
	return 1, 1, nil
}

// parseVersionString parses version from string like "heif-convert 1.1.0" or "1.1"
func parseVersionString(versionStr string) (major, minor int, err error) {
	parts := strings.Fields(versionStr)
	if len(parts) == 0 {
		return 0, 0, fmt.Errorf("empty version string")
	}

	// Try to find version number in output
	var versionPart string
	for _, part := range parts {
		if strings.Contains(part, ".") && isVersionLike(part) {
			versionPart = part
			break
		}
	}

	if versionPart == "" {
		// Try the last part (sometimes it's just the version number)
		versionPart = parts[len(parts)-1]
	}

	// Parse "X.Y.Z" or "X.Y"
	versionComponents := strings.Split(versionPart, ".")
	if len(versionComponents) < 2 {
		return 0, 0, fmt.Errorf("unable to parse version from: %s", versionStr)
	}

	major, err = strconv.Atoi(versionComponents[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid major version: %s", versionComponents[0])
	}

	minor, err = strconv.Atoi(versionComponents[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid minor version: %s", versionComponents[1])
	}

	return major, minor, nil
}

// isVersionLike checks if a string looks like a version number.
func isVersionLike(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) < 2 {
		return false
	}
	for _, part := range parts {
		if _, err := strconv.Atoi(part); err != nil {
			return false
		}
	}
	return true
}

// ConvertFromHEIC decodes a HEIC/HEIF source file to a temporary JPEG file.
// Returns (workPath, cleanup function, error).
// The cleanup function must be called (typically via defer) to remove the temporary file.
func ConvertFromHEIC(heicPath, jpegPath string) (workPath string, cleanup func() error, err error) {
	// Check if source file exists
	if _, err := os.Stat(heicPath); err != nil {
		if os.IsNotExist(err) {
			return "", nil, fmt.Errorf("%w: source file not found: %s", ErrHeifConvertIOError, err)
		}
		return "", nil, fmt.Errorf("%w: cannot stat source file: %s", ErrHeifConvertIOError, err)
	}

	// Generate temporary JPEG path
	tmpJPEGPath := createTempJPEGPath(heicPath)

	// Create cleanup function upfront
	cleanup = func() error {
		if err := os.Remove(tmpJPEGPath); err != nil && !os.IsNotExist(err) {
			// Return error but don't fail if file doesn't exist
			return err
		}
		return nil
	}

	// Invoke heif-convert to decode HEIC
	if err := invokeHeifConvert(heicPath, tmpJPEGPath); err != nil {
		// Clean up on error
		_ = cleanup()
		return "", nil, err
	}

	// Verify temporary JPEG was created
	if _, err := os.Stat(tmpJPEGPath); err != nil {
		_ = cleanup()
		return "", nil, fmt.Errorf("%w: temporary JPEG was not created: %v", ErrHeifConvertIOError, err)
	}

	return tmpJPEGPath, cleanup, nil
}

// createTempJPEGPath generates a unique temporary JPEG file path.
// Format: <source_dir>/<stem>.tmp.<pid>.<nanotime>.jpg
func createTempJPEGPath(sourcePath string) string {
	dir := filepath.Dir(sourcePath)
	filename := filepath.Base(sourcePath)
	ext := filepath.Ext(filename)
	stem := strings.TrimSuffix(filename, ext)

	pid := os.Getpid()
	nanotime := time.Now().UnixNano()

	tmpName := fmt.Sprintf("%s.tmp.%d.%d.jpg", stem, pid, nanotime)
	return filepath.Join(dir, tmpName)
}

// invokeHeifConvert executes heif-convert to decode HEIC to JPEG.
func invokeHeifConvert(heicPath, jpegPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), decodeTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "heif-convert", heicPath, jpegPath)
	out, err := cmd.CombinedOutput()

	if err != nil {
		stderr := strings.TrimSpace(string(out))

		// Check for specific error types
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("%w: decode timed out after %v: %s", ErrHeifConvertDecodeError, decodeTimeout, stderr)
		}

		// Wrap error based on stderr content
		wrappedErr := wrapHeifConvertError(stderr, err)
		return wrappedErr
	}

	return nil
}

// wrapHeifConvertError wraps heif-convert errors into appropriate error types.
func wrapHeifConvertError(stderr string, exitErr error) error {
	stderrLower := strings.ToLower(stderr)

	// Check for "not found" or "command not found"
	if strings.Contains(stderrLower, "not found") || strings.Contains(stderrLower, "no such file") {
		return fmt.Errorf("%w: %v", ErrHeifConvertNotFound, exitErr)
	}

	// Check for version-related errors (unlikely at decode time)
	if strings.Contains(stderrLower, "version") {
		return fmt.Errorf("%w: %v", ErrHeifConvertVersionOld, exitErr)
	}

	// Check for IO-related errors
	if strings.Contains(stderrLower, "permission denied") || strings.Contains(stderrLower, "disk") {
		return fmt.Errorf("%w: %v: %s", ErrHeifConvertIOError, exitErr, stderr)
	}

	// Default to decode error
	return fmt.Errorf("%w: %v: %s", ErrHeifConvertDecodeError, exitErr, stderr)
}
