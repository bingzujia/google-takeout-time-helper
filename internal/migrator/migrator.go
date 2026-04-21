package migrator

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bingzujia/google-takeout-time-helper/internal/logutil"
	"github.com/bingzujia/google-takeout-time-helper/internal/matcher"
	"github.com/bingzujia/google-takeout-time-helper/internal/mediatype"
	"github.com/bingzujia/google-takeout-time-helper/internal/organizer"
	"github.com/bingzujia/google-takeout-time-helper/internal/parser"
	"github.com/bingzujia/google-takeout-time-helper/internal/progress"
	"github.com/bingzujia/google-takeout-time-helper/internal/workerpool"
)

// Stats holds processing statistics.
type Stats struct {
	Scanned       int
	Processed     int
	SkippedExists int
	FailedExif    int
	FailedOther   int
	ManualReview  int // files that couldn't have EXIF written but are otherwise valid
}

// Config holds migration settings.
type Config struct {
	InputDir     string
	OutputDir    string
	ShowProgress bool            // whether to display progress bar
	DryRun       bool            // preview only — no file operations
	Logger       *logutil.Logger // structured log; if nil a Nop logger is used
}

// FileEntry holds pre-scanned file information.
type FileEntry struct {
	Path    string // absolute path
	RelPath string // relative path (for logging)
}

// Run executes the full migration pipeline.
func Run(cfg Config) (*Stats, error) {
	// Dry-run mode: skip output directory validation and creation
	if cfg.DryRun {
		return runDry(cfg)
	}

	// Step 1: Check output directory
	if err := checkOutputDir(cfg.OutputDir); err != nil {
		return nil, err
	}

	// Create output directories
	metadataDir := filepath.Join(cfg.OutputDir, "metadata")
	if err := os.MkdirAll(metadataDir, 0755); err != nil {
		return nil, fmt.Errorf("create metadata dir: %w", err)
	}
	manualReviewDir := filepath.Join(cfg.OutputDir, "manual_review")
	if err := os.MkdirAll(manualReviewDir, 0755); err != nil {
		return nil, fmt.Errorf("create manual review dir: %w", err)
	}

	// Step 2: Resolve logger
	logger := cfg.Logger
	if logger == nil {
		logger = logutil.Nop()
	}

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
	entries, err := scanFiles(yearFolders, cfg.InputDir)
	if err != nil {
		return nil, fmt.Errorf("scan files: %w", err)
	}
	if len(entries) == 0 {
		fmt.Println("No media files found in year folders.")
		return &Stats{}, nil
	}
	fmt.Printf("Found %d files in %d year folder(s)\n\n", len(entries), len(yearFolders))

	// Phase 2: Process files with progress bar
	stats := &Stats{}
	processFiles(entries, cfg.OutputDir, metadataDir, manualReviewDir, logger, stats, cfg.ShowProgress)

	return stats, nil
}

// scanFiles collects all media files from the given year folders.
func scanFiles(yearFolders []string, inputDir string) ([]FileEntry, error) {
	var entries []FileEntry
	for _, yf := range yearFolders {
		if err := filepath.Walk(yf, func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if info.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			if !mediatype.IsImage(ext) && !mediatype.IsVideo(ext) {
				return nil
			}
			relPath, relErr := filepath.Rel(inputDir, path)
			if relErr != nil {
				relPath = path // fallback to absolute path
			}
			entries = append(entries, FileEntry{
				Path:    path,
				RelPath: relPath,
			})
			return nil
		}); err != nil {
			return nil, fmt.Errorf("walk %s: %w", yf, err)
		}
	}
	return entries, nil
}

// processFiles iterates over all entries and processes each one concurrently.
func processFiles(entries []FileEntry, outputDir, metadataDir, manualReviewDir string,
	logger *logutil.Logger, stats *Stats, showProgress bool) {

	var statsMu sync.Mutex // protects Stats fields
	var processed atomic.Int64
	total := len(entries)
	reporter := progress.NewReporter(total, showProgress)
	defer reporter.Close()

	dirCache := &matcher.DirCache{}

	_ = workerpool.Run(entries, workerpool.DefaultWorkers(), func(entry FileEntry) error {
		processSingleFile(entry, outputDir, metadataDir, manualReviewDir, logger, stats, &statsMu, dirCache)
		reporter.Update(int(processed.Add(1)))
		return nil
	})
}

