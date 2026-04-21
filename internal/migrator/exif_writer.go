package migrator

import (
	"fmt"
	"os/exec"
	"time"
)

// ExifWriter writes metadata to files using direct exiftool CLI calls.
// Parser-side EXIF reads may use a shared wrapper, but writes stay on os/exec
// so the full exiftool write surface remains available.
type ExifWriter struct{}

// WriteTimestamp writes DateTimeOriginal and FileModifyDate to a file.
func (w *ExifWriter) WriteTimestamp(filePath string, t time.Time) error {
	exifTime := t.Format("2006:01:02 15:04:05")
	cmd := exec.Command("exiftool", "-ignoreMinorErrors", "-overwrite_original",
		"-DateTimeOriginal="+exifTime,
		"-FileModifyDate="+exifTime,
		filePath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("exiftool write timestamp: %w: %s", err, string(out))
	}
	return nil
}

// WriteGPS writes GPS coordinates to a file.
func (w *ExifWriter) WriteGPS(filePath string, lat, lon float64) error {
	cmd := exec.Command("exiftool", "-overwrite_original",
		fmt.Sprintf("-GPSLatitude=%f", lat),
		fmt.Sprintf("-GPSLongitude=%f", lon),
		filePath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("exiftool write GPS: %w: %s", err, string(out))
	}
	return nil
}

// WriteCreateAndModifyDate writes CreateDate and FileModifyDate to a file.
// It does NOT write DateTimeOriginal; use WriteTimestamp for that.
func (w *ExifWriter) WriteCreateAndModifyDate(filePath string, t time.Time) error {
	exifTime := t.Format("2006:01:02 15:04:05")
	cmd := exec.Command("exiftool", "-ignoreMinorErrors", "-overwrite_original",
		"-CreateDate="+exifTime,
		"-FileModifyDate="+exifTime,
		filePath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("exiftool write create/modify date: %w: %s", err, string(out))
	}
	return nil
}

// WriteSeparateTimestamps writes CreateDate and FileModifyDate from separate Unix timestamps.
// photoTakenTs: Unix timestamp for CreateDate field
// fileModifyTs: Unix timestamp for FileModifyDate field (if 0, CreateDate value is used as fallback)
// It does NOT write DateTimeOriginal; DateTimeOriginal is left untouched.
func (w *ExifWriter) WriteSeparateTimestamps(filePath string, photoTakenTs, fileModifyTs int64) error {
	// If both timestamps are 0, skip writing
	if photoTakenTs == 0 && fileModifyTs == 0 {
		return nil
	}

	args := []string{"-ignoreMinorErrors", "-overwrite_original"}

	// Write CreateDate from photoTakenTs
	if photoTakenTs != 0 {
		createDate := time.Unix(photoTakenTs, 0).Format("2006:01:02 15:04:05")
		args = append(args, "-CreateDate="+createDate)
	}

	// Write FileModifyDate from fileModifyTs, or fall back to photoTakenTs
	modifyTs := fileModifyTs
	if modifyTs == 0 {
		modifyTs = photoTakenTs
	}
	if modifyTs != 0 {
		fileModifyDate := time.Unix(modifyTs, 0).Format("2006:01:02 15:04:05")
		args = append(args, "-FileModifyDate="+fileModifyDate)
	}

	args = append(args, filePath)

	cmd := exec.Command("exiftool", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("exiftool write separate timestamps: %w: %s", err, string(out))
	}
	return nil
}

// WriteAll writes timestamp and optionally GPS to a file.
func (w *ExifWriter) WriteAll(filePath string, t time.Time, hasGPS bool, lat, lon float64) error {
	exifTime := t.Format("2006:01:02 15:04:05")
	args := []string{"-ignoreMinorErrors", "-overwrite_original",
		"-DateTimeOriginal=" + exifTime,
		"-FileModifyDate=" + exifTime,
	}
	if hasGPS {
		args = append(args,
			fmt.Sprintf("-GPSLatitude=%f", lat),
			fmt.Sprintf("-GPSLongitude=%f", lon),
		)
	}
	args = append(args, filePath)

	cmd := exec.Command("exiftool", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("exiftool write: %w: %s", err, string(out))
	}
	return nil
}
