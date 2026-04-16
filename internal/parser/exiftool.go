package parser

import (
	"time"
)

// ParseEXIFTimestamp extracts the DateTimeOriginal tag from a file using exiftool.
// Returns zero time if exiftool is not available, the file has no DateTimeOriginal,
// or the command fails.
func ParseEXIFTimestamp(filePath string) (time.Time, bool) {
	fields, err := readEXIFFields(filePath)
	if err != nil {
		return time.Time{}, false
	}

	rawTimestamp, ok := parseStringField(fields, "DateTimeOriginal")
	if !ok || rawTimestamp == "" {
		return time.Time{}, false
	}

	// exiftool outputs "YYYY:MM:DD HH:MM:SS" format
	t, err := time.Parse("2006:01:02 15:04:05", rawTimestamp)
	if err != nil {
		return time.Time{}, false
	}

	return t, true
}

// EXIFInfo holds combined timestamp and GPS data from a single exiftool call.
type EXIFInfo struct {
	Timestamp   time.Time
	TimestampOk bool
	Latitude    float64
	Longitude   float64
	GPSOk       bool
}

// ParseEXIFAll extracts both DateTimeOriginal and GPS coordinates in a single exiftool call.
// This is more efficient than calling ParseEXIFTimestamp and ParseEXIFGPS separately.
func ParseEXIFAll(filePath string) (*EXIFInfo, error) {
	fields, err := readEXIFFields(filePath)
	if err != nil {
		return nil, err
	}

	info := &EXIFInfo{}
	if rawTimestamp, ok := parseStringField(fields, "DateTimeOriginal"); ok && rawTimestamp != "" {
		t, err := time.Parse("2006:01:02 15:04:05", rawTimestamp)
		if err == nil {
			info.Timestamp = t
			info.TimestampOk = true
		}
	}

	lat, latOK := parseFloatField(fields, "GPSLatitude")
	lon, lonOK := parseFloatField(fields, "GPSLongitude")
	if latOK && lonOK && (lat != 0 || lon != 0) {
		info.Latitude = lat
		info.Longitude = lon
		info.GPSOk = true
	}

	return info, nil
}