// processSingleFile handles one media file through the full pipeline.
func processSingleFile(entry FileEntry, outputDir, metadataDir, manualReviewDir string,
	logger *logutil.Logger, stats *Stats, statsMu *sync.Mutex, dirCache *matcher.DirCache) {

	statsMu.Lock()
	stats.Scanned++
	statsMu.Unlock()

	// Step 3a: Match JSON sidecar
	jsonResult := matcher.JSONForFile(entry.Path, dirCache)
	var deviceFolder, deviceType, destDir string
	if jsonResult == nil {
		logger.Info("no_json_sidecar", entry.RelPath)
		destDir = outputDir
	} else {
		deviceFolder = jsonResult.DeviceFolder
		deviceType = jsonResult.DeviceType
		// Determine destination directory based on LocalFolderName
		if jsonResult.LocalFolderName != "" {
			destDir = filepath.Join(outputDir, jsonResult.LocalFolderName)
		} else {
			destDir = outputDir
		}
	}

	// Create device folder if needed
	if destDir != outputDir {
		if err := os.MkdirAll(destDir, 0755); err != nil {
			statsMu.Lock()
			stats.FailedOther++
			statsMu.Unlock()
			logger.Fail("mkdir_error", entry.RelPath, err.Error())
			return
		}
	}

	// Step 3b: Extract JSON timestamps and GPS
	jsonTimestamp := time.Time{}
	var jsonGPS parser.GPSInfo
	jsonGPSOk := false
	fileModifyTs := int64(0)
	modifyTsSource := ""
	shouldMoveModifyTs := false

	if jsonResult != nil {
		if !jsonResult.Timestamp.IsZero() {
			jsonTimestamp = jsonResult.Timestamp
		}
		if jsonResult.Lat != 0 || jsonResult.Lon != 0 {
			jsonGPS = parser.GPSInfo{
				Lat: jsonResult.Lat,
				Lon: jsonResult.Lon,
				Alt: jsonResult.Alt,
				Has: true,
			}
			jsonGPSOk = true
		}
		// Resolve ModifyTime timestamp (unified priority: photoTakenTime → creationTime → manual_review)
		fileModifyTs, modifyTsSource, shouldMoveModifyTs = resolveModifyTimestamp(jsonResult)
	} else {
		// No JSON sidecar — requires manual review
		modifyTsSource = "manual_review"
		shouldMoveModifyTs = true
	}

	// Step 3c: Extract EXIF GPS to determine if JSON GPS supplement is needed
	exifGPS := parser.ParseEXIFGPS(entry.Path)
	exifGPSOk := exifGPS.Has

	// Check if we should move to manual_review due to missing timestamps
	if shouldMoveModifyTs {
		statsMu.Lock()
		stats.ManualReview++
		statsMu.Unlock()
		logger.Info("missing_timestamps", entry.RelPath)
		moveToManualReview(entry, outputDir, manualReviewDir, jsonResult, jsonTimestamp,
			exifGPS, exifGPSOk, jsonGPS, jsonGPSOk, deviceFolder, deviceType, "missing_timestamps")
		return
	}

	// Step 3e: Copy file to output; SHA-256 computed at copy time.
	dstPath, copySHA256, exists, err := copyAndHash(entry.Path, destDir)
	if err != nil {
		statsMu.Lock()
		stats.FailedOther++
		statsMu.Unlock()
		logger.Fail("copy_error", entry.RelPath, err.Error())
		moveToError(entry, outputDir, jsonResult)
		return
	}
	if exists {
		statsMu.Lock()
		stats.SkippedExists++
		statsMu.Unlock()
		logger.Skip("file_exists", entry.RelPath)
		return
	}

	// Step 3f: Apply file ModifyTime using os.Chtimes() if we have a timestamp
	// New behavior: Set file system timestamp using photoTakenTime or creationTime (fallback)
	if fileModifyTs != 0 {
		if err := applyFileTimestamp(dstPath, fileModifyTs); err != nil {
			statsMu.Lock()
			stats.ManualReview++
			statsMu.Unlock()
			logger.Fail("timestamp_write", entry.RelPath, err.Error())
			moveToManualReviewByPath(dstPath, entry.RelPath, outputDir, manualReviewDir, jsonResult, jsonTimestamp,
				exifGPS, exifGPSOk, jsonGPS, jsonGPSOk, deviceFolder, deviceType, "timestamp_error")
			return
		}
	}

	// Step 3g: Write metadata JSON
	finalGPS, gpsSource := resolveGPS(exifGPS, exifGPSOk, jsonGPS, jsonGPSOk)
	meta := buildMetadata(entry.RelPath, filepath.Base(dstPath), jsonTimestamp, finalGPS, gpsSource, deviceFolder, deviceType, "", modifyTsSource, modifyTsSource)
	meta.SHA256 = copySHA256
	if err := writeMetadata(metadataDir, meta); err != nil {
		statsMu.Lock()
		stats.FailedOther++
		statsMu.Unlock()
		logger.Fail("metadata_write", entry.RelPath, err.Error())
		moveToErrorByPath(dstPath, entry.RelPath, outputDir, jsonResult)
		return
	}

	statsMu.Lock()
	stats.Processed++
	statsMu.Unlock()
}

