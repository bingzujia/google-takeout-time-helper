package migrator

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bingzujia/g_photo_take_out_helper/internal/matcher"
	"github.com/bingzujia/g_photo_take_out_helper/internal/organizer"
	"github.com/bingzujia/g_photo_take_out_helper/internal/parser"
	"github.com/bingzujia/g_photo_take_out_helper/internal/progress"
)

// supported media extensions
var mediaExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
	".bmp": true, ".tiff": true, ".tif": true, ".webp": true,
	".heic": true, ".heif": true,
	".mp4": true, ".mov": true, ".avi": true, ".mkv": true,
	".wmv": true, ".flv": true, ".3gp": true, ".m4v": true,
}

// Stats holds processing statistics.
type Stats struct {
	Scanned       int
	Processed     int
	SkippedNoTime int
	SkippedExists int
	FailedExif    int
	FailedOther   int
}

// Config holds migration settings.
type Config struct {
	InputDir     string
	OutputDir    string
	ShowProgress bool // whether to display progress bar
}

// FileEntry holds pre-scanned file information.
type FileEntry struct {
	Path    string // absolute path
	RelPath string // relative path (for logging)
}

// Run executes the full migration pipeline.
func Run(cfg Config) (*Stats, error) {
	// Step 1: Check output directory
	if err := checkOutputDir(cfg.OutputDir); err != nil {
		return nil, err
	}

	// Create output directories
	metadataDir := filepath.Join(cfg.OutputDir, "metadata")
	if err := os.MkdirAll(metadataDir, 0755); err != nil {
		return nil, fmt.Errorf("create metadata dir: %w", err)
	}

	// Step 2: Initialize logger
	logPath := filepath.Join(cfg.OutputDir, "gtoh.log")
	logger, err := NewLogger(logPath)
	if err != nil {
		return nil, fmt.Errorf("create logger: %w", err)
	}
	defer logger.Close()

	// Step 3: Classify folders
	yearFolders, _, err := organizer.ClassifyFolder(cfg.InputDir)
	if err != nil {
		return nil, fmt.Errorf("classify folders: %w", err)
	}

	if len(yearFolders) == 0 {
		fmt.Println("No year folders (Photos from XXXX) found.")
		return &Stats{}, nil
	}

	// Phase 1: Scan all media files
	fmt.Println("Scanning files...")
	entries := scanFiles(yearFolders, cfg.InputDir)
	if len(entries) == 0 {
		fmt.Println("No media files found in year folders.")
		return &Stats{}, nil
	}
	fmt.Printf("Found %d files in %d year folder(s)\n\n", len(entries), len(yearFolders))

	// Phase 2: Process files with progress bar
	exifWriter := &ExifWriter{}
	stats := &Stats{}
	processFiles(entries, cfg.OutputDir, metadataDir, logger, exifWriter, stats, cfg.ShowProgress)

	return stats, nil
}

// scanFiles collects all media files from the given year folders.
func scanFiles(yearFolders []string, inputDir string) []FileEntry {
	var entries []FileEntry
	for _, yf := range yearFolders {
		filepath.Walk(yf, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if !mediaExts[strings.ToLower(filepath.Ext(path))] {
				return nil
			}
			relPath, _ := filepath.Rel(inputDir, path)
			entries = append(entries, FileEntry{
				Path:    path,
				RelPath: relPath,
			})
			return nil
		})
	}
	return entries
}

