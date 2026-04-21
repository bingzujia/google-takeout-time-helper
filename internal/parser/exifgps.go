package parser

import (
	"strconv"
	"strings"
)

// GPSInfo holds parsed GPS data from a photo file.
type GPSInfo struct {
	Lat float64
	Lon float64
	Alt float64
	Has bool // true if any GPS coordinate was found
}

// resetSharedExifReaderForTest resets the shared exif reader (test helper).
func resetSharedExifReaderForTest() {
	exifReaderMu.Lock()
	defer exifReaderMu.Unlock()

	if sharedExifReader != nil {
		_ = sharedExifReader.Close()
		sharedExifReader = nil
	}
	newSharedReaderFn = newGoExiftoolReader
}

func setSharedExifReaderFactoryForTest(factory func() (exifReader, error)) {
	exifReaderMu.Lock()
	defer exifReaderMu.Unlock()

	if sharedExifReader != nil {
		_ = sharedExifReader.Close()
		sharedExifReader = nil
	}
	newSharedReaderFn = factory
}

// parseGPSCoords parses a "lat, lon, alt" or "lat, lon" coordinate string.
func parseGPSCoords(s string) (lat, lon, alt float64, ok bool) {
	parts := strings.Split(s, ",")
	if len(parts) < 2 {
		return 0, 0, 0, false
	}

	var err error
	lat, err = strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	if err != nil {
		return 0, 0, 0, false
	}
	lon, err = strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err != nil {
		return 0, 0, 0, false
	}
	if len(parts) >= 3 {
		alt, _ = strconv.ParseFloat(strings.TrimSpace(parts[2]), 64)
	}
	return lat, lon, alt, true
}
