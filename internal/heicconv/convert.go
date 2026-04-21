package heicconv

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

// ErrorKind identifies which stage of conversion failed.
type ErrorKind string

const (
	ErrorKindDecode   ErrorKind = "decode"
	ErrorKindEncode   ErrorKind = "encode"
	ErrorKindMetadata ErrorKind = "metadata"
)

// oversizedPixelThreshold is the pixel count (width × height) above which an image
// is treated as oversized and receives stricter encode controls to reduce OOM risk.
const oversizedPixelThreshold int64 = 40_000_000

// MaxDecodeDimension is the maximum width or height (in pixels) that a source image
// may have before conversion is refused. Apple ImageIO rejects HEIC with any
// dimension > 16383, and most hardware HEVC decoders cap at 16384 px.
const MaxDecodeDimension = 16383

// ErrAlreadyHEIC indicates that the source file is already HEIC/HEIF content.
var ErrAlreadyHEIC = errors.New("source image is already HEIC/HEIF")

// ErrDimensionTooLarge indicates that a source image dimension exceeds MaxDecodeDimension
// and the resulting HEIC would be undecodable on most consumer devices.
var ErrDimensionTooLarge = errors.New("source image dimension exceeds maximum HEIC decode limit")

// Error wraps a stage-specific conversion error.
type Error struct {
	Kind ErrorKind
	Path string
	Err  error
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s error for %s: %v", e.Kind, e.Path, e.Err)
}

func (e *Error) Unwrap() error {
	return e.Err
}

// EncodeOptions parameterizes a single HEIC encode operation.
type EncodeOptions struct {
	// Oversized indicates the image exceeds the oversized pixel threshold;
	// the encoder applies stricter memory controls (fewer threads, explicit pix_fmt).
	Oversized bool
	// ChromaSubsampling is the chroma subsampling value to pass to the encoder
	// ("420", "422", or "444"). Empty string means use the encoder default.
	ChromaSubsampling string
	// Quality is the encoding quality (1–100). 0 means use the package default (heifEncQuality).
	Quality int
}

// IsOversized reports whether pixelCount exceeds the oversized threshold.
func IsOversized(pixelCount int64) bool {
	return pixelCount > oversizedPixelThreshold
}

type encoder interface {
	Encode(srcPath, dstPath string, opts EncodeOptions) error
}

type metadataRestorer interface {
	CopyAll(srcPath, dstPath string) error
	RestoreKeyTimes(srcPath, dstPath string) error
}

// Converter converts supported non-HEIC source images into HEIC outputs.
type Converter struct {
	encoder          encoder
	metadataRestorer metadataRestorer
	stat             func(string) (os.FileInfo, error)
	chtimes          func(string, time.Time, time.Time) error
}

// New returns a converter using the heif-enc encoder from libheif-examples.
func New() *Converter {
	return &Converter{
		encoder:          newHeifEncEncoder(),
		metadataRestorer: exiftoolMetadataRestorer{},
		stat:             os.Stat,
		chtimes:          os.Chtimes,
	}
}

// Convert converts a supported non-HEIC image into a HEIC output file.
func Convert(srcPath, dstPath string) error {
	return New().Convert(srcPath, dstPath)
}

// Convert converts a supported non-HEIC image into a HEIC output file.
func (c *Converter) Convert(srcPath, dstPath string) error {
	srcInfo, err := c.stat(srcPath)
	if err != nil {
		return &Error{Kind: ErrorKindDecode, Path: srcPath, Err: err}
	}

	decoded, err := decodeSourceImage(srcPath)
	if err != nil {
		return &Error{Kind: ErrorKindDecode, Path: srcPath, Err: err}
	}

	return c.convertDecoded(srcPath, dstPath, srcInfo, decoded)
}

type decodedSource struct {
	format            string
	canonicalExt      string
	pixelCount        int64
	chromaSubsampling string
	quality           int // 0 means use package default
}

func (c *Converter) convertDecoded(srcPath, dstPath string, srcInfo os.FileInfo, decoded *decodedSource) error {
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return &Error{Kind: ErrorKindEncode, Path: dstPath, Err: err}
	}

	opts := EncodeOptions{
		Oversized:         IsOversized(decoded.pixelCount),
		ChromaSubsampling: decoded.chromaSubsampling,
		Quality:           decoded.quality,
	}
	if err := c.encoder.Encode(srcPath, dstPath, opts); err != nil {
		return &Error{Kind: ErrorKindEncode, Path: dstPath, Err: err}
	}

	if err := c.metadataRestorer.CopyAll(srcPath, dstPath); err != nil {
		return &Error{Kind: ErrorKindMetadata, Path: dstPath, Err: err}
	}
	if err := c.metadataRestorer.RestoreKeyTimes(srcPath, dstPath); err != nil {
		return &Error{Kind: ErrorKindMetadata, Path: dstPath, Err: err}
	}
	if err := c.chtimes(dstPath, srcInfo.ModTime(), srcInfo.ModTime()); err != nil {
		return &Error{Kind: ErrorKindMetadata, Path: dstPath, Err: err}
	}

	return nil
}

