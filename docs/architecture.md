# Architecture Overview

This document provides a high-level system design for `takeout-helper`, explaining command delegation patterns, data flows, and extension points for developers.

## System Design

`takeout-helper` is a Cobra-based CLI with a clear separation of concerns:

```
┌─────────────────────────────────────────────────────────────┐
│                      CLI Layer (Cobra)                       │
│        cmd/takeout-helper/cmd/{command}.go                  │
│  migrate  classify  convert  fix-exif  fix-name  dedup...   │
└──────────────────────┬──────────────────────────────────────┘
                       │ delegates to
                       ↓
┌─────────────────────────────────────────────────────────────┐
│               Internal Package Layer                         │
│  migrator  classifier  heicconv  renamer  dedup  etc.      │
│  (business logic, independently testable)                   │
└──────────────────────┬──────────────────────────────────────┘
                       │ orchestrate
                       ↓
┌─────────────────────────────────────────────────────────────┐
│              Utility / Infrastructure Packages               │
│  matcher  parser  exifrunner  logutil  workerpool  etc.    │
│  (reusable components, cross-command)                       │
└─────────────────────────────────────────────────────────────┘
```

## Command Delegation Pattern

Each command follows the same structure:

```go
// cmd/takeout-helper/cmd/migrate.go
func init() {
    rootCmd.AddCommand(migrateCmd)  // Self-register via init()
}

var migrateCmd = &cobra.Command{
    Use:   "migrate",
    Short: "Migrate photos...",
    RunE: func(cmd *cobra.Command, args []string) error {
        // 1. Parse flags from command line
        inputDir, _ := cmd.Flags().GetString("input-dir")
        outputDir, _ := cmd.Flags().GetString("output-dir")
        
        // 2. Delegate to internal package
        stats, err := migrator.Migrate(migrator.Config{
            InputDir:  inputDir,
            OutputDir: outputDir,
        }, logger)
        
        // 3. Print summary to stdout
        fmt.Printf("Processed: %d, Failed: %d\n", stats.Processed, stats.Failed)
        
        return err
    },
}
```

**Pattern:**
1. Parse CLI flags
2. Create Config struct
3. Delegate to internal package function
4. Return Stats struct for summary printing
5. All commands support `--dry-run` (no side effects)

---

## Data Flows

### migrate Command Flow

```
User Input
    │
    ├─ --input-dir:  Google Takeout export root
    ├─ --output-dir: Destination
    └─ --year:       (optional) filter to specific year
         ↓
    1. Scan year folders (Photos from XXXX)
         ↓
    2. For each file in year folder:
         ├─ matcher.JSONForFile()    ← locate JSON sidecar
         │                             (6-step degradation)
         ├─ parser.ParseTime()        ← parse JSON timestamp
         ├─ parser.ParseGPS()         ← parse JSON GPS
         ├─ fileutil.CopyFile()       ← copy to output
         └─ exifrunner.BatchWrite()   ← write EXIF metadata
              │                          via exiftool
              └─ logutil.Logger.Info()  ← log result
         ↓
    3. Return Stats (Processed, Skipped, Failed)
         ↓
    Output
    ├─ ~/output/IMG_0030.JPG      (with EXIF metadata)
    ├─ ~/output/.metadata.json    (audit trail)
    └─ takeout-helper-log/        (log file)
```

### convert Command Flow

```
User Input
    ├─ --input-dir:  source images
    ├─ --quality:    HEIC encoding quality (default 75)
    └─ --workers:    parallel workers
         ↓
    1. Scan input directory
         ↓
    2. For each image (parallel, up to N workers):
         ├─ mediatype.DetectMediaType()     ← check actual format
         ├─ heicconv.decodeSourceImage()    ← decode + dimension check
         │   └─ Check: dimension ≤ 16383 px
         ├─ heicconv.detectChromaSubsampling() ← always 4:2:0
         ├─ heif-enc subprocess             ← encode to HEIC
         ├─ exifrunner.SingleWrite()        ← copy EXIF/XMP/ICC
         ├─ fileutil.SafeRemove()           ← delete original
         └─ logutil.Logger.Info()           ← log result
         ↓
    3. Return Stats (Processed, SkippedOversizeDim, Failed)
         ↓
    Output
    ├─ ~/input/image.heic     (converted, with metadata)
    └─ takeout-helper-log/    (log file with per-file decisions)
```

### dedup Command Flow