// handleTypeMismatch detects the actual file type and temporarily renames for exiftool.
// Returns the temporary path for exiftool, a cleanup function to restore the original name,
// or an error if the file should be moved to the error directory.
func handleTypeMismatch(dstPath string, entry FileEntry, outputDir string,
	jsonResult *matcher.JSONLookupResult, logger *logutil.Logger, stats *Stats, statsMu *sync.Mutex) (string, func() error, error) {

	noOp := func() error { return nil }

	newExt, err := detectFileType(dstPath)
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

	// Check if temp target already exists — use counter-based suffix
	if _, err := os.Stat(tmpPath); err == nil {
		ext := newExt
		stem := base
		for i := 1; ; i++ {
			candidate := filepath.Join(outputDir, fmt.Sprintf("%s_%d%s", stem, i, ext))
			if _, err := os.Stat(candidate); os.IsNotExist(err) {
				tmpName = fmt.Sprintf("%s_%d%s", stem, i, ext)
				tmpPath = candidate
				break
			}
		}
	}

	// Temporary rename
	if err := os.Rename(dstPath, tmpPath); err != nil {
		statsMu.Lock()
		stats.FailedOther++
		statsMu.Unlock()
		logger.Fail("rename_error", entry.RelPath, err.Error())
		moveToErrorByPath(dstPath, entry.RelPath, outputDir, jsonResult)
		return "", noOp, fmt.Errorf("rename: %w", err)
	}

	// Cleanup function to restore original name
	cleanup := func() error {
		return os.Rename(tmpPath, dstPath)
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
	if err := os.Rename(srcPath, dstError); err != nil {
		// Fallback: copy + remove
		if copyErr := copyToPath(srcPath, dstError); copyErr != nil {
			// Both rename and copy failed — file may remain in output dir
			// Best effort: log and move on
			return
		}
		os.Remove(srcPath)
	}

	// Copy JSON sidecar from input tree into error dir (do not rename out of input tree)
	if jsonResult != nil {
		jsonError := filepath.Join(errorDir, filepath.Base(jsonResult.JSONFile))
		copyToPath(jsonResult.JSONFile, jsonError)
	}
}

// resolveGPS picks the best GPS source: EXIF first, JSON second.
func resolveGPS(exifGPS parser.GPSInfo, exifGPSOk bool, jsonGPS parser.GPSInfo, jsonGPSOk bool) (parser.GPSInfo, string) {
	if exifGPSOk {
		return exifGPS, "exif"
	}
	if jsonGPSOk {
		return jsonGPS, "json"
	}
	return parser.GPSInfo{}, "none"
}

// resolvePhotoTimestamp returns the Unix timestamp for file ModifyTime from JSON.
// Priority: photoTakenTime → 0 (no fallback)
// Returns (timestamp, source, shouldMoveToManualReview)
// Updated: Both CreateDate and FileModifyDate now use photoTakenTime priorityuniformly,
// with separate fallback logic in resolveModifyTimestamp()
func resolvePhotoTimestamp(jsonResult *matcher.JSONLookupResult) (int64, string, bool) {
	if jsonResult == nil {
		return 0, "manual_review", true
	}
	if jsonResult.PhotoTakenTimeUnix != 0 {
		return jsonResult.PhotoTakenTimeUnix, "photoTakenTime", false
	}
	return 0, "manual_review", true
}

// resolveModifyTimestamp returns the Unix timestamp for file ModifyTime from JSON.
// Priority: photoTakenTime (優先) → creationTime (fallback) → 0 (manual_review)
// Returns (timestamp, source, shouldMoveToManualReview)
// Updated behavior: photoTakenTime is now the primary source for ModifyTime (not just fallback)
func resolveModifyTimestamp(jsonResult *matcher.JSONLookupResult) (int64, string, bool) {
	if jsonResult == nil {
		return 0, "manual_review", true
	}
	// Priority 1: photoTakenTime (優先使用拍摄时间)
	if jsonResult.PhotoTakenTimeUnix != 0 {
		return jsonResult.PhotoTakenTimeUnix, "photoTakenTime", false
	}
	// Priority 2: creationTime (fallback 使用上传时间)
	if jsonResult.CreationTimeUnix != 0 {
		return jsonResult.CreationTimeUnix, "creationTime", false
	}
	// Both missing: manual_review
	return 0, "manual_review", true
}

// applyFileTimestamp sets file modification time using os.Chtimes() with error handling.
// Returns error if the operation fails (permission denied, invalid timestamp, etc.)
// Returns nil if successful.
func applyFileTimestamp(filePath string, modifyTimestamp int64) error {
	if modifyTimestamp <= 0 {
		// Invalid timestamp, skip silently (should be caught earlier)
		return nil
	}

	// Convert Unix timestamp to time.Time
	mtime := time.Unix(modifyTimestamp, 0)

	// Use same time for both atime and mtime
	// os.Chtimes(path, atime, mtime) modifies access and modification times
	err := os.Chtimes(filePath, mtime, mtime)
	if err != nil {
		return fmt.Errorf("os.Chtimes(%s): %w", filePath, err)
	}

	return nil
}

// moveToManualReview copies a file (not yet in output) to the manual_review directory
// with its own metadata JSON for later manual handling.
func moveToManualReview(entry FileEntry, outputDir, manualReviewDir string,
	jsonResult *matcher.JSONLookupResult,
	jsonTimestamp time.Time,
	exifGPS parser.GPSInfo, exifGPSOk bool,
	jsonGPS parser.GPSInfo, jsonGPSOk bool,
	deviceFolder, deviceType, reviewReason string) {

	// Copy file to manual_review
	reviewFileDir := filepath.Join(manualReviewDir, filepath.Dir(entry.RelPath))
	if err := os.MkdirAll(reviewFileDir, 0755); err != nil {
		return
	}
	dstReview := filepath.Join(reviewFileDir, filepath.Base(entry.Path))
	if err := copyToPath(entry.Path, dstReview); err != nil {
		return
	}

	// Copy JSON sidecar if exists
	if jsonResult != nil {
		jsonReviewDir := filepath.Join(manualReviewDir, filepath.Dir(entry.RelPath))
		jsonReview := filepath.Join(jsonReviewDir, filepath.Base(jsonResult.JSONFile))
		copyToPath(jsonResult.JSONFile, jsonReview)
	}

	// Write metadata JSON to manual_review/metadata/
	manualMetaDir := filepath.Join(manualReviewDir, "metadata")
	finalGPS, gpsSource := resolveGPS(exifGPS, exifGPSOk, jsonGPS, jsonGPSOk)
	meta := buildMetadata(entry.RelPath, filepath.Base(entry.Path), jsonTimestamp, finalGPS, gpsSource, deviceFolder, deviceType, reviewReason, "manual_review", "manual_review")
	sha256, err := hashFile(dstReview)
	if err == nil {
		meta.SHA256 = sha256
	}
	writeMetadata(manualMetaDir, meta)
}

// moveToManualReviewByPath moves an already-copied file from outputDir to manual_review
// and writes metadata JSON there.
func moveToManualReviewByPath(srcPath, relPath, outputDir, manualReviewDir string,
	jsonResult *matcher.JSONLookupResult,
	jsonTimestamp time.Time,
	exifGPS parser.GPSInfo, exifGPSOk bool,
	jsonGPS parser.GPSInfo, jsonGPSOk bool,
	deviceFolder, deviceType, reviewReason string) {

	// Move file from output to manual_review
	reviewFileDir := filepath.Join(manualReviewDir, filepath.Dir(relPath))
	if err := os.MkdirAll(reviewFileDir, 0755); err != nil {
		return
	}
	dstReview := filepath.Join(reviewFileDir, filepath.Base(srcPath))
	if err := os.Rename(srcPath, dstReview); err != nil {
		// Fallback: copy + remove
		if copyErr := copyToPath(srcPath, dstReview); copyErr != nil {
			return
		}
		os.Remove(srcPath)
	}

	// Copy JSON sidecar from input tree
	if jsonResult != nil {
		jsonReviewDir := filepath.Join(manualReviewDir, filepath.Dir(relPath))
		jsonReview := filepath.Join(jsonReviewDir, filepath.Base(jsonResult.JSONFile))
		copyToPath(jsonResult.JSONFile, jsonReview)
	}

	// Write metadata JSON to manual_review/metadata/
	manualMetaDir := filepath.Join(manualReviewDir, "metadata")
	finalGPS, gpsSource := resolveGPS(exifGPS, exifGPSOk, jsonGPS, jsonGPSOk)
	meta := buildMetadata(relPath, filepath.Base(srcPath), jsonTimestamp, finalGPS, gpsSource, deviceFolder, deviceType, reviewReason, "manual_review", "manual_review")
	sha256, err := hashFile(dstReview)
	if err == nil {
		meta.SHA256 = sha256
	}
	writeMetadata(manualMetaDir, meta)
}

// buildMetadata constructs a Metadata struct from JSON sidecar data.
func buildMetadata(relPath, outputFilename string,
	jsonTimestamp time.Time,
	finalGPS parser.GPSInfo, gpsSource string,
	deviceFolder, deviceType, reviewReason string,
	createDateSource, fileModifyDateSource string) *Metadata {

	timestampSource := "none"
	jsonTS := ""
	if !jsonTimestamp.IsZero() {
		timestampSource = "json"
		jsonTS = timeStr(jsonTimestamp)
	}

	meta := &Metadata{
		OriginalPath:   relPath,
		OutputFilename: outputFilename,
		Timestamp: TSInfo{
			Final:  jsonTS,
			Source: timestampSource,
			JSON:   jsonTS,
		},
		DeviceFolder: deviceFolder,
		DeviceType:   deviceType,
		ReviewReason: reviewReason,
	}

	// Add separate timestamp source tracking
	if createDateSource != "" {
		meta.CreateDate = &TSSource{Source: createDateSource}
	}
	if fileModifyDateSource != "" {
		meta.FileModifyDate = &TSSource{Source: fileModifyDateSource}
	}

	if finalGPS.Has {
		meta.GPS = &GPSInfo{
			Lat:    finalGPS.Lat,
			Lon:    finalGPS.Lon,
			Source: gpsSource,
		}
	}

	return meta
}

// isUnsupportedFormatError checks if the exiftool error is due to an unsupported file format.
func isUnsupportedFormatError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "not yet supported") || strings.Contains(msg, "Writing of")
}

func checkOutputDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read output dir: %w", err)
	}
	// Count entries that are not created by the tool itself
	toolDirs := map[string]bool{
		logutil.LogDirName(): true,
		"metadata":           true,
		"manual_review":      true,
	}
	userEntries := 0
	for _, e := range entries {
		if !toolDirs[e.Name()] {
			userEntries++
		}
	}
	if userEntries > 0 {
		return fmt.Errorf("output directory is not empty (%d entries), please clean it first", userEntries)
	}
	return nil
}

// runDry executes the migration pipeline in dry-run mode.
// No files are copied, no EXIF is written, no directories are created.
func runDry(cfg Config) (*Stats, error) {
	// Classify folders
	yearFolders, _, err := organizer.ClassifyFolder(cfg.InputDir)
	if err != nil {
		return nil, fmt.Errorf("classify folders: %w", err)
	}

	if len(yearFolders) == 0 {
		fmt.Println("No year folders (Photos from XXXX) found.")
		return &Stats{}, nil
	}

	// Scan files
	fmt.Println("Scanning files...")
	entries, err := scanFiles(yearFolders, cfg.InputDir)
	if err != nil {
		return nil, fmt.Errorf("scan files: %w", err)
	}
	if len(entries) == 0 {
		fmt.Println("No media files found in year folders.")
		return &Stats{}, nil
	}
	fmt.Printf("Found %d files in %d year folder(s)\n\n", len(entries), len(yearFolders))

	// Process files in dry-run mode
	stats := &Stats{}
	dryRunProcessFiles(entries, cfg.InputDir, stats)

	return stats, nil
}

