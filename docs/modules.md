# Internal Packages Reference

This document provides detailed reference for all 18 internal packages (`internal/*`), including their roles, responsibilities, key APIs, and usage patterns.

## Quick Reference

| Package | Role | Key Exports |
|---------|------|-------------|
| **matcher** | JSON sidecar matching | JSONForFile, ResolveTimestamp, ResolveGPS |
| **migrator** | Migration pipeline | Migrate, Stats |
| **parser** | Timestamp/GPS parsing | ParseTime, ParseGPS |
| **heicconv** | HEIC encoding & conversion | Convert, Config, Stats |
| **exifrunner** | Batch exiftool operations | BatchQuery, BatchWrite |
| **workerpool** | Generic goroutine pool | Run |
| **logutil** | Thread-safe file logging | Logger, Nop |
| **organizer** | Year folder classification | Organize |
| **classifier** | Media type classification | Classify, Stats |
| **mediatype** | Content-based type detection | DetectMediaType |
| **dedup** | Perceptual deduplication | ProcessDirectory, Stats |
| **hashcache** | SQLite hash caching | Cache |
| **destlocker** | Concurrent write prevention | Locker |
| **renamer** | Filename generation | GenerateName, Stats |
| **progress** | Terminal progress bar | Bar |
| **fileutil** | File operation helpers | CopyFile, SafeRemove |
| **qqmedia** | QQ media format handling | FormatQQMedia, Stats |
| **screenshotter** | Screenshot detection | IsScreenshot |

---

## matcher

**Location:** `internal/matcher/`

**Role:** Locates the JSON sidecar file for each photo using a multi-step degradation strategy to handle Google Takeout's filename truncation and encoding quirks.

**Key Responsibility:** Solve the "JSON sidecar matching problem" — Google Takeout truncates sidecar filenames at 51 characters, uses locale-specific "edited" suffixes, and rearranges bracket-numbers, making exact matching impossible.

### Main API

```go
// JSONForFile locates the JSON sidecar for a given photo file
// Returns nil if no sidecar found after 6-step degradation
func JSONForFile(photoPath string, dir string) *JSONPath

// ResolveTimestamp extracts timestamp from EXIF, filename, or JSON sidecar
// Priority: EXIF DateTimeOriginal → filename timestamp → JSON photoTakenTime
func ResolveTimestamp(photoPath string, exifDate *time.Time, jsonTime *time.Time) *time.Time

// ResolveGPS extracts GPS from EXIF or JSON geoData
// Priority: EXIF GPS tags → JSON geoData
func ResolveGPS(exifGPS *GPS, jsonGPS *GPS) *GPS
```

### Matching Strategy (6 steps, in order)

1. **Identity** — Exact filename match: `IMG_0030.JPG.json`
2. **ShortenName** — Truncate photo name to 51 chars + `.json`
3. **BracketSwap** — Rearrange bracket-numbers: `IMG_0030 (1).JPG` → `IMG_0030.JPG` → match sidecar
4. **RemoveExtra** — Remove common suffixes like `_edited`: `IMG_0030_edited.JPG` → `IMG_0030.JPG`
5. **Supplemental** — Try supplemental sidecar if main not found
6. **NoExtension** — Last resort: match by name without extension

### Key Types

```go
type JSONPath struct {
    Path string      // Full path to JSON sidecar
    Name string      // Filename of sidecar
}

type GPS struct {
    Latitude  float64
    Longitude float64
    Altitude  float64
}
```

### Extension Points

- Add new matching step to handle edge cases (e.g., new Google Takeout versions)
- Modify degradation strategy order if specific pattern becomes more common

### Usage Example

```go
import "github.com/bingzujia/google-takeout-time-helper/internal/matcher"

// In a migration loop
photoPath := "Photos/2023/IMG_0030.JPG"
jsonSidecar := matcher.JSONForFile(photoPath, "Photos/2023")
if jsonSidecar != nil {
    // Parse JSON and extract timestamp/GPS
}
```

### Design Notes

- **Do NOT** add heuristics elsewhere in the codebase — all edge cases should be handled in this package
- The 6-step strategy is comprehensive; additional cases should be added to this package, not scattered through migrator
- See memory notes on "JSON sidecar matching strategy" for historical context on why truncation occurs

---

## migrator