```
User Input
    ├─ --input-dir:     source images
    ├─ --threshold:     perceptual hash distance (default 10)
    ├─ --cache-dir:     hash cache location
    └─ --convert-heic:  convert HEIC to JPEG for hashing
         ↓
    1. Scan input directory (parallel decode)
         ↓
    2. For each image:
         ├─ hashcache.Get(path)           ← check cache
         ├─ If not cached:
         │  ├─ mediatype.DetectMediaType() ← get actual format
         │  ├─ heicconv.Convert()         ← convert HEIC→JPEG
         │  ├─ Compute pHash               ← perceptual hash
         │  ├─ Compute dHash               ← difference hash
         │  └─ hashcache.Set(path, h1, h2) ← cache result
         └─ Collect (path, pHash, dHash)
         ↓
    3. Group by similarity (threshold-based distance)
         ↓
    4. For each group (parallel):
         ├─ destlocker.Lock(output_path)  ← prevent race
         ├─ mkdir dedup/group-001/
         ├─ fileutil.CopyFile() or Move()
         └─ destlocker.Unlock()
         ↓
    5. Return Stats (Processed, Grouped)
         ↓
    Output
    dedup/
    ├─ group-001/image1.jpg
    ├─ group-001/image2.jpg
    └─ group-002/similar.png
```

### Timestamp Resolution Priority

Used by `matcher.ResolveTimestamp()`:

```
Priority Order:
    1. EXIF DateTimeOriginal (most reliable)
         ↓ if missing
    2. Filename embedded timestamp (IMG_YYYYMMDD_HHMMSS)
         ↓ if missing
    3. JSON sidecar photoTakenTime (from Google Takeout)
         ↓ if all missing
    NULL (timestamp unknown)
```

### GPS Resolution Priority

Used by `matcher.ResolveGPS()`:

```
Priority Order:
    1. EXIF GPS tags (most reliable)
         ↓ if missing
    2. JSON sidecar geoData (from Google Takeout)
         ↓ if all missing
    NULL (location unknown)
```

---

## Extension Points

### Adding a New Command

**1. Create command file:** `cmd/takeout-helper/cmd/mynewcmd.go`

```go
package cmd

import "github.com/spf13/cobra"

func init() {
    rootCmd.AddCommand(mynewCmd)  // Self-register
}

var mynewCmd = &cobra.Command{
    Use:   "mynew",
    Short: "Description of mynew command",
    RunE: func(cmd *cobra.Command, args []string) error {
        // 1. Parse flags
        inputDir, _ := cmd.Flags().GetString("input-dir")
        
        // 2. Create config
        config := mypkg.Config{InputDir: inputDir}
        
        // 3. Call internal package
        stats, err := mypkg.ProcessDirectory(config, logger)
        
        // 4. Print summary
        fmt.Printf("Processed: %d\n", stats.Processed)
        
        return err
    },
}

func init() {
    mynewCmd.Flags().StringP("input-dir", "i", "", "Input directory")
    mynewCmd.Flags().Bool("dry-run", false, "Preview only")
    // ... other flags
}
```

**2. Create internal package:** `internal/mypkg/mypkg.go`

```go
package mypkg

type Config struct {
    InputDir string
    Dry      bool
    // ... other config
}

type Stats struct {
    Processed int
    Skipped   int
    Failed    int
}

func ProcessDirectory(config Config, logger logutil.Logger) (*Stats, error) {
    // Core logic here
    return &Stats{}, nil
}
```

**3. Follow conventions:**
- Use `--dry-run` flag for safety preview
- Return `Stats` struct for summary
- Use `logutil.Logger` for logging
- Support `--input-dir`, `--output-dir` for paths
- Add tests for internal package logic

### Extending an Existing Package

**Example: Add new timestamp format to parser**

1. Identify the package: `internal/parser/`
2. Add new format constant and pattern
3. Update `ParseTime()` function to recognize it
4. Add test case
5. Update docs/modules.md if user-facing

```go
// internal/parser/parser.go
const newPattern = "YYYY-MM-DD@HH:MM:SS"

func ParseTime(s string) (*time.Time, error) {
    // ... existing code ...
    
    // Add new format
    if t, err := time.Parse("2006-01-02@15:04:05", s); err == nil {
        return &t, nil
    }
    
    // ... fallback to other formats ...
}
```

**Example: Add new classification bucket to classifier**

1. Open `internal/classifier/`
2. Add new rule function
3. Call from `Classify()` dispatcher
4. Add new Stats counter
5. Update docs/modules.md

