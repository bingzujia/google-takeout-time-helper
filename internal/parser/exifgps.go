package parser

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"strconv"
	"strings"
)

// exifGPSOutput mirrors the JSON output from exiftool -j for GPS tags.
type exifGPSOutput struct {
	GPSLatitude  *float64 `json:"GPSLatitude,omitempty"`
	GPSLongitude *float64 `json:"GPSLongitude,omitempty"`
	GPSAltitude  *float64 `json:"GPSAltitude,omitempty"`
	// GPSCoordinates is a fallback tag some cameras use (e.g. "37.7749, -122.4194, 0")
	GPSCoordinates string `json:"GPSCoordinates,omitempty"`
}

// GPSInfo holds parsed GPS data from a photo file.
type GPSInfo struct {
	Lat  float64
	Lon  float64
	Alt  float64
	Has  bool // true if any GPS coordinate was found
}

// ParseEXIFGPS extracts GPS coordinates from a file using exiftool.
// Returns GPSInfo with Has=false if exiftool is not available, the file has
// no GPS data, or the command fails.
func ParseEXIFGPS(filePath string) GPSInfo {
	cmd := exec.Command("exiftool", "-n", "-j", "-GPSLatitude", "-GPSLongitude", "-GPSAltitude", "-GPSCoordinates", filePath)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return GPSInfo{}
	}

	var results []exifGPSOutput
	if err := json.Unmarshal(stdout.Bytes(), &results); err != nil {
		return GPSInfo{}
	}

	if len(results) == 0 {
		return GPSInfo{}
	}

	r := results[0]
	info := GPSInfo{}

	if r.GPSLatitude != nil && r.GPSLongitude != nil {
		info.Lat = *r.GPSLatitude
		info.Lon = *r.GPSLongitude
		// Filter out (0,0) — common placeholder for missing GPS
		if info.Lat != 0 || info.Lon != 0 {
			info.Has = true
		}
	} else if r.GPSCoordinates != "" {
		// Fallback: parse "lat, lon, alt" string
		if lat, lon, alt, ok := parseGPSCoords(r.GPSCoordinates); ok {
			info.Lat = lat
			info.Lon = lon
			info.Alt = alt
			// Filter out (0,0) — common placeholder for missing GPS
			if lat != 0 || lon != 0 {
				info.Has = true
			}
		}
	}

	if r.GPSAltitude != nil {
		info.Alt = *r.GPSAltitude
		if !info.Has {
			info.Has = true
		}
	}

	return info
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