**Location:** `internal/migrator/`

**Role:** Core migration pipeline for copying photos and restoring metadata from Google Takeout JSON sidecars.

**Key Responsibility:** Orchestrate the migration workflow: scan year folders, copy files, parse JSON metadata, resolve timestamps/GPS, write EXIF via exiftool, and generate metadata tracking JSON files.

### Main API

```go
type Config struct {
    InputDir    string
    OutputDir   string
    Year        string  // Optional: process only specific year
    Quality     int     // HEIC encoding quality
    Dry         bool    // Preview only
}

type Stats struct {
    Processed   int  // Files successfully copied
    Skipped     int  // Files without JSON or other skip reason
    Failed      int  // Files with errors
    // ... other counters
}

// Migrate performs the migration pipeline
func Migrate(config Config, logger logutil.Logger) (*Stats, error)
```

### Pipeline Flow

```
1. Scan input directory for year folders (Photos from XXXX)
   ↓
2. For each year folder:
   2.1 List all files
   2.2 For each file:
       2.2.1 Locate JSON sidecar via matcher.JSONForFile
       2.2.2 Parse JSON metadata (photoTakenTime, geoData)
       2.2.3 Resolve timestamp (priority: EXIF → filename → JSON)
       2.2.4 Resolve GPS (priority: EXIF → JSON)
       2.2.5 Copy file to output directory
       2.2.6 Write EXIF metadata via exifrunner
       2.2.7 Generate metadata JSON file for audit trail
       2.2.8 Log result (Processed/Skipped/Failed)
   ↓
3. Return Stats with summary counts
```

### Key Behaviors

- **Files without JSON sidecars**: Still copied (counted as Processed, not Failed) — prioritize file integrity over metadata
- **exiftool availability**: Gracefully degrades when exiftool unavailable
- **Dry-run mode**: No file writes or logging (uses `logutil.Nop()`)
- **Log output**: Stored at `takeout-helper-log/migrate-{date}-{index}.log`

### Extension Points

- Override metadata resolution priority in `ResolveTimestamp` / `ResolveGPS`
- Add new file filtering (e.g., size limits)
- Customize metadata JSON format written for audit

### Usage Example

```go
import "github.com/bingzujia/google-takeout-time-helper/internal/migrator"

stats, err := migrator.Migrate(migrator.Config{
    InputDir:  "~/Takeout",
    OutputDir: "~/Photos",
    Year:      "2023",
    Dry:       false,
}, logger)

fmt.Printf("Processed: %d, Skipped: %d, Failed: %d\n", 
    stats.Processed, stats.Skipped, stats.Failed)
```

---

## parser

**Location:** `internal/parser/`

**Role:** Parse timestamps and GPS coordinates from various formats (EXIF DateTimeOriginal, filenames, JSON metadata).

**Key Responsibility:** Handle the diversity of timestamp formats in Google Takeout files and user-provided media.

### Main API

```go
// ParseTime extracts time from various timestamp formats
func ParseTime(s string) (*time.Time, error)

// ParseGPS extracts GPS from EXIF or JSON format
func ParseGPS(latitude, longitude, altitude string) (*GPS, error)

// SupportedFormats lists all recognized timestamp patterns
var SupportedFormats = []string{
    "YYYY-MM-DD HH:MM:SS",      // EXIF standard
    "YYYYMMDDHHMMSS",            // Dense format
    "IMG_YYYYMMDD_HHMMSS",       // Common camera pattern
    // ... 10+ more patterns
}
```

### Supported Timestamp Formats

- EXIF standard: `2023-01-15 14:30:45`
- Dense: `20230115143045`
- Filename patterns: `IMG_20230115_143045`, `VID_20230115_143045`
- WeChat: `mmexport1673794245` (Unix timestamp)
- QQ: `_20230115_143045`
- Screenshot: `Screenshot_2023-01-15-14-30-45-000`
- Unix timestamp (seconds): `1673794245`
- Unix timestamp (milliseconds): `1673794245000`

### Extension Points

- Add new timestamp format by extending `ParseTime` function
- Customize GPS parsing (currently handles degrees, minutes, seconds conversion)

### Usage Example

```go
import "github.com/bingzujia/google-takeout-time-helper/internal/parser"

t, err := parser.ParseTime("IMG_20230115_143045")  // Returns 2023-01-15 14:30:45
```

