# Architecture Overview

`takeout-helper` is a Cobra-based CLI tool for organizing Google Takeout photos. This document explains the architecture, data flows, and key components.

---

## System Design

```
┌─────────────────────────────────────────────────────┐
│               CLI Layer (Cobra)                     │
│        cmd/takeout-helper/cmd/migrate.go            │
│              (command parsing)                      │
└──────────────────────┬────────────────────────────┘
                       │ delegates to
                       ↓
┌─────────────────────────────────────────────────────┐
│           Core Logic: migrator Package              │
│  - Scans year folders from Takeout export           │
│  - Matches JSON sidecars to photos                  │
│  - Copies files to output directory                 │
│  - Sets file timestamps via os.Chtimes()           │
│  - Writes metadata JSON files                       │
└──────────────────────┬────────────────────────────┘
                       │ uses
                       ↓
┌─────────────────────────────────────────────────────┐
│        Supporting Packages (cross-cutting)          │
│  matcher       - JSON sidecar matching              │
│  parser        - Timestamp/GPS parsing              │
│  organizer     - Photo classification               │
│  logutil       - Logging                            │
│  workerpool    - Goroutine pool                     │
│  mediatype     - File type detection                │
│  progress      - Terminal progress bar              │
└─────────────────────────────────────────────────────┘
```

---

## Command Structure

Entry point: `cmd/takeout-helper/cmd/migrate.go`

```go
var migrateCmd = &cobra.Command{
    Use:   "migrate",
    Short: "Migrate Google Takeout photos with file timestamps",
    RunE: runMigrate,
}

func runMigrate(cmd *cobra.Command, _ []string) error {
    // Parse flags
    inputDir := migrateInputDir
    outputDir := migrateOutputDir
    
    // Delegate to migrator package
    stats, err := migrator.Run(migrator.Config{
        InputDir:     inputDir,
        OutputDir:    outputDir,
        DryRun:       migrateDryRun,
        ShowProgress: !migrateDryRun,
        Logger:       logger,
    })
    
    // Print summary
    fmt.Printf("Processed: %d, Skipped: %d, Manual Review: %d\n", 
        stats.Processed, stats.SkippedExists, stats.ManualReview)
    
    return err
}
```

---

## Data Flow: Photo Migration

```
Input: Google Takeout Export
│
├─ Photos from 2015/
│  ├─ IMG_1234.jpg ──┐
│  └─ IMG_1234.json ─┤
├─ Photos from 2016/ │
│  └─ ...            │
│                    ↓
          ┌──────────────────────┐
          │  migrator.Run()      │
          │                      │
          │ 1. Scan year folders │
          │ 2. Find JSON pairs   │
          │ 3. Copy files        │
          │ 4. Set timestamps    │
          │ 5. Write metadata    │
          └──────────────────────┘
                    │
                    ↓
         Output Directory
         │
         ├─ camera-model-1/
         │  ├─ metadata/
         │  │  └─ IMG_1234.json
         │  ├─ IMG_1234.jpg
         │  └─ ...
         │
         └─ takeout-helper-log/
            └─ migrate-20260421-001.log
```

---

## Key Packages

### migrator
**Core logic.** Orchestrates the entire migration process:
- Scans input directory for year folders
- Matches JSON sidecars to photo files
- Copies files with proper error handling
- Sets file modification times from JSON timestamps
- Writes metadata JSON for each photo
- Logs per-file decisions

**Main function:** `migrator.Run(config) → Stats`

**Key concepts:**
- **Timestamp Priority:** photoTakenTime → creationTime → manual_review
- **File Timestamps:** Set via `os.Chtimes()` (no EXIF modification)
- **Metadata Tracking:** Each photo gets a SHA-256 metadata JSON with source information

### matcher
Locates the JSON sidecar for a photo using a 6-step degradation strategy:
1. Identity matching (exact name)
2. Name shortening (Google Takeout truncates at 51 chars)
3. Bracket swapping (handles `IMG_1234 (1).jpg` → `IMG_1234.jpg`)
4. Extra removal (removes extraneous characters)
5. Supplemental matching (for `supplemental-metadata.json` files)
6. No extension matching (handles case where extension is missing)

### parser
Parses timestamps from EXIF and filenames; parses GPS from JSON.

**Priority for timestamps:**
1. EXIF DateTimeOriginal (most reliable)
2. Filename embedded timestamp
3. JSON photoTakenTime
4. NULL if all missing

### organizer
Classifies input directories and organizes output by device/camera model using metadata from JSON sidecars (`googlePhotosOrigin`).

### logutil
Thread-safe file logging. Writes `INFO`/`SKIP`/`FAIL` entries to `takeout-helper-log/{command}-YYYYMMDD-NNN.log`.

### workerpool
Generic goroutine pool for parallel processing. Capped at `min(NumCPU, 8)` to avoid resource exhaustion.

### Other Packages
- **mediatype** - Detects true media type from file content (not extension)
- **progress** - Terminal progress bar for visual feedback
- **fileutil** - Shared file helpers (exists check, copy, etc.)
- **exifrunner** - Batches exiftool calls (optional, gracefully degrades if absent)
- **destlocker** - Prevents concurrent writes to the same path across goroutines

---

## Dry-run Mode

All operations check the `DryRun` flag. In dry-run mode:
- Files are NOT copied
- Timestamps are NOT modified
- Metadata files are NOT written
- Log files are NOT created
- Only stdout shows what *would* happen

Implementation: `logutil.Nop()` logger in dry-run to suppress file writes.

---

## Error Handling

Each file is processed independently. Errors don't block the entire migration:

- **Missing JSON sidecar** → File moved to manual_review, logged as SKIP
- **Timestamp resolution failure** → File moved to manual_review
- **Copy failure** → Logged as FAIL, file skipped
- **Permission error** → Logged as FAIL, continues to next file

Summary printed at end shows counts for each category.

---

## Testing

Tests are located in `*_test.go` files alongside package code:
- `internal/migrator/migrator_test.go` - Migration logic and timestamp resolution
- `internal/matcher/matcher_test.go` - JSON sidecar matching
- `internal/parser/parser_test.go` - Timestamp/GPS parsing
- `test/integration/` - Full pipeline integration tests

Run with: `make test`

---

## Performance

**Concurrency:** Uses `workerpool.Run()` with `min(NumCPU, 8)` goroutines to process files in parallel.

**Large Files:** HEIC conversion (if enabled) processes images >40MP serially to manage memory.

**Batching:** exiftool calls are batched up to 1000 files per subprocess invocation.

---

## Extension Points

To add new features in the future:

1. **New command** - Create `cmd/takeout-helper/cmd/mycommand.go`, register via `init()`
2. **New logic package** - Create `internal/myfeature/`, implement core logic
3. **New CLI flag** - Add to command definition in `.go` file
4. **New logging** - Use `logutil.Logger` interface already available

The Cobra framework is preserved for horizontal extensibility.
