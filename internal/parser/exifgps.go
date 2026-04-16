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

// ParseEXIFGPS extracts GPS coordinates from a file using exiftool.
// Returns GPSInfo with Has=false if exiftool is not available, the file has
// no GPS data, or the command fails.
func ParseEXIFGPS(filePath string) GPSInfo {
	fields, err := readEXIFFields(filePath)
	if err != nil {
		return GPSInfo{}
	}
	info := GPSInfo{}

	if lat, latOK := parseFloatField(fields, "GPSLatitude"); latOK {
		if lon, lonOK := parseFloatField(fields, "GPSLongitude"); lonOK {
			info.Lat = lat
			info.Lon = lon
			// Filter out (0,0) — common placeholder for missing GPS
			if info.Lat != 0 || info.Lon != 0 {
				info.Has = true
			}
		}
	} else if coords, ok := parseStringField(fields, "GPSCoordinates"); ok && coords != "" {
		// Fallback: parse "lat, lon, alt" string
		if lat, lon, alt, ok := parseGPSCoords(coords); ok {
			info.Lat = lat
			info.Lon = lon
			info.Alt = alt
			// Filter out (0,0) — common placeholder for missing GPS
			if lat != 0 || lon != 0 {
				info.Has = true
			}
		}
	}

	if alt, ok := parseFloatField(fields, "GPSAltitude"); ok {
		info.Alt = alt
		if !info.Has {
			info.Has = true
		}
	}

	return info
}

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