---

## heicconv

**Location:** `internal/heicconv/`

**Role:** HEIC encoding and conversion pipeline with quality tuning, chroma handling, and metadata preservation.

**Key Responsibility:** Convert images to HEIC format with proper:
- Quality level control (1–100 scale)
- Chroma subsampling (always 4:2:0 for HEIC compatibility)
- Metadata copy (EXIF, XMP, ICC profile)
- Dimension validation (reject > 16383 px)

### Main API

```go
type Config struct {
    InputDir  string
    OutputDir string
    Quality   int  // 1-100 (default 75)
    Dry       bool // Preview only
}

type Stats struct {
    Processed       int  // Successfully converted
    Skipped         int  // Extension mismatch, already HEIC, etc.
    SkippedOversizeDim int  // Dimension > 16383 px
    Failed          int  // Conversion errors
}

// Convert performs the conversion pipeline
func Convert(config Config, logger logutil.Logger) (*Stats, error)
```

### Quality Levels

- **35–50**: Photos only (screenshots show artifacts)
- **75** (default): Balanced for mixed media
- **85–95**: High quality (larger files)

### Metadata Handling

```
Source Image ──→ heif-enc (quality, chroma) ──→ HEIC encoded
                        ↓
                  [CopyAll] Copy EXIF:all, XMP:all, ICC_Profile:all
                        ↓
                  [RestoreKeyTimes] Write timestamp tags if needed
                        ↓
                  Output HEIC with metadata
```

### Chroma Subsampling

- **Decision:** Always output 4:2:0 chroma (HEIC/HEIF standard)
- **Reason:** HEIC brand requires 4:2:0; other ratios trigger Range Extensions profile (profile_idc=4) which decoders reject
- **Result:** Profile remains Main Still Picture (profile_idc=3) — all platforms decode

### Dimension Limits

- **Hard limit:** Images with dimension > 16,383 px are skipped with `ErrDimensionTooLarge`
- **Reason:** Apple ImageIO (HEIC decoder) caps at 16383 px per dimension
- **Detection:** Checked via `DecodeConfig` before encoding

### Extension Points

- Adjust quality default `heifEncQuality = 75`
- Customize metadata groups copied in `CopyAll` (currently: EXIF, XMP, ICC_Profile)
- Handle new image formats by extending decoder

### Usage Example

```go
import "github.com/bingzujia/google-takeout-time-helper/internal/heicconv"

stats, err := heicconv.Convert(heicconv.Config{
    InputDir: "~/Photos",
    Quality:  75,
}, logger)
```

### Design Notes

- HEIC encodes to Main Still Picture profile only (4:2:0 chroma mandatory)
- Never use `exiftool -all:all` on HEIC (copies JPEG-specific IFD1 markers)
- Requires `heif-enc` binary for encoding; gracefully degraded without it

---

## exifrunner

**Location:** `internal/exifrunner/`

**Role:** Batch exiftool operations (query and write) with process pooling to avoid per-file subprocess spawning.

**Key Responsibility:** Efficiently execute exiftool commands in batches (up to 1000 files/subprocess) to reduce process overhead. Gracefully degrade when exiftool is absent.

### Main API

```go
// LookupPath checks if exiftool is available in PATH
func LookupPath() (string, error)

// BatchQuery reads tags from multiple files in one subprocess
func BatchQuery(files []string, tags []string) (map[string]map[string]string, error)

// BatchWrite writes tags to multiple files in one subprocess
func BatchWrite(updates map[string]map[string]string) error

// SingleWrite writes tags to one file (convenience wrapper)
func SingleWrite(path string, tags map[string]string) error
```

### Batching Strategy

- Files grouped into batches of ≤ 1000 files/subprocess
- One subprocess per batch (avoid long command lines)
- Reduces subprocess spawning overhead vs. per-file invocation

### Example: Batch Write

```go
updates := map[string]map[string]string{
    "photo1.jpg": {
        "DateTimeOriginal": "2023:01:15 14:30:45",
        "CreateDate":       "2023:01:15 14:30:45",
    },
    "photo2.jpg": {
        "DateTimeOriginal": "2023:01-16 10:20:30",
        "CreateDate":       "2023:01:16 10:20:30",
    },
}
exifrunner.BatchWrite(updates)
```