// dryRunProcessFiles scans all entries and prints what would happen, without modifying any files.
func dryRunProcessFiles(entries []FileEntry, inputDir string, stats *Stats) {
	for _, entry := range entries {
		dryRunProcessSingle(entry, inputDir, stats)
	}

	fmt.Println()
}

// dryRunProcessSingle analyzes one file and prints its status.
func dryRunProcessSingle(entry FileEntry, inputDir string, stats *Stats) {
	stats.Scanned++

	// Match JSON sidecar
	jsonResult := matcher.JSONForFile(entry.Path, nil)
	jsonStatus := "missing"
	if jsonResult != nil {
		jsonStatus = "matched"
	}

	// Extract JSON timestamp and GPS
	jsonTimestamp := time.Time{}
	jsonTimeOk := false
	var jsonGPS parser.GPSInfo
	jsonGPSOk := false
	if jsonResult != nil {
		if !jsonResult.Timestamp.IsZero() {
			jsonTimestamp = jsonResult.Timestamp
			jsonTimeOk = true
		}
		if jsonResult.Lat != 0 || jsonResult.Lon != 0 {
			jsonGPS = parser.GPSInfo{
				Lat: jsonResult.Lat,
				Lon: jsonResult.Lon,
				Has: true,
			}
			jsonGPSOk = true
		}
	}

	// Extract EXIF GPS
	exifGPS := parser.ParseEXIFGPS(entry.Path)
	exifGPSOk := exifGPS.Has

	willWriteExif := jsonTimeOk || (!exifGPSOk && jsonGPSOk)

	// Determine final GPS for display
	finalGPS, gpsSource := resolveGPS(exifGPS, exifGPSOk, jsonGPS, jsonGPSOk)

	// Format output strings
	jsonTimeStr := "none"
	finalTimeSrc := "none"
	if jsonTimeOk {
		jsonTimeStr = jsonTimestamp.Format("2006-01-02 15:04:05")
		finalTimeSrc = "json"
	}
	gpsStr := "no"
	if finalGPS.Has {
		gpsStr = fmt.Sprintf("yes (%s)", gpsSource)
	}

	// Check if format is supported (only relevant when we'd write)
	reviewReason := ""
	if willWriteExif && !isWriteSupported(entry.Path) {
		reviewReason = "exif_unsupported"
	}

	// Print status
	if reviewReason != "" {
		stats.ManualReview++
		fmt.Printf("  [REVIEW   ] %-50s Time: %-20s (%-8s) GPS: %-12s JSON: %-7s Reason: %s\n",
			entry.RelPath, jsonTimeStr, finalTimeSrc, gpsStr, jsonStatus, reviewReason)
	} else {
		stats.Processed++
		fmt.Printf("  [PROCESSED] %-50s Time: %-20s (%-8s) GPS: %-12s JSON: %-7s\n",
			entry.RelPath, jsonTimeStr, finalTimeSrc, gpsStr, jsonStatus)
	}
}