// decodeSourceImage opens srcPath, confirms it is not already HEIC/HEIF, and uses
// image.DecodeConfig to read the format and dimensions without loading the full pixel data.
func decodeSourceImage(srcPath string) (*decodedSource, error) {
	f, err := os.Open(srcPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	header := make([]byte, 64)
	n, err := f.Read(header)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	if isHEIFData(header[:n]) {
		return nil, ErrAlreadyHEIC
	}
	if _, err := f.Seek(0, 0); err != nil {
		return nil, err
	}

	// DecodeConfig reads only the image header — no full pixel decode — to get
	// the format string and dimensions cheaply.
	cfg, format, err := image.DecodeConfig(f)
	if err != nil {
		return nil, err
	}

	if format == "heic" || format == "heif" {
		return nil, ErrAlreadyHEIC
	}

	if cfg.Width > MaxDecodeDimension || cfg.Height > MaxDecodeDimension {
		return nil, fmt.Errorf("%w: %dx%d (max %d)", ErrDimensionTooLarge, cfg.Width, cfg.Height, MaxDecodeDimension)
	}

	normalizedFormat := normalizeSourceFormat(format, srcPath)
	return &decodedSource{
		format:            normalizedFormat,
		canonicalExt:      canonicalExtensionForFormat(normalizedFormat),
		pixelCount:        int64(cfg.Width) * int64(cfg.Height),
		chromaSubsampling: detectChromaSubsampling(),
	}, nil
}

func isHEIFData(header []byte) bool {
	if len(header) < 12 || string(header[4:8]) != "ftyp" {
		return false
	}

	brands := [][]byte{header[8:12]}
	for i := 16; i+4 <= len(header); i += 4 {
		brands = append(brands, header[i:i+4])
	}

	for _, brand := range brands {
		switch string(bytes.ToLower(brand)) {
		case "heic", "heix", "hevc", "hevx", "heim", "heis", "mif1", "msf1", "avic", "avis":
			return true
		}
	}
	return false
}

func canonicalExtensionForFormat(format string) string {
	switch strings.ToLower(format) {
	case "jpeg":
		return ".jpg"
	case "png":
		return ".png"
	case "gif":
		return ".gif"
	case "bmp":
		return ".bmp"
	case "tiff":
		return ".tiff"
	case "webp":
		return ".webp"
	default:
		return ""
	}
}

func normalizeSourceFormat(format, path string) string {
	switch strings.ToLower(format) {
	case "jpeg":
		return "jpeg"
	case "png":
		return "png"
	case "gif":
		return "gif"
	case "bmp":
		return "bmp"
	case "tiff":
		return "tiff"
	case "webp":
		return "webp"
	}

	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		return "jpeg"
	case ".png":
		return "png"
	case ".gif":
		return "gif"
	case ".bmp":
		return "bmp"
	case ".tif", ".tiff":
		return "tiff"
	case ".webp":
		return "webp"
	default:
		return format
	}
}

type exiftoolMetadataRestorer struct{}

func (exiftoolMetadataRestorer) CopyAll(srcPath, dstPath string) error {
	return runExiftool(
		"-m",
		"-overwrite_original",
		"-TagsFromFile", srcPath,
		"-EXIF:all",
		"-XMP:all",
		"-ICC_Profile:all",
		dstPath,
	)
}

func (exiftoolMetadataRestorer) RestoreKeyTimes(srcPath, dstPath string) error {
	return runExiftool(
		"-m",
		"-overwrite_original",
		"-TagsFromFile", srcPath,
		"-DateTimeOriginal<DateTimeOriginal",
		"-CreateDate<CreateDate",
		"-ModifyDate<ModifyDate",
		"-FileModifyDate<FileModifyDate",
		dstPath,
	)
}

func runExiftool(args ...string) error {
	cmd := exec.Command("exiftool", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("exiftool %v: %w: %s", args, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// EncodePNG is a small helper used by tests to create source fixtures.
func EncodePNG(dstPath string, img image.Image) error {
	f, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

// EncodeJPEG is a small helper used by tests to create source fixtures.
func EncodeJPEG(dstPath string, img image.Image, quality int) error {
	f, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer f.Close()
	return jpeg.Encode(f, img, &jpeg.Options{Quality: quality})
}