// processFiles iterates over all entries and processes each one concurrently.
func processFiles(entries []FileEntry, outputDir, metadataDir string,
	logger *Logger, exifWriter *ExifWriter, stats *Stats, showProgress bool) {

	// Determine worker count
	workers := runtime.NumCPU()
	if workers > 8 {
		workers = 8
	}

	var wg sync.WaitGroup
	var mu sync.Mutex // protects logger and stats
	var processed atomic.Int64
	total := len(entries)

	jobCh := make(chan FileEntry, workers)

	// progCh serializes all progress updates through a single goroutine so that
	// concurrent workers never interleave their \r-based progress output.
	var progWg sync.WaitGroup
	progCh := make(chan int, workers)
	if showProgress && total > 0 {
		progWg.Add(1)
		go func() {
			defer progWg.Done()
			for cur := range progCh {
				progress.PrintProgress(cur, total)
			}
			fmt.Println()
		}()
	}

	// Start workers
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for entry := range jobCh {
				processSingleFile(entry, outputDir, metadataDir, logger, exifWriter, stats, &mu)
				cur := int(processed.Add(1))
				if showProgress && shouldUpdate(cur, total) {
					progCh <- cur
				}
			}
		}()
	}

	// Dispatch jobs
	for _, entry := range entries {
		jobCh <- entry
	}
	close(jobCh)

	// Wait for workers then signal progress goroutine to exit.
	wg.Wait()
	close(progCh)
	progWg.Wait()
}

// shouldUpdate determines whether to refresh the progress bar.
func shouldUpdate(current, total int) bool {
	if total < 1000 {
		return true
	}
	return current%10 == 0 || current == total
}