### Graceful Degradation

- If exiftool not found: Return error upfront via `LookupPath()`
- Commands check availability before invoking
- Metadata writes skipped gracefully when exiftool absent

### Extension Points

- Adjust batch size (currently 1000) via package constant
- Customize tag formatting (currently uses exiftool native tags)

### Usage Pattern

```go
import "github.com/bingzujia/google-takeout-time-helper/internal/exifrunner"

// Check availability
path, err := exifrunner.LookupPath()
if err != nil {
    // exiftool not available; degrade gracefully
}

// Batch write metadata
exifrunner.BatchWrite(updates)
```

---

## workerpool

**Location:** `internal/workerpool/`

**Role:** Generic goroutine pool for parallel task execution with configurable worker count.

**Key Responsibility:** Safely distribute jobs across workers, handle backpressure, and limit concurrency to prevent OOM.

### Main API

```go
// Run processes jobs with a bounded pool of workers
// T = job type, R = result type
func Run[T, R any](jobs []T, workers int, fn func(T) (R, error)) ([]R, error)

// MaxWorkers returns capped worker count (min(NumCPU, 8))
func MaxWorkers(requested int) int
```

### Default Capping

- **Default workers:** `min(NumCPU, 8)`
- **Reason:** Prevent resource exhaustion on high-core systems
- **Override:** User can specify via command flags

### Example: Parallel File Processing

```go
import "github.com/bingzujia/google-takeout-time-helper/internal/workerpool"

results, err := workerpool.Run(
    files,  // []string of file paths
    4,      // 4 workers
    func(path string) (Result, error) {
        // Process one file
        return processFile(path)
    },
)
```

### Extension Points

- Adjust CPU capping formula (currently `min(NumCPU, 8)`)
- Add context support for cancellation
- Add progress callback for long-running jobs

---

## logutil

**Location:** `internal/logutil/`

**Role:** Thread-safe file logging with standard log entries (INFO, SKIP, FAIL) and counters.

**Key Responsibility:** Log per-file decisions to `takeout-helper-log/{command}-{date}-{index}.log` with thread safety.

### Main API

```go
type Logger interface {
    Info(format string, args ...interface{})
    Skip(format string, args ...interface{})
    Fail(format string, args ...interface{})
    Close() error
}

// NewFileLogger creates a logger that writes to file
func NewFileLogger(logDir string, prefix string) (Logger, error)

// Nop returns a no-op logger (used in --dry-run mode)
func Nop() Logger
```

### Log Entry Format

```
[INFO] 2023-01-15 14:30:45 Processed file IMG_0030.JPG
[SKIP] 2023-01-15 14:30:46 No JSON sidecar for IMG_0031.JPG
[FAIL] 2023-01-15 14:30:47 Error writing EXIF to IMG_0032.JPG: permission denied
```

### Log Location

- **Path:** `<working-dir>/takeout-helper-log/`
- **Filename:** `{command}-YYYYMMDD-NNN.log` (e.g., `migrate-20230115-001.log`)
- **Rollover:** Creates new file with incremented index if file already exists

### Dry-Run Mode

- When `--dry-run` flag set: Use `logutil.Nop()` to suppress file writes
- Ensures no side effects in dry-run mode

### Thread Safety

- All log writes are protected by mutex
- Safe for concurrent calls from worker goroutines

### Extension Points

- Add new entry types (currently INFO/SKIP/FAIL)
- Customize log format (currently: `[LEVEL] timestamp message`)

### Usage Example

```go
import "github.com/bingzujia/google-takeout-time-helper/internal/logutil"

logger, err := logutil.NewFileLogger("takeout-helper-log", "migrate")
defer logger.Close()

logger.Info("Processing file %s", filename)
logger.Skip("No JSON sidecar found")
logger.Fail("Error: %v", err)
```

---

## organizer

**Location:** `internal/organizer/`

**Role:** Classify input directories into year folders based on Google Takeout pattern (`Photos from XXXX`).

**Key Responsibility:** Identify which input directories correspond to which years for filtering in `migrate --year`.

### Main API

```go
// Organize scans input directory for year folders
func Organize(inputDir string) (map[int]string, error)
// Returns: year → directory path mapping (e.g., 2023 → "Photos from 2023")
```

### Pattern