```go
// internal/classifier/classifier.go
func matchesNewBucket(filename string) bool {
    return strings.HasPrefix(filename, "special_prefix_")
}

func Classify(config Config, logger Logger) (*Stats, error) {
    // ... existing code ...
    
    for _, file := range files {
        if matchesNewBucket(file) {
            // Move to newbucket/ subdirectory
            stats.NewBucket++
        }
        // ... other classification rules ...
    }
}
```

### Customizing Configuration

Each command's Config struct allows customization:

```go
// Example: customize migration behavior
config := migrator.Config{
    InputDir:  "~/Takeout",
    OutputDir: "~/Photos",
    Year:      "2023",              // Filter to specific year
    Dry:       true,                 // Preview only
    // ... other options
}
```

Developers can:
- Add new Config fields for new options
- Add new flags to CLI to expose Config options
- Update internal package logic to respect Config

---

## Conventions

### Command-Line Interface

**Standard flags (all commands):**
- `--input-dir string` — Input directory (required for most commands)
- `--output-dir string` — Output directory (required for some commands)
- `--dry-run` — Preview mode, no side effects

**Logging:**
- All commands create logs in `takeout-helper-log/` directory
- Filename pattern: `{command}-YYYYMMDD-NNN.log`
- Dry-run mode uses `logutil.Nop()` to suppress file writes

### Error Handling

**Pattern:**
1. Validate input upfront (flags, directories)
2. Return package-specific errors with context
3. Log errors via logutil.Logger
4. Include actionable error message (what went wrong, how to fix)
5. Continue processing on non-fatal errors (count as Failed/Skipped)

**Example:**
```go
if !fileExists(inputDir) {
    return fmt.Errorf("input directory not found: %s", inputDir)
}

// On per-file error:
logger.Fail("Error processing %s: %v (skipping)", filePath, err)
stats.Failed++
```

### Logging

**Use logutil.Logger, never fmt.Print in packages:**

```go
logger.Info("Starting migration from %s", inputDir)
logger.Skip("File %s has no JSON sidecar", filePath)
logger.Fail("Error writing EXIF to %s: %v", filePath, err)
```

**Log entry types:**
- `Info` — Successful operation
- `Skip` — File skipped (no JSON sidecar, etc.)
- `Fail` — Error during processing

### Concurrency

**Use workerpool for parallel operations:**

```go
import "github.com/bingzujia/google-takeout-time-helper/internal/workerpool"

results, err := workerpool.Run(
    files,  // []string of work items
    4,      // desired workers
    func(file string) (Result, error) {
        return processFile(file)
    },
)
```

**Worker count:** Capped at `min(NumCPU, 8)` by default to prevent resource exhaustion

**Shared state:** Protect with mutexes or use `destlocker.Locker` for file paths

### Testing

**Patterns:**
- Test internal package logic independently (not CLI)
- Use `testdata/` directory for test fixtures
- Mock `exifrunner` when avoiding subprocess dependency
- Test both success and error paths

**Example:**
```go
func TestMigrate(t *testing.T) {
    config := migrator.Config{
        InputDir:  "testdata/input",
        OutputDir: "testdata/output",
        Dry:       true,
    }
    stats, err := migrator.Migrate(config, logutil.Nop())
    
    if stats.Processed != 5 {
        t.Errorf("Expected 5 processed, got %d", stats.Processed)
    }
}
```

### Metadata Handling

**EXIF tag groups (safe for all formats):**
- `EXIF:all` — EXIF metadata (camera, lens, timestamp)
- `XMP:all` — XMP metadata (keywords, creator)
- `ICC_Profile:all` — Color profile

**NEVER use:**
- `-all:all` — Copies format-specific tags (JPEG IFD1 thumbnail offsets → breaks HEIC)

**Timestamp tag priority when writing:**
```
DateTimeOriginal    (most important, used by most software)
CreateDate          (fallback, used by some software)
ModifyDate          (when known source time of modification)
```

### Optional Dependencies

**exiftool:**
- Gracefully degrade when absent
- Check availability: `exifrunner.LookupPath()`
- Commands succeed without exiftool (but skip metadata writes)
- Document requirement in command help text

**Example:**
```go
path, err := exifrunner.LookupPath()
if err != nil {
    logger.Skip("exiftool not found, skipping EXIF writes")
    // Continue processing, just without EXIF
}
```

---

## Key Design Decisions

### 1. Matcher: 6-Step Degradation Strategy

