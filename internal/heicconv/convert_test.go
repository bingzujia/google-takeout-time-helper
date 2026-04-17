package heicconv

import (
	"errors"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type fakeEncoder struct {
	err      error
	lastSrc  string
	lastDst  string
	lastOpts EncodeOptions
}

func (f *fakeEncoder) Encode(srcPath, dstPath string, opts EncodeOptions) error {
	f.lastSrc = srcPath
	f.lastDst = dstPath
	f.lastOpts = opts
	if f.err != nil {
		return f.err
	}
	return os.WriteFile(dstPath, []byte("fake-heic"), 0o644)
}

type fakeMetadataRestorer struct {
	copyAllCalled      bool
	restoreTimesCalled bool
	copyAllErr         error
	restoreTimesErr    error
	copySrc            string
	copyDst            string
	restoreSrc         string
	restoreDst         string
}

func (f *fakeMetadataRestorer) CopyAll(src, dst string) error {
	f.copyAllCalled = true
	f.copySrc = src
	f.copyDst = dst
	return f.copyAllErr
}

func (f *fakeMetadataRestorer) RestoreKeyTimes(src, dst string) error {
	f.restoreTimesCalled = true
	f.restoreSrc = src
	f.restoreDst = dst
	return f.restoreTimesErr
}

func TestConvertSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "src.png")
	dstPath := filepath.Join(tmpDir, "nested", "out.heic")

	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err := EncodePNG(srcPath, img); err != nil {
		t.Fatalf("EncodePNG: %v", err)
	}

	modTime := time.Date(2024, 4, 15, 10, 30, 0, 0, time.UTC)
	if err := os.Chtimes(srcPath, modTime, modTime); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	metadata := &fakeMetadataRestorer{}
	converter := &Converter{
		encoder:          &fakeEncoder{},
		metadataRestorer: metadata,
		stat:             os.Stat,
		chtimes:          os.Chtimes,
	}

	if err := converter.Convert(srcPath, dstPath); err != nil {
		t.Fatalf("Convert returned error: %v", err)
	}

	if !metadata.copyAllCalled || !metadata.restoreTimesCalled {
		t.Fatalf("metadata restorer was not fully called: %+v", metadata)
	}

	info, err := os.Stat(dstPath)
	if err != nil {
		t.Fatalf("Stat(dst): %v", err)
	}
	if !info.ModTime().Equal(modTime) {
		t.Fatalf("dst mtime = %v, want %v", info.ModTime(), modTime)
	}
}

func TestConvertDecodeFailure(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "bad.txt")
	if err := os.WriteFile(srcPath, []byte("not an image"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	err := (&Converter{
		encoder:          &fakeEncoder{},
		metadataRestorer: &fakeMetadataRestorer{},
		stat:             os.Stat,
		chtimes:          os.Chtimes,
	}).Convert(srcPath, filepath.Join(tmpDir, "out.heic"))
	if err == nil {
		t.Fatal("expected decode error")
	}

	var convErr *Error
	if !errors.As(err, &convErr) || convErr.Kind != ErrorKindDecode {
		t.Fatalf("error = %#v, want decode Error", err)
	}
}

func TestConvertRejectsActualHEICInput(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "already.jpg")
	if err := os.WriteFile(srcPath, fakeHEIFHeader(), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	err := (&Converter{
		encoder:          &fakeEncoder{},
		metadataRestorer: &fakeMetadataRestorer{},
		stat:             os.Stat,
		chtimes:          os.Chtimes,
	}).Convert(srcPath, filepath.Join(tmpDir, "out.heic"))
	if err == nil {
		t.Fatal("expected decode error")
	}

	var convErr *Error
	if !errors.As(err, &convErr) || convErr.Kind != ErrorKindDecode {
		t.Fatalf("error = %#v, want decode Error", err)
	}
	if !errors.Is(err, ErrAlreadyHEIC) {
		t.Fatalf("error = %v, want ErrAlreadyHEIC", err)
	}
}

func TestConvertEncodeFailure(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "src.png")
	dstPath := filepath.Join(tmpDir, "out.heic")

	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	if err := EncodePNG(srcPath, img); err != nil {
		t.Fatalf("EncodePNG: %v", err)
	}

	err := (&Converter{
		encoder:          &fakeEncoder{err: errors.New("encode failed")},
		metadataRestorer: &fakeMetadataRestorer{},
		stat:             os.Stat,
		chtimes:          os.Chtimes,
	}).Convert(srcPath, dstPath)
	if err == nil {
		t.Fatal("expected encode error")
	}

	var convErr *Error
	if !errors.As(err, &convErr) || convErr.Kind != ErrorKindEncode {
		t.Fatalf("error = %#v, want encode Error", err)
	}
}