Matches directories named:
- `Photos from 2023`
- `Photos from 2022`
- etc.

### Extension Points

- Add support for other folder naming schemes
- Customize year extraction logic

---

## classifier

**Location:** `internal/classifier/`

**Role:** Classify media files into buckets (camera, screenshot, WeChat, seemsCamera) based on filename and EXIF rules.

**Key Responsibility:** Apply classification rules to organize mixed media sources.

### Main API

```go
type Classifier struct {
    // Configuration for bucket rules
}

type Stats struct {
    Camera       int  // Matched camera filename pattern
    Screenshot   int  // Contains "screenshot" in filename
    WeChat       int  // Filename starts with "mmexport"
    SeemsCamera  int  // EXIF Make/Model detected (no filename match)
    Skipped      int  // No match
}

// Classify processes all files in directory
func Classify(config Config, logger Logger) (*Stats, error)
```

### Classification Rules

| Bucket | Rule | Pattern Examples |
|--------|------|------------------|
| **camera** | Filename matches camera pattern | `IMG_`, `VID_`, `PXL_`, `DJI_`, etc. |
| **screenshot** | Filename contains "screenshot" | `screenshot_*.png`, `Screenshot_*.png` |
| **wechat** | Filename starts with "mmexport" | `mmexport1234567890.jpg` |
| **seemsCamera** | exiftool detects Make/Model | EXIF `Make=Canon, Model=EOS 5D` |
| **skipped** | No rule matches | Left in place |

### Extension Points

- Add new bucket by extending classification rules
- Customize filename patterns
- Override EXIF Make/Model detection

### Usage Example

```go
import "github.com/bingzujia/google-takeout-time-helper/internal/classifier"

stats, err := classifier.Classify(config, logger)
```

---

## mediatype

**Location:** `internal/mediatype/`

**Role:** Detect true media type from file content (not extension) using file magic bytes.

**Key Responsibility:** Correct mismatched file extensions (e.g., JPEG named .png).

### Main API

```go
// DetectMediaType reads file header and returns actual type
func DetectMediaType(filePath string) (string, error)
// Returns: "image/jpeg", "image/png", "video/mp4", etc.

// ToExtension maps media type to canonical extension
func ToExtension(mediaType string) string
// Returns: ".jpg", ".png", ".mp4", etc.
```

### Detection Method

- Reads first few bytes of file (magic number)
- Compares against known signatures
- Returns canonical MIME type

### Supported Formats

- Images: JPEG, PNG, BMP, GIF, TIFF, WebP, HEIC, HEIF
- Videos: MP4, MOV, AVI, MKV, WebM

### Extension Points

- Add new file format by extending magic number table
- Customize extension mapping

### Usage Example

```go
import "github.com/bingzujia/google-takeout-time-helper/internal/mediatype"

mediaType, err := mediatype.DetectMediaType("image.jpg")
// Catches cases like "image.jpg" that's actually a PNG
```

---

## dedup

**Location:** `internal/dedup/`

**Role:** Perceptual deduplication using pHash and dHash to find similar images.

**Key Responsibility:** Group visually similar images regardless of exact pixel differences (compression, slight crops, filters).

### Main API

```go
type Config struct {
    InputDir      string
    CacheDir      string  // SQLite cache location
    Threshold     int     // Hash distance tolerance (default: 10)
    ConvertHEIC   bool    // Convert HEIC to JPEG for hashing
    DecodeWorkers int     // Parallel decode workers
    NoCache       bool    // Skip hash caching
}

type Stats struct {
    Processed      int
    Grouped        int  // Number of duplicate groups found
    SkippedFormat  int
    SkippedOversized int
}

// ProcessDirectory finds and groups duplicates
func ProcessDirectory(config Config) (*Stats, error)
```

### Hashing Algorithms

- **pHash (Perceptual Hash):** Hash based on image content (robust to compression)
- **dHash (Difference Hash):** Hash based on pixel gradients
- **Dual check:** Uses both for higher accuracy

### Output Structure

```
input_dir/
├── dedup/
│   ├── group-001/
│   │   ├── image1.jpg
│   │   └── image2.jpg
│   └── group-002/
│       └── similar_image.png
├── file1.jpg   (kept in root, not duplicate)
└── file2.png
```

### Hash Caching

