package parser

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
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

func TestParseEXIFTimestamp(t *testing.T) {
	useFakeExifReader(t, func() (exifReader, error) {
		return &fakeExifReader{
			results: []fileMetadata{{
				Fields: map[string]interface{}{
					"DateTimeOriginal": "2024:04:15 12:34:56",
				},
			}},
		}, nil
	})

	got, ok := ParseEXIFTimestamp("test.jpg")
	if !ok {
		t.Fatal("expected ParseEXIFTimestamp to succeed")
	}

	want := time.Date(2024, 4, 15, 12, 34, 56, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("timestamp = %v, want %v", got, want)
	}
}

func TestParseEXIFTimestamp_GracefulFailures(t *testing.T) {
	t.Run("missing tag", func(t *testing.T) {
		useFakeExifReader(t, func() (exifReader, error) {
			return &fakeExifReader{results: []fileMetadata{{Fields: map[string]interface{}{}}}}, nil
		})

		if _, ok := ParseEXIFTimestamp("missing.jpg"); ok {
			t.Fatal("expected false for missing timestamp")
		}
	})

	t.Run("malformed tag", func(t *testing.T) {
		useFakeExifReader(t, func() (exifReader, error) {
			return &fakeExifReader{
				results: []fileMetadata{{Fields: map[string]interface{}{"DateTimeOriginal": "bad"}}},
			}, nil
		})

		if _, ok := ParseEXIFTimestamp("bad.jpg"); ok {
			t.Fatal("expected false for malformed timestamp")
		}
	})

	t.Run("reader init error", func(t *testing.T) {
		useFakeExifReader(t, func() (exifReader, error) {
			return nil, errors.New("boom")
		})

		if _, ok := ParseEXIFTimestamp("err.jpg"); ok {
			t.Fatal("expected false on reader init error")
		}
	})
}

func TestParseEXIFAll(t *testing.T) {
	useFakeExifReader(t, func() (exifReader, error) {
		return &fakeExifReader{
			results: []fileMetadata{{
				Fields: map[string]interface{}{
					"DateTimeOriginal": "2024:04:15 12:34:56",
					"GPSLatitude":      10.5,
					"GPSLongitude":     20.5,
				},
			}},
		}, nil
	})

	info, err := ParseEXIFAll("combined.jpg")
	if err != nil {
		t.Fatalf("ParseEXIFAll returned error: %v", err)
	}
	if !info.TimestampOk || !info.GPSOk {
		t.Fatalf("expected timestamp and GPS to be present: %#v", info)
	}
	if info.Latitude != 10.5 || info.Longitude != 20.5 {
		t.Fatalf("unexpected GPS: %#v", info)
	}
}

func TestParseEXIFAll_SubsetsAndErrors(t *testing.T) {
	t.Run("timestamp only", func(t *testing.T) {
		useFakeExifReader(t, func() (exifReader, error) {
			return &fakeExifReader{
				results: []fileMetadata{{
					Fields: map[string]interface{}{"DateTimeOriginal": "2024:04:15 12:34:56"},
				}},
			}, nil
		})

		info, err := ParseEXIFAll("timestamp.jpg")
		if err != nil {
			t.Fatalf("ParseEXIFAll returned error: %v", err)
		}
		if !info.TimestampOk || info.GPSOk {
			t.Fatalf("unexpected subset result: %#v", info)
		}
	})

	t.Run("reader file error", func(t *testing.T) {
		useFakeExifReader(t, func() (exifReader, error) {
			return &fakeExifReader{
				results: []fileMetadata{{Err: errors.New("read failed")}},
			}, nil
		})

		if _, err := ParseEXIFAll("fail.jpg"); err == nil {
			t.Fatal("expected ParseEXIFAll to return error")
		}
	})
}

func TestParseEXIFTimestamp_RealImage(t *testing.T) {
	resetSharedExifReaderForTest()
	t.Cleanup(resetSharedExifReaderForTest)

	testFiles := []string{
		"testdata/test.jpg",
		"testdata/test.jpeg",
	}

	for _, f := range testFiles {
		if _, err := os.Stat(f); err != nil {
			continue
		}

		ts, ok := ParseEXIFTimestamp(f)
		if !ok {
			t.Logf("no EXIF DateTimeOriginal in %s (may be expected)", f)
			return
		}

		if ts.Year() < 1970 || ts.Year() > 2100 {
			t.Errorf("EXIF timestamp %v out of reasonable range", ts)
		}
		return
	}

	t.Log("no testdata images found, skipping EXIF real-image test")
}

