package parser

import (
	"errors"
	"path/filepath"
	"testing"
)

type fakeExifReader struct {
	results []fileMetadata
	closeFn func() error
}

func (f *fakeExifReader) ExtractMetadata(files ...string) []fileMetadata {
	return f.results
}

func (f *fakeExifReader) Close() error {
	if f.closeFn != nil {
		return f.closeFn()
	}
	return nil
}

func useFakeExifReader(t *testing.T, factory func() (exifReader, error)) {
	t.Helper()
	resetSharedExifReaderForTest()
	setSharedExifReaderFactoryForTest(factory)
	t.Cleanup(resetSharedExifReaderForTest)
}

func TestReadEXIFFields_UsesFileError(t *testing.T) {
	useFakeExifReader(t, func() (exifReader, error) {
		return &fakeExifReader{results: []fileMetadata{{Err: errors.New("bad file")}}}, nil
	})

	if _, err := readEXIFFields(filepath.Join(t.TempDir(), "bad.jpg")); err == nil {
		t.Fatal("expected readEXIFFields to return file error")
	}
}

func TestSharedExifReaderLifecycle(t *testing.T) {
	useFakeExifReader(t, func() (exifReader, error) {
		var closes int
		return &fakeExifReader{
			closeFn: func() error {
				closes++
				return nil
			},
		}, nil
	})

	if err := closeSharedExifReader(); err != nil {
		t.Fatalf("closeSharedExifReader returned error: %v", err)
	}
}
