package metadata

import (
	"fmt"
	"os"
	"os/exec"
	"time"
)

// MetadataWriter writes metadata to media files.
type MetadataWriter interface {
	WriteTimestamp(filePath string, t time.Time) error
	WriteGPS(filePath string, lat, lon, alt float64) error
}

// NewWriter auto-detects exiftool and returns ExifToolWriter if found, else NativeWriter.
func NewWriter() MetadataWriter {
	if path, err := exec.LookPath("exiftool"); err == nil {
		return &ExifToolWriter{exiftoolPath: path}
	}
	return &NativeWriter{}
}

// ExifToolWriter uses exiftool to write metadata.
type ExifToolWriter struct {
	exiftoolPath string
}

func (w *ExifToolWriter) WriteTimestamp(filePath string, t time.Time) error {
	formatted := t.Format("2006:01:02 15:04:05")
	cmd := exec.Command(w.exiftoolPath,
		"-overwrite_original",
		fmt.Sprintf("-DateTimeOriginal=%s", formatted),
		fmt.Sprintf("-CreateDate=%s", formatted),
		fmt.Sprintf("-ModifyDate=%s", formatted),
		filePath,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("exiftool WriteTimestamp %q: %w\n%s", filePath, err, out)
	}
	return nil
}

func (w *ExifToolWriter) WriteGPS(filePath string, lat, lon, alt float64) error {
	latRef := "N"
	if lat < 0 {
		latRef = "S"
		lat = -lat
	}
	lonRef := "E"
	if lon < 0 {
		lonRef = "W"
		lon = -lon
	}
	altRef := "0"
	if alt < 0 {
		altRef = "1"
		alt = -alt
	}

	cmd := exec.Command(w.exiftoolPath,
		"-overwrite_original",
		fmt.Sprintf("-GPSLatitude=%f", lat),
		fmt.Sprintf("-GPSLatitudeRef=%s", latRef),
		fmt.Sprintf("-GPSLongitude=%f", lon),
		fmt.Sprintf("-GPSLongitudeRef=%s", lonRef),
		fmt.Sprintf("-GPSAltitude=%f", alt),
		fmt.Sprintf("-GPSAltitudeRef=%s", altRef),
		filePath,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("exiftool WriteGPS %q: %w\n%s", filePath, err, out)
	}
	return nil
}

// NativeWriter uses os.Chtimes for timestamps; GPS is not supported.
type NativeWriter struct{}

func (w *NativeWriter) WriteTimestamp(filePath string, t time.Time) error {
	return os.Chtimes(filePath, t, t)
}

func (w *NativeWriter) WriteGPS(_ string, _, _, _ float64) error {
	// Not supported without exiftool
	return nil
}
