package heicconv

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// heifEncQuality is the quality factor passed to heif-enc (0–100; higher = better quality / larger file).
// 35 provides a significant file-size reduction over the previous 80 while remaining perceptually
// acceptable for photo archives. Lossless (-L) is intentionally excluded; it defeats the
// compression goal and is never passed to heif-enc.
const heifEncQuality = 35

// normalEncodeTimeout is the context deadline for a single non-oversized HEIC encode.
const normalEncodeTimeout = 5 * time.Minute

// oversizedEncodeTimeout is the extended deadline applied to oversized HEIC encodes.
const oversizedEncodeTimeout = 30 * time.Minute

// encodeTimeout returns the context deadline for a single HEIC encode.
func encodeTimeout(opts EncodeOptions) time.Duration {
	if opts.Oversized {
		return oversizedEncodeTimeout
	}
	return normalEncodeTimeout
}

// heifEncEncoder encodes images to HEIC by invoking the system heif-enc binary
// (from the libheif-examples package).
type heifEncEncoder struct{}

func newHeifEncEncoder() encoder {
	return heifEncEncoder{}
}

// ValidateHeifEncSupport returns an error if heif-enc is absent from PATH.
func ValidateHeifEncSupport() error {
	if _, err := exec.LookPath("heif-enc"); err != nil {
		return fmt.Errorf("heif-enc not found in PATH: %v", err)
	}
	return nil
}

// ValidateEncoderSupport returns nil if heif-enc is available, otherwise returns
// an error with install instructions for libheif-examples.
func ValidateEncoderSupport() error {
	if err := ValidateHeifEncSupport(); err != nil {
		return fmt.Errorf(
			"no supported HEIC encoder found:\n  heif-enc: %v\n\nInstall heif-enc on Debian/Ubuntu:\n  sudo apt-get install -y libheif-examples",
			err,
		)
	}
	return nil
}

// Encode invokes heif-enc to convert srcPath into a HEIC file at dstPath.
func (heifEncEncoder) Encode(srcPath, dstPath string, opts EncodeOptions) error {
	timeout := encodeTimeout(opts)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	args := buildHeifEncArgs(srcPath, dstPath, opts)
	cmd := exec.CommandContext(ctx, "heif-enc", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("heif-enc encode timed out after %v: %s", timeout, strings.TrimSpace(string(out)))
		}
		return fmt.Errorf("heif-enc encode: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// buildHeifEncArgs constructs the argument list for a heif-enc HEIC encode.
// Separated from Encode so the argument composition can be unit-tested without
// actually invoking heif-enc.
//
// Note: -L (lossless) is intentionally never included; it defeats the compression goal.
func buildHeifEncArgs(srcPath, dstPath string, opts EncodeOptions) []string {
	args := []string{"-q", fmt.Sprintf("%d", heifEncQuality)}
	if opts.ChromaSubsampling != "" {
		args = append(args, "-p", "chroma="+opts.ChromaSubsampling)
	}
	args = append(args, srcPath, "-o", dstPath)
	return args
}

// detectChromaSubsampling returns the chroma subsampling value for srcPath.
// For JPEG sources it calls exiftool to read the YCbCrSubSampling tag and maps
// the result to "420", "422", or "444". Non-JPEG formats and any parse failure
// fall back to "420".
func detectChromaSubsampling(srcPath, format string) string {
	if format != "jpeg" {
		return "420"
	}
	out, err := exec.Command("exiftool", "-j", "-YCbCrSubSampling", srcPath).Output()
	if err != nil {
		return "420"
	}
	return parseChromaSubsampling(string(out))
}

// parseChromaSubsampling extracts a chroma subsampling value ("420", "422", "444")
// from the JSON output of `exiftool -j -YCbCrSubSampling`. Defaults to "420".
func parseChromaSubsampling(exiftoolJSON string) string {
	var records []map[string]interface{}
	if err := json.Unmarshal([]byte(exiftoolJSON), &records); err != nil || len(records) == 0 {
		return "420"
	}
	raw, ok := records[0]["YCbCrSubSampling"]
	if !ok {
		return "420"
	}
	val := strings.ToLower(fmt.Sprintf("%v", raw))
	switch {
	case strings.Contains(val, "4:4:4"):
		return "444"
	case strings.Contains(val, "4:2:2"):
		return "422"
	default:
		return "420"
	}
}
