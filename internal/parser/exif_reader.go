package parser

import (
	"fmt"
	"strconv"
	"sync"

	goexiftool "github.com/barasher/go-exiftool"
)

type fileMetadata struct {
	Fields map[string]interface{}
	Err    error
}

type exifReader interface {
	ExtractMetadata(files ...string) []fileMetadata
	Close() error
}

type goExiftoolReader struct {
	inner *goexiftool.Exiftool
}

func newGoExiftoolReader() (exifReader, error) {
	et, err := goexiftool.NewExiftool(goexiftool.NoPrintConversion())
	if err != nil {
		return nil, err
	}
	return &goExiftoolReader{inner: et}, nil
}

func (r *goExiftoolReader) ExtractMetadata(files ...string) []fileMetadata {
	items := r.inner.ExtractMetadata(files...)
	results := make([]fileMetadata, len(items))
	for i, item := range items {
		results[i] = fileMetadata{
			Fields: item.Fields,
			Err:    item.Err,
		}
	}
	return results
}

func (r *goExiftoolReader) Close() error {
	return r.inner.Close()
}

var (
	exifReaderMu      sync.Mutex
	sharedExifReader  exifReader
	newSharedReaderFn = newGoExiftoolReader
)

func getSharedExifReader() (exifReader, error) {
	exifReaderMu.Lock()
	defer exifReaderMu.Unlock()

	if sharedExifReader != nil {
		return sharedExifReader, nil
	}

	reader, err := newSharedReaderFn()
	if err != nil {
		return nil, err
	}
	sharedExifReader = reader
	return sharedExifReader, nil
}

func closeSharedExifReader() error {
	exifReaderMu.Lock()
	defer exifReaderMu.Unlock()

	if sharedExifReader == nil {
		return nil
	}

	err := sharedExifReader.Close()
	sharedExifReader = nil
	return err
}

func readEXIFFields(filePath string) (map[string]interface{}, error) {
	reader, err := getSharedExifReader()
	if err != nil {
		return nil, err
	}

	metadata := reader.ExtractMetadata(filePath)
	if len(metadata) == 0 {
		return nil, fmt.Errorf("no exif metadata returned for %s", filePath)
	}
	if metadata[0].Err != nil {
		return nil, metadata[0].Err
	}
	return metadata[0].Fields, nil
}

func parseFloatField(fields map[string]interface{}, key string) (float64, bool) {
	value, ok := fields[key]
	if !ok || value == nil {
		return 0, false
	}

	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case string:
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}

func parseStringField(fields map[string]interface{}, key string) (string, bool) {
	value, ok := fields[key]
	if !ok || value == nil {
		return "", false
	}

	switch v := value.(type) {
	case string:
		return v, true
	default:
		return fmt.Sprint(v), true
	}
}