// processSingleFile handles one media file through the full pipeline.
func processSingleFile(entry FileEntry, outputDir, metadataDir string,
	logger *Logger, exifWriter *ExifWriter, stats *Stats, mu *sync.Mutex) {

	mu.Lock()
	stats.Scanned++
	mu.Unlock()

	// Step 3a: Match JSON sidecar
	jsonResult := matcher.JSONForFile(entry.Path)
	var deviceFolder, deviceType string
	if jsonResult == nil {
		mu.Lock()
		logger.Info("no_json_sidecar", entry.RelPath)
		mu.Unlock()
	} else {
		deviceFolder = jsonResult.DeviceFolder
		deviceType = jsonResult.DeviceType
	}

	// Step 3b: Extract timestamps — filename first (zero cost), exiftool only if needed
	filenameTimestamp, filenameTimeOk := parser.ParseFilenameTimestamp(filepath.Base(entry.Path))
	var exifTimestamp time.Time
	var exifTimeOk bool
	var exifGPS parser.GPSInfo
	var exifGPSOk bool

	if !filenameTimeOk {
		// Filename can't be parsed, try exiftool (single call for both timestamp and GPS)
		exifInfo, err := parser.ParseEXIFAll(entry.Path)
		if err == nil && exifInfo != nil {
			exifTimestamp = exifInfo.Timestamp
			exifTimeOk = exifInfo.TimestampOk
			if exifInfo.GPSOk {
				exifGPS = parser.GPSInfo{
					Lat: exifInfo.Latitude,
					Lon: exifInfo.Longitude,
					Has: true,
				}
				exifGPSOk = true
			}
		}
	} else {
		// Filename parsed successfully, still get GPS from exiftool if needed
		exifGPS = parser.ParseEXIFGPS(entry.Path)
		exifGPSOk = exifGPS.Has
	}

	jsonTimestamp := time.Time{}
	jsonTimeOk := false
	if jsonResult != nil && !jsonResult.Timestamp.IsZero() {
		jsonTimestamp = jsonResult.Timestamp
		jsonTimeOk = true
	}

	// Determine final timestamp
	finalTimestamp := time.Time{}
	timestampSource := "none"
	if exifTimeOk {
		finalTimestamp = exifTimestamp
		timestampSource = "exif"
	} else if filenameTimeOk {
		finalTimestamp = filenameTimestamp
		timestampSource = "filename"
	} else if jsonTimeOk {
		finalTimestamp = jsonTimestamp
		timestampSource = "json"
	}

	if finalTimestamp.IsZero() {
		mu.Lock()
		stats.SkippedNoTime++
		logger.Skip("no_timestamp", entry.RelPath)
		mu.Unlock()
		return
	}

	// Step 3c: Use GPS from exiftool (already extracted) or JSON
	var jsonGPS parser.GPSInfo
	jsonGPSOk := false
	if jsonResult != nil && (jsonResult.Lat != 0 || jsonResult.Lon != 0) {
		jsonGPS = parser.GPSInfo{
			Lat: jsonResult.Lat,
			Lon: jsonResult.Lon,
			Alt: jsonResult.Alt,
			Has: true,
		}
		jsonGPSOk = true
	}

	finalGPS := parser.GPSInfo{}
	gpsSource := "none"
	if exifGPSOk {
		finalGPS = exifGPS
		gpsSource = "exif"
	} else if jsonGPSOk {
		finalGPS = jsonGPS
		gpsSource = "json"
	}

	// Step 3d: Check if format is supported by exiftool (uses cached file type detection)
	if !IsWriteSupported(entry.Path) {
		mu.Lock()
		stats.FailedExif++
		logger.Fail("filetype_unsupported", entry.RelPath, "exiftool does not support writing this format")
		mu.Unlock()
		moveToError(entry, outputDir, jsonResult)
		return
	}

	// Step 3e: Copy file to output (flat) and compute SHA-256 in single pass
	dstPath, sha256, exists, err := CopyAndHash(entry.Path, outputDir)
	if err != nil {
		mu.Lock()
		stats.FailedOther++
		logger.Fail("copy_error", entry.RelPath, err.Error())
		mu.Unlock()
		moveToError(entry, outputDir, jsonResult)
		return
	}
	if exists {
		mu.Lock()
		stats.SkippedExists++
		logger.Skip("file_exists", entry.RelPath)
		mu.Unlock()
		return
	}

	// Step 3f: Detect file type and temporarily rename for exiftool
	exifPath, cleanupRename, err := handleTypeMismatch(dstPath, entry, outputDir, jsonResult, logger, stats, mu)
	if err != nil {
		// handleTypeMismatch already logs and moves to error
		return
	}

	// Step 3g: exiftool write (use exifPath which may have different extension)
	hasGPS := finalGPS.Has
	if err := exifWriter.WriteAll(exifPath, finalTimestamp, hasGPS, finalGPS.Lat, finalGPS.Lon); err != nil {
		cleanupRename() // restore original name before moving to error
		mu.Lock()
		stats.FailedExif++
		logger.Fail("exiftool_write", entry.RelPath, err.Error())
		mu.Unlock()
		moveToErrorByPath(dstPath, entry.RelPath, outputDir, jsonResult)
		return
	}

	// Restore original filename before metadata
	cleanupRename()

	// Step 3f: Write metadata JSON
	meta := &Metadata{
		OriginalPath:   entry.RelPath,
		OutputFilename: filepath.Base(dstPath),
		SHA256:         sha256,
		Timestamp: TSInfo{
			Final:  timeStr(finalTimestamp),
			Source: timestampSource,
		},
		DeviceFolder: deviceFolder,
		DeviceType:   deviceType,
	}

	if exifTimeOk {
		meta.Timestamp.EXIF = timeStr(exifTimestamp)
	}
	if filenameTimeOk {
		meta.Timestamp.Filename = timeStr(filenameTimestamp)
	}
	if jsonTimeOk {
		meta.Timestamp.JSON = timeStr(jsonTimestamp)
	}

	if finalGPS.Has {
		meta.GPS = &GPSInfo{
			Lat:    finalGPS.Lat,
			Lon:    finalGPS.Lon,
			Source: gpsSource,
		}
		if exifGPSOk {
			meta.GPS.EXIF = &GPSPoint{Lat: exifGPS.Lat, Lon: exifGPS.Lon}
		}
		if jsonGPSOk {
			meta.GPS.JSON = &GPSPoint{Lat: jsonGPS.Lat, Lon: jsonGPS.Lon}
		}
	}

	if err := WriteMetadata(metadataDir, meta); err != nil {
		mu.Lock()
		stats.FailedOther++
		logger.Fail("metadata_write", entry.RelPath, err.Error())
		mu.Unlock()
		moveToErrorByPath(dstPath, entry.RelPath, outputDir, jsonResult)
		return
	}

	mu.Lock()
	stats.Processed++
	mu.Unlock()
}

