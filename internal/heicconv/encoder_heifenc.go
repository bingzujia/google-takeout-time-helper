package heicconv

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// heifEncQuality is the default quality factor passed to heif-enc (0–100; higher = better quality / larger file).
// 75 balances perceptual fidelity with file-size reduction and is appropriate for both photos and
// text-heavy screenshots. Users can override this via the --quality CLI flag.
// Lossless (-L) is intentionally excluded; it defeats the compression goal and is never passed to heif-enc.
const heifEncQuality = 75

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
	q := heifEncQuality
	if opts.Quality > 0 {
		q = opts.Quality
	}
	args := []string{"-q", fmt.Sprintf("%d", q)}
	if opts.ChromaSubsampling != "" {
		args = append(args, "-p", "chroma="+opts.ChromaSubsampling)
	}
	args = append(args, srcPath, "-o", dstPath)
	return args
}

// detectChromaSubsampling returns the chroma subsampling value for srcPath.
// For JPEG sources it first checks chromaMap (pre-queried batch result), then falls back
// to calling exiftool directly. Non-JPEG formats and any parse failure fall back to "420".
//
// NOTE: Always returns "420" regardless of source. HEIC files branded as "heic" must use
// HEVC Main or Main Still Picture profile, both of which require YCbCr 4:2:0. Passing
// chroma=444 causes heif-enc/x265 to select HEVC Range Extensions profile (profile_idc=4),
// which is incompatible with the "heic" brand — every major platform decoder (Apple
// ImageIO, Android MediaCodec, Windows HEVC codec) rejects such files as corrupt.
func detectChromaSubsampling() string {
	return "420"
}
