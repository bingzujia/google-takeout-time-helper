package parser

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"time"
)

// exifOutput mirrors the JSON output from exiftool -j.
type exifOutput struct {
	DateTimeOriginal string  `json:"DateTimeOriginal"`
	GPSLatitude      float64 `json:"GPSLatitude"`
	GPSLongitude     float64 `json:"GPSLongitude"`
}

// ParseEXIFTimestamp extracts the DateTimeOriginal tag from a file using exiftool.
// Returns zero time if exiftool is not available, the file has no DateTimeOriginal,
// or the command fails.
func ParseEXIFTimestamp(filePath string) (time.Time, bool) {
	cmd := exec.Command("exiftool", "-j", "-DateTimeOriginal", filePath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return time.Time{}, false
	}

	var results []exifOutput
	if err := json.Unmarshal(stdout.Bytes(), &results); err != nil {
		return time.Time{}, false
	}

	if len(results) == 0 || results[0].DateTimeOriginal == "" {
		return time.Time{}, false
	}

	// exiftool outputs "YYYY:MM:DD HH:MM:SS" format
	t, err := time.Parse("2006:01:02 15:04:05", results[0].DateTimeOriginal)
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
	cmd := exec.Command("exiftool", "-j", "-n", "-DateTimeOriginal", "-GPSLatitude", "-GPSLongitude", filePath)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	var results []exifOutput
	if err := json.Unmarshal(stdout.Bytes(), &results); err != nil {
		return nil, err
	}

	info := &EXIFInfo{}

	if len(results) > 0 {
		r := results[0]
		if r.DateTimeOriginal != "" {
			t, err := time.Parse("2006:01:02 15:04:05", r.DateTimeOriginal)
			if err == nil {
				info.Timestamp = t
				info.TimestampOk = true
			}
		}
		// GPS is valid if at least one of lat/lon is non-zero
		if r.GPSLatitude != 0 || r.GPSLongitude != 0 {
			info.Latitude = r.GPSLatitude
			info.Longitude = r.GPSLongitude
			info.GPSOk = true
		}
	}

	return info, nil
}