**Problem:** Google Takeout truncates JSON sidecar names at 51 characters, uses locale-specific suffixes, and rearranges bracket-numbers.

**Solution:** Multi-step matching strategy:
- Identity → ShortenName → BracketSwap → RemoveExtra → Supplemental → NoExtension
- Handles 99% of Google Takeout quirks without regex complexity

**Why not grepping:** Would require expensive file system search; degradation strategy uses deterministic transformations.

### 2. HEIC: Always 4:2:0 Chroma

**Problem:** Some cameras/phones produce 4:4:4 or 4:2:2 chroma. Passing to x265 causes it to select HEVC Range Extensions profile (profile_idc=4), which HEIC container brand rejects.

**Solution:** Always output 4:2:0 chroma regardless of source, forcing profile_idc=3 (Main Still Picture), ensuring universal decoder support.

**Trade-off:** Tiny quality loss on 4:4:4 sources, but guaranteed compatibility.

### 3. Perceptual Hashing (dedup)

**Problem:** Exact pixel comparison misses similar images (compression, filters, slight crops).

**Solution:** pHash + dHash dual check captures both content and structure similarity.

**Alternative considered:** Deep learning embeddings — rejected due to dependency weight and GPU requirement.

### 4. Batch exiftool Calls

**Problem:** Per-file subprocess invocation causes overhead (fork + exec for each file).

**Solution:** Batch up to 1000 files per subprocess, reduces process spawn overhead significantly.

**Trade-off:** Slightly more complex error handling (per-batch vs. per-file).

### 5. Dimension Limit (16383 px)

**Hard limit set by Apple ImageIO:** HEIC decoders cap at 16383 px per dimension.

**Solution:** Detect oversized images before encoding, skip with clear warning.

**Why not downscale:** Would change image content; requires explicit user consent.

---

## Performance Characteristics

### Memory Usage

- **Parallel workers:** Capped at `min(NumCPU, 8)` to prevent OOM
- **Image decoding:** Can be memory-intensive; `dedup` has `--max-decode-mb` flag to skip oversized images
- **Hash caching:** SQLite DB grows ~1KB per image; negligible for typical collections

### Disk I/O

- **Batch processing:** Reduces process overhead, improves throughput
- **Copy operations:** Avoid re-reading source; use efficient copy routines
- **Logging:** Asynchronous writes (mutex-protected, buffered)

### Network

- No network operations; all processing is local

---

## Graceful Degradation

Commands continue operating even when optional dependencies are missing:

| Dependency | Missing Behavior |
|------------|------------------|
| **exiftool** | Skip EXIF metadata writes; files still copied |
| **heif-enc** | Skip HEIC conversion; other formats proceed |
| **heif decoder** | Skip HEIC input; process other formats |

This design prioritizes **data integrity** over **metadata completeness** — a file without metadata is better than a file not processed at all.

---

## Development Workflow

### Building

```bash
go build -o bin/takeout-helper ./cmd/takeout-helper
```

### Running Tests

```bash
go test ./...              # All tests
go test ./internal/matcher/...  # Single package
go test -run TestName ./...     # Single test
```

### Linting

```bash
go vet ./...
```

### Common Development Tasks

**Add new command:**
1. Create `cmd/takeout-helper/cmd/mynew.go` with init() self-registration
2. Create `internal/mypkg/` package with core logic
3. Add tests in `internal/mypkg/*_test.go`
4. Update `docs/commands.md` and `docs/modules.md`

**Fix bug in package:**
1. Write failing test
2. Fix implementation
3. Verify test passes
4. Run full test suite to check for regressions

**Performance improvement:**
1. Profile with `pprof` if applicable
2. Implement change
3. Benchmark before/after
4. Document trade-offs in code comments

---

## Future Considerations

### Possible Extensions

- **RAW support:** Add raw photo format (Canon CR3, Nikon NEF) handling
- **Video processing:** Extract/set video metadata (duration, resolution)
- **AI tagging:** Use machine learning for automatic tag suggestions
- **Cloud sync:** Optional export to cloud storage (Google Photos, OneDrive)

### Potential Performance Improvements

- **GPU acceleration:** Use GPU for hash computation (CUDA/Metal)
- **Incremental processing:** Track which files changed since last run
- **Multi-file reading:** Use io.Reader pooling for concurrent I/O

### Architectural Considerations

- Keep internal packages independent and testable
- Maintain clear separation between CLI and business logic
- Support programmatic use (as a library) via stable internal APIs
- Document breaking changes in CHANGELOG for each release