func TestSharedExifReaderLifecycle(t *testing.T) {
	var creates int
	var closes int
	useFakeExifReader(t, func() (exifReader, error) {
		creates++
		return &fakeExifReader{
			results: []fileMetadata{{Fields: map[string]interface{}{"DateTimeOriginal": "2024:04:15 12:34:56"}}},
			closeFn: func() error {
				closes++
				return nil
			},
		}, nil
	})

	if _, ok := ParseEXIFTimestamp("one.jpg"); !ok {
		t.Fatal("expected first parse to succeed")
	}
	if _, ok := ParseEXIFTimestamp("two.jpg"); !ok {
		t.Fatal("expected second parse to succeed")
	}
	if creates != 1 {
		t.Fatalf("reader created %d times, want 1", creates)
	}

	if err := closeSharedExifReader(); err != nil {
		t.Fatalf("closeSharedExifReader returned error: %v", err)
	}
	if closes != 1 {
		t.Fatalf("reader closed %d times, want 1", closes)
	}
}

func TestParseEXIFGPS_FakeMetadata(t *testing.T) {
	t.Run("numeric GPS", func(t *testing.T) {
		useFakeExifReader(t, func() (exifReader, error) {
			return &fakeExifReader{
				results: []fileMetadata{{
					Fields: map[string]interface{}{
						"GPSLatitude":  37.7749,
						"GPSLongitude": -122.4194,
						"GPSAltitude":  12.0,
					},
				}},
			}, nil
		})

		info := ParseEXIFGPS("gps.jpg")
		if !info.Has || info.Lat != 37.7749 || info.Lon != -122.4194 || info.Alt != 12.0 {
			t.Fatalf("unexpected GPS info: %#v", info)
		}
	})

	t.Run("GPSCoordinates fallback", func(t *testing.T) {
		useFakeExifReader(t, func() (exifReader, error) {
			return &fakeExifReader{
				results: []fileMetadata{{
					Fields: map[string]interface{}{
						"GPSCoordinates": "37.7749, -122.4194, 50.0",
					},
				}},
			}, nil
		})

		info := ParseEXIFGPS("coords.jpg")
		if !info.Has || info.Lat != 37.7749 || info.Lon != -122.4194 || info.Alt != 50.0 {
			t.Fatalf("unexpected GPSCoordinates fallback result: %#v", info)
		}
	})

	t.Run("altitude only", func(t *testing.T) {
		useFakeExifReader(t, func() (exifReader, error) {
			return &fakeExifReader{
				results: []fileMetadata{{
					Fields: map[string]interface{}{
						"GPSAltitude": 99.0,
					},
				}},
			}, nil
		})

		info := ParseEXIFGPS("alt.jpg")
		if !info.Has || info.Alt != 99.0 {
			t.Fatalf("unexpected altitude-only result: %#v", info)
		}
	})
}

func TestParseEXIFGPS_NoGPS(t *testing.T) {
	useFakeExifReader(t, func() (exifReader, error) {
		return &fakeExifReader{results: []fileMetadata{{Fields: map[string]interface{}{}}}}, nil
	})

	info := ParseEXIFGPS("/dev/null")
	if info.Has {
		t.Error("expected Has=false for /dev/null")
	}
}

func TestParseEXIFGPS_RealImage(t *testing.T) {
	resetSharedExifReaderForTest()
	t.Cleanup(resetSharedExifReaderForTest)

	testFiles := []string{
		"testdata/test.jpg",
		"testdata/test.jpeg",
	}

	for _, f := range testFiles {
		info := ParseEXIFGPS(f)
		if !info.Has {
			t.Logf("no GPS data in %s (may be expected)", f)
			return
		}

		if info.Lat < -90 || info.Lat > 90 {
			t.Errorf("latitude %v out of range [-90, 90]", info.Lat)
		}
		if info.Lon < -180 || info.Lon > 180 {
			t.Errorf("longitude %v out of range [-180, 180]", info.Lon)
		}
		return
	}

	t.Log("no testdata images found, skipping EXIF GPS real-image test")
}

func TestReadEXIFFields_UsesFileError(t *testing.T) {
	useFakeExifReader(t, func() (exifReader, error) {
		return &fakeExifReader{results: []fileMetadata{{Err: errors.New("bad file")}}}, nil
	})

	if _, err := readEXIFFields(filepath.Join(t.TempDir(), "bad.jpg")); err == nil {
		t.Fatal("expected readEXIFFields to return file error")
	}
}