// handleTypeMismatch detects the actual file type and temporarily renames for exiftool.
// Returns the temporary path for exiftool, a cleanup function to restore the original name,
// or an error if the file should be moved to the error directory.
func handleTypeMismatch(dstPath string, entry FileEntry, outputDir string,
	jsonResult *matcher.JSONLookupResult, logger *Logger, stats *Stats, mu *sync.Mutex) (string, func(), error) {

	noOp := func() {}

	newExt, err := DetectFileType(dstPath)
	if err != nil {
		// Can't detect type, continue anyway
		return dstPath, noOp, nil
	}
	if newExt == "" {
		// Type matches current extension, no rename needed
		return dstPath, noOp, nil
	}

	// Type mismatch: compute temporary filename
	base := strings.TrimSuffix(filepath.Base(dstPath), filepath.Ext(dstPath))
	tmpName := base + newExt
	tmpPath := filepath.Join(outputDir, tmpName)

	// Check if temp target already exists
	if _, err := os.Stat(tmpPath); err == nil {
		// Conflict: move both image and JSON to error
		mu.Lock()
		stats.FailedOther++
		logger.Fail("rename_conflict", entry.RelPath,
			fmt.Sprintf("actual: %s, target: %s exists", strings.TrimPrefix(newExt, "."), tmpName))
		mu.Unlock()
		moveToErrorByPath(dstPath, entry.RelPath, outputDir, jsonResult)
		return "", noOp, fmt.Errorf("rename conflict")
	}

	// Temporary rename
	if err := os.Rename(dstPath, tmpPath); err != nil {
		mu.Lock()
		stats.FailedOther++
		logger.Fail("rename_error", entry.RelPath, err.Error())
		mu.Unlock()
		moveToErrorByPath(dstPath, entry.RelPath, outputDir, jsonResult)
		return "", noOp, fmt.Errorf("rename: %w", err)
	}

	// Cleanup function to restore original name
	cleanup := func() {
		os.Rename(tmpPath, dstPath)
	}

	return tmpPath, cleanup, nil
}

// copyToPath copies src to dst using streaming io.Copy, leaving src untouched.
func copyToPath(src, dst string) error {
	srcF, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcF.Close()

	dstF, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer dstF.Close()

	_, err = io.Copy(dstF, srcF)
	return err
}

// moveToError copies the source file and its JSON sidecar into the error
// directory without removing them from the input tree (input stays read-only).
func moveToError(entry FileEntry, outputDir string, jsonResult *matcher.JSONLookupResult) {
	errorDir := filepath.Join(outputDir, "error", filepath.Dir(entry.RelPath))
	if err := os.MkdirAll(errorDir, 0755); err != nil {
		return
	}

	// Copy source file into error dir (do not rename out of input tree)
	dstError := filepath.Join(errorDir, filepath.Base(entry.Path))
	copyToPath(entry.Path, dstError)

	// Copy JSON sidecar if exists (do not rename out of input tree)
	if jsonResult != nil {
		jsonError := filepath.Join(errorDir, filepath.Base(jsonResult.JSONFile))
		copyToPath(jsonResult.JSONFile, jsonError)
	}
}

// moveToErrorByPath moves an already-copied file from outputDir to error directory
// and copies the JSON sidecar from the input tree (without removing it from input).
func moveToErrorByPath(srcPath, relPath, outputDir string, jsonResult *matcher.JSONLookupResult) {
	errorDir := filepath.Join(outputDir, "error", filepath.Dir(relPath))
	if err := os.MkdirAll(errorDir, 0755); err != nil {
		return
	}

	// Move the file from output to error (both are inside outputDir, rename is safe)
	dstError := filepath.Join(errorDir, filepath.Base(srcPath))
	os.Rename(srcPath, dstError)

	// Copy JSON sidecar from input tree into error dir (do not rename out of input tree)
	if jsonResult != nil {
		jsonError := filepath.Join(errorDir, filepath.Base(jsonResult.JSONFile))
		copyToPath(jsonResult.JSONFile, jsonError)
	}
}

func checkOutputDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read output dir: %w", err)
	}
	if len(entries) > 0 {
		return fmt.Errorf("output directory is not empty (%d entries), please clean it first", len(entries))
	}
	return nil
}
