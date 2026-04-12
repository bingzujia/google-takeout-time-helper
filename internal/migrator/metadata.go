package migrator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Metadata holds all information to write into a metadata JSON file.
type Metadata struct {
	OriginalPath   string     `json:"original_path"`
	OutputFilename string     `json:"output_filename"`
	SHA256         string     `json:"sha256"`
	Timestamp      TSInfo     `json:"timestamp"`
	GPS            *GPSInfo   `json:"gps,omitempty"`
	DeviceFolder   string     `json:"device_folder,omitempty"`
	DeviceType     string     `json:"device_type,omitempty"`
	ReviewReason   string     `json:"review_reason,omitempty"` // set when file needs manual attention
}

// TSInfo holds timestamp data from all sources.
type TSInfo struct {
	Final    string `json:"final"`
	Source   string `json:"source"`
	EXIF     string `json:"exif,omitempty"`
	Filename string `json:"filename,omitempty"`
	JSON     string `json:"json,omitempty"`
}

// GPSInfo holds GPS data from all sources.
type GPSInfo struct {
	Lat    float64   `json:"lat"`
	Lon    float64   `json:"lon"`
	Source string    `json:"source"`
	EXIF   *GPSPoint `json:"exif,omitempty"`
	JSON   *GPSPoint `json:"json,omitempty"`
}

// GPSPoint holds a single GPS coordinate.
type GPSPoint struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

// WriteMetadata writes a metadata JSON file to metadataDir/<sha256>.json.
func WriteMetadata(metadataDir string, m *Metadata) error {
	if err := os.MkdirAll(metadataDir, 0755); err != nil {
		return fmt.Errorf("create metadata dir: %w", err)
	}

	dstPath := filepath.Join(metadataDir, m.SHA256+".json")
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	if err := os.WriteFile(dstPath, data, 0644); err != nil {
		return fmt.Errorf("write metadata: %w", err)
	}

	return nil
}

// timeStr formats a time.Time as RFC3339, or returns empty string if zero.
func timeStr(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