- SQLite database stores computed hashes
- File path + modification time key
- Speeds up re-runs after filtering
- Disable with `--no-cache` for one-off runs

### Threshold Tuning

- **Lower (5–8):** Stricter; fewer false positives (real duplicates only)
- **Default (10):** Balanced
- **Higher (15–20):** More permissive; may include similar-looking images

### Extension Points

- Adjust hash distance threshold algorithm
- Add new hashing methods (e.g., deep learning embeddings)
- Customize duplicate group naming

### Usage Example

```go
import "github.com/bingzujia/google-takeout-time-helper/internal/dedup"

stats, err := dedup.ProcessDirectory(dedup.Config{
    InputDir:   "~/Photos",
    Threshold:  10,
    ConvertHEIC: true,
})
```

---

## hashcache

**Location:** `internal/hashcache/`

**Role:** SQLite-backed cache for perceptual hash values (pHash/dHash).

**Key Responsibility:** Persist hash computations across runs to speed up re-processing.

### Main API

```go
type Cache interface {
    // Get retrieves cached hash for file
    Get(filePath string) (pHash, dHash string, found bool)
    
    // Set stores hash for file
    Set(filePath string, pHash, dHash string) error
    
    // Close closes database connection
    Close() error
}

// New creates or opens cache database
func New(cacheDir string) (Cache, error)
```

### Storage

- **Location:** `.gtoh_cache/hashes.db` (SQLite)
- **Schema:** `(file_path TEXT PRIMARY KEY, phash TEXT, dhash TEXT, mtime INTEGER)`
- **Key:** File path + modification time

### Extension Points

- Add new hash types (currently: pHash, dHash)
- Customize cache eviction policy
- Add cache statistics/diagnostics

---

## destlocker

**Location:** `internal/destlocker/`

**Role:** Prevent concurrent writes to the same destination file path across goroutines.

**Key Responsibility:** Ensure atomic file operations when multiple workers run in parallel.

### Main API

```go
type Locker interface {
    // Lock acquires exclusive lock for path
    Lock(path string)
    
    // Unlock releases lock
    Unlock(path string)
}

// New creates a new locker
func New() Locker
```

### Usage Pattern

```go
locker := destlocker.New()

go func(filePath string) {
    locker.Lock(filePath)
    defer locker.Unlock(filePath)
    
    // Safe to write to filePath
    writeFile(filePath)
}(path)
```

### Extension Points

- Add timeout support
- Add lock statistics

---

## renamer

**Location:** `internal/renamer/`

**Role:** Generate standardized filenames based on timestamp and media type.

**Key Responsibility:** Create consistent naming convention (IMG_YYYYMMDD_HHMMSS, etc.) with burst detection.

### Main API

```go
type Renamer struct {
    // Configuration for naming patterns
}

type Stats struct {
    Processed    int
    Skipped      int
    Failed       int
    BurstDetected int
}

// GenerateName creates standardized filename
func GenerateName(originalPath string, timestamp time.Time) (string, error)

// ProcessDirectory renames all files in directory
func ProcessDirectory(config Config) (*Stats, error)
```

### Naming Patterns

| Type | Pattern | Example |
|------|---------|---------|
| HEIC Image | `IMG{YYYYMMDD}{HHMMSS}.heic` | `IMG20230115143045.heic` |
| Other Image | `IMG_{YYYYMMDD}_{HHMMSS}.ext` | `IMG_20230115_143045.jpg` |
| Video | `VID{YYYYMMDD}{HHMMSS}.ext` | `VID20230115143045.mp4` |
| Burst (HEIC) | `IMG{YYYYMMDD}{HHMMSS}_BURST{NNN}.heic` | `IMG20230115143045_BURST001.heic` |
| Burst (Other) | `IMG_{YYYYMMDD}_{HHMMSS}_BURST{NNN}.ext` | `IMG_20230115_143045_BURST001.jpg` |
| Screenshot | `Screenshot_{YYYY-MM-DD-HH-MM-SS-MS}.ext` | `Screenshot_2023-01-15-14-30-45-000.png` |

### Burst Detection

Triggered when:
- ≥ 2 files share same `YYYYMMDD_HHMMSS` prefix
- Files are named with pattern `YYYYMMDD_HHMMSS_NNN`

### Conflict Resolution

If target filename exists, append `_001`, `_002`, etc.