func TestDecodeSourceImageUsesActualContentForCanonicalExtension(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "wrong.png")

	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{G: 255, A: 255})
	if err := EncodeJPEG(srcPath, img, 90); err != nil {
		t.Fatalf("EncodeJPEG: %v", err)
	}

	decoded, err := decodeSourceImage(srcPath, nil)
	if err != nil {
		t.Fatalf("decodeSourceImage returned error: %v", err)
	}
	if decoded.format != "jpeg" {
		t.Fatalf("decoded.format = %q, want jpeg", decoded.format)
	}
	if decoded.canonicalExt != ".jpg" {
		t.Fatalf("decoded.canonicalExt = %q, want .jpg", decoded.canonicalExt)
	}
}

func fakeHEIFHeader() []byte {
	return []byte{
		0x00, 0x00, 0x00, 0x18,
		'f', 't', 'y', 'p',
		'h', 'e', 'i', 'c',
		0x00, 0x00, 0x00, 0x00,
		'm', 'i', 'f', '1',
		'h', 'e', 'i', 'c',
	}
}

func TestConvertMetadataFailures(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "src.png")
	dstPath := filepath.Join(tmpDir, "out.heic")

	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	if err := EncodePNG(srcPath, img); err != nil {
		t.Fatalf("EncodePNG: %v", err)
	}

	tests := []struct {
		name     string
		metadata *fakeMetadataRestorer
	}{
		{
			name:     "copy all fails",
			metadata: &fakeMetadataRestorer{copyAllErr: errors.New("copy failed")},
		},
		{
			name:     "restore key times fails",
			metadata: &fakeMetadataRestorer{restoreTimesErr: errors.New("restore failed")},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := (&Converter{
				encoder:          &fakeEncoder{},
				metadataRestorer: tc.metadata,
				stat:             os.Stat,
				chtimes:          os.Chtimes,
			}).Convert(srcPath, dstPath)
			if err == nil {
				t.Fatal("expected metadata error")
			}

			var convErr *Error
			if !errors.As(err, &convErr) || convErr.Kind != ErrorKindMetadata {
				t.Fatalf("error = %#v, want metadata Error", err)
			}
		})
	}
}

func TestIsOversized(t *testing.T) {
	tests := []struct {
		pixels int64
		want   bool
	}{
		{39_999_999, false},
		{40_000_000, false}, // exactly at threshold is not oversized
		{40_000_001, true},
		{100_000_000, true},
	}
	for _, tc := range tests {
		got := IsOversized(tc.pixels)
		if got != tc.want {
			t.Errorf("IsOversized(%d) = %v, want %v", tc.pixels, got, tc.want)
		}
	}
}

func TestDecodeSourceImageRecordsPixelCount(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "src.png")

	img := image.NewRGBA(image.Rect(0, 0, 100, 200))
	if err := EncodePNG(srcPath, img); err != nil {
		t.Fatalf("EncodePNG: %v", err)
	}

	decoded, err := decodeSourceImage(srcPath, nil)
	if err != nil {
		t.Fatalf("decodeSourceImage: %v", err)
	}
	if decoded.pixelCount != 100*200 {
		t.Fatalf("pixelCount = %d, want %d", decoded.pixelCount, 100*200)
	}
}

func TestConvertPassesOversizedOptionToEncoder(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a tiny image; pixel count = 1 < 40M → Oversized should be false.
	srcPath := filepath.Join(tmpDir, "tiny.png")
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	if err := EncodePNG(srcPath, img); err != nil {
		t.Fatalf("EncodePNG: %v", err)
	}

	enc := &fakeEncoder{}
	converter := &Converter{
		encoder:          enc,
		metadataRestorer: &fakeMetadataRestorer{},
		stat:             os.Stat,
		chtimes:          os.Chtimes,
	}

	if err := converter.Convert(srcPath, filepath.Join(tmpDir, "out.heic")); err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if enc.lastOpts.Oversized {
		t.Fatal("Oversized = true for a 1×1 image, want false")
	}
	if enc.lastSrc != srcPath {
		t.Fatalf("encoder srcPath = %q, want %q", enc.lastSrc, srcPath)
	}
}