### MP4 Companions

When renaming image, also rename same-name `.mp4` file with image name.

### Extension Points

- Add new naming patterns
- Customize burst detection logic
- Override conflict resolution strategy

---

## progress

**Location:** `internal/progress/`

**Role:** Terminal progress bar for long-running operations.

**Key Responsibility:** Provide user feedback on operation progress without cluttering output.

### Main API

```go
type Bar interface {
    // Add increments progress
    Add(delta int)
    
    // Finish completes the progress bar
    Finish()
}

// New creates a new progress bar
func New(total int, description string) Bar
```

### Usage Example

```go
import "github.com/bingzujia/google-takeout-time-helper/internal/progress"

bar := progress.New(numFiles, "Converting images")
for _, file := range files {
    processFile(file)
    bar.Add(1)
}
bar.Finish()
```

---

## fileutil

**Location:** `internal/fileutil/`

**Role:** Common file operation helpers.

**Key Responsibility:** Safe file copy, move, delete, and extension handling.

### Main API

```go
// CopyFile safely copies file with error checking
func CopyFile(src, dst string) error

// SafeRemove removes file if exists (no error if missing)
func SafeRemove(path string) error

// CorrectExtension renames file to match actual content type
func CorrectExtension(path string) error

// GetExtension returns file extension
func GetExtension(path string) string
```

---

## qqmedia

**Location:** `internal/qqmedia/`

**Role:** Format QQ-exported media files with standardized naming.

**Key Responsibility:** Extract timestamp from various QQ filename patterns and rename consistently.

### Supported Patterns

1. `_YYYYMMDD_HHMMSS` — e.g., `photo_20170709_002844.jpg`
2. 13-digit Unix milliseconds — e.g., `1688017744459`
3. `QQ视频YYYYMMDDHHMMSS` — QQ video format
4. `Record_YYYY-MM-DD-HH-MM-SS` — Recording timestamp
5. `Snipaste_YYYY-MM-DD_HH-MM-SS` — Snipaste screenshot
6. `tb_image_share_13digits` — Taobao image share
7. `TIM图片YYYYMMDDHHMMSS` — TIM picture
8. File modification time (fallback)

### Output Format

- Images: `Image_<unix-ms>.ext`
- Videos: `Video_<unix-ms>.ext`

---

## screenshotter

**Location:** `internal/screenshotter/`

**Role:** Detect screenshot files by filename pattern.

**Key Responsibility:** Identify screenshots for special handling (naming, classification).

### Main API

```go
// IsScreenshot detects if filename indicates screenshot
func IsScreenshot(filename string) bool
```

### Detection Criteria

- Filename contains "screenshot" (case-insensitive)
- Common screenshot app patterns: Snipaste, QQ截图, etc.

### Extension Points

- Add new screenshot app pattern detection
- Customize confidence thresholds

---

## Integration Patterns

### Command → Package Delegation

Each CLI command delegates to a corresponding package:

```
cmd/takeout-helper/cmd/migrate.go
    ↓ (delegates)
internal/migrator/
    ↓ (uses)
internal/matcher/       (find JSON sidecars)
internal/parser/        (parse timestamps)
internal/exifrunner/    (write EXIF metadata)
internal/logutil/       (log results)
```

### Metadata Flow

```
JSON Sidecar (Google Takeout)
    ↓
matcher.JSONForFile     (locate sidecar)
    ↓
parser.ParseTime        (extract timestamp)
parser.ParseGPS         (extract GPS)
    ↓
exifrunner.BatchWrite   (write to EXIF tags)
    ↓
Output file with metadata
```

### Error Handling

All packages follow consistent error pattern:
- Return nil/error pairs
- Package-level error types for domain-specific errors
- Logger for contextual error logging

### Testing Patterns

- Internal packages tested independently with unit tests
- Use testdata/ for test fixtures
- Mock exifrunner when needed to avoid subprocess dependency

---

## Development Guidelines

- **Backward compatibility:** Breaking API changes require major version bump
- **Concurrency:** Use workerpool for parallel operations; protect shared state with mutexes
- **Logging:** Use logutil.Logger for all logging; avoid fmt.Print in packages
- **Error messages:** Include actionable error messages (what went wrong, how to fix)
- **Documentation:** Keep godocs up-to-date; reference architecture docs for context
