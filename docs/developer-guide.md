# Developer Guide

This guide covers the `takeout-helper` codebase, architecture, and how to extend or modify the migrate command.

---

## Quick Start

### Build

```bash
make build          # produces bin/takeout-helper
make test           # go test ./...
make lint           # go vet ./...
make clean          # removes bin/
```

### Project Structure

```
.
├── cmd/takeout-helper/
│   └── cmd/
│       ├── root.go              # Cobra root command, entry point
│       ├── migrate.go           # migrate subcommand definition
│       └── main.go              # main() function
├── internal/
│   ├── migrator/                # Core migration logic
│   ├── matcher/                 # JSON sidecar matching (6-step strategy)
│   ├── parser/                  # Timestamp/GPS parsing
│   ├── organizer/               # Photo classification by device
│   ├── logutil/                 # Thread-safe file logging
│   ├── workerpool/              # Goroutine pool for parallel processing
│   ├── progress/                # Terminal progress bar
│   ├── fileutil/                # File helpers
│   ├── exifrunner/              # exiftool subprocess wrapper
│   ├── mediatype/               # Media type detection
│   └── destlocker/              # Concurrent write protection
├── docs/
│   ├── README.md                # User guide (already in repo root)
│   ├── architecture.md          # System design
│   └── developer-guide.md       # This file
├── test/
│   └── integration/             # Integration tests
├── Makefile                     # Build commands
├── go.mod / go.sum              # Module definition
└── LICENSE
```

---

## Module Path

```
github.com/bingzujia/google-takeout-time-helper
```

---

## Key Concepts

### Timestamp Resolution

The migrate command sets file modification time using this priority:

**Primary:** JSON `photoTakenTime` (when photo was taken)  
**Fallback:** JSON `creationTime` (when photo was uploaded to Google Photos)  
**Manual Review:** If both missing, file is moved to `manual_review/` subfolder

```go
// internal/migrator/migrator.go
func ResolveModifyTimestamp(metadata *Metadata) (int64, TSSource) {
    // Returns Unix timestamp and source (photoTakenTime/creationTime/manual_review)
    if metadata.PhotoTakenTime > 0 {
        return metadata.PhotoTakenTime, SourcePhotoTakenTime
    }
    if metadata.CreationTime > 0 {
        return metadata.CreationTime, SourceCreationTime
    }
    return 0, SourceManualReview
}
```

### File Timestamp Setting

Timestamps are set using Go stdlib `os.Chtimes()`:

```go
// internal/migrator/migrator.go
func applyFileTimestamp(filePath string, modifyTimestamp int64) error {
    if modifyTimestamp == 0 {
        return nil  // Skip if timestamp is zero
    }
    // Convert Unix seconds to time.Time
    mtime := time.Unix(modifyTimestamp, 0)
    // Set file modification time (access time = modification time)
    return os.Chtimes(filePath, mtime, mtime)
}
```

**Why `os.Chtimes()` instead of exiftool?**
- Cross-platform (Windows, macOS, Linux)
- No external tool dependency
- Simpler pipeline
- No EXIF modification (only file timestamps)

### JSON Sidecar Matching

The `matcher.JSONForFile()` function uses a 6-step degradation strategy to find the JSON sidecar:

1. **Identity** - Look for exact filename match: `photo.jpg.json`
2. **Shorten** - Google Takeout truncates at 51 chars: try shortened names
3. **Bracket Swap** - Handle `IMG_1234 (1).jpg` → `IMG_1234.jpg`
4. **Remove Extra** - Remove extra characters added by Google
5. **Supplemental** - Try `photo.supplemental-metadata.json`
6. **No Extension** - Try matching without file extension

This handles all Google Takeout filename quirks gracefully.

```go
// internal/matcher/json_matcher.go
func JSONForFile(photoPath string, dir string) string {
    // Returns path to JSON sidecar, or "" if not found
}
```

### Parallel Processing

The migrate command uses a worker pool to process files in parallel:

```go
// internal/workerpool/pool.go
func Run[J any](jobs []J, workers int, fn func(J) error) error {
    // Process jobs with up to `workers` goroutines
    // Default: min(NumCPU, 8)
}
```

Workers are capped at `min(NumCPU, 8)` to avoid resource exhaustion on high-CPU systems.

### Logging

All decisions are logged to `takeout-helper-log/migrate-YYYYMMDD-NNN.log`:

```go
// Example log entries
// INFO: File successfully migrated (photoTakenTime)
// SKIP: File already exists at destination
// FAIL: Error copying file (permission denied)
```

The logger is thread-safe (mutex-protected) for concurrent writes from worker goroutines.

**Dry-run Mode:** Uses `logutil.Nop()` logger to suppress file writes.

---

## Architecture Reference

See [docs/architecture.md](architecture.md) for full system design, data flows, and package responsibilities.

---

## Writing Tests

Tests should be placed in `*_test.go` files alongside package code.

### Example: Testing Timestamp Resolution

```go
// internal/migrator/migrator_test.go
func TestResolveModifyTimestamp(t *testing.T) {
    tests := []struct {
        name     string
        meta     *Metadata
        expected int64
        source   TSSource
    }{
        {
            name: "photoTakenTime present",
            meta: &Metadata{
                PhotoTakenTime: 1683012040,
                CreationTime:   1683012041,
            },
            expected: 1683012040,
            source:   SourcePhotoTakenTime,
        },
        {
            name: "only creationTime",
            meta: &Metadata{
                CreationTime: 1683012041,
            },
            expected: 1683012041,
            source:   SourceCreationTime,
        },
        {
            name: "both missing",
            meta: &Metadata{},
            expected: 0,
            source:   SourceManualReview,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, gotSource := ResolveModifyTimestamp(tt.meta)
            if got != tt.expected || gotSource != tt.source {
                t.Errorf("got %d (%v), want %d (%v)", 
                    got, gotSource, tt.expected, tt.source)
            }
        })
    }
}
```

### Running Tests

```bash
# All tests
make test

# Single package
go test ./internal/migrator/...

# Specific test
go test ./internal/migrator/... -run TestResolveModifyTimestamp

# With verbose output
go test -v ./...

# With coverage
go test -cover ./...
```

---

## Adding a New Feature

### Example: Add Support for Custom Output Folder Names

1. **Modify migrator Config:**
   ```go
   // internal/migrator/migrator.go
   type Config struct {
       // ... existing fields
       FolderNaming string  // "device" (default), "year", "camera"
   }
   ```

2. **Update migrate command:**
   ```go
   // cmd/takeout-helper/cmd/migrate.go
   migrateCmd.Flags().StringVar(&migrateFolder, "folder-naming", "device", 
       "how to organize output folders: device, year, camera")
   ```

3. **Implement logic in migrator:**
   ```go
   // internal/migrator/migrator.go
   func buildOutputPath(config Config, metadata *Metadata, filename string) string {
       switch config.FolderNaming {
       case "device":
           return filepath.Join(config.OutputDir, metadata.Device, filename)
       case "year":
           return filepath.Join(config.OutputDir, metadata.Year, filename)
       // ...
       }
   }
   ```

4. **Add tests:**
   ```go
   // internal/migrator/migrator_test.go
   func TestBuildOutputPath_CustomNaming(t *testing.T) {
       // Test different folder naming strategies
   }
   ```

5. **Test manually:**
   ```bash
   make build
   ./bin/takeout-helper migrate --input-dir ~/Takeout --output-dir ~/Photos --folder-naming year
   ```

---

## Debugging

### Enable Verbose Logging

Add debug output to specific functions:

```go
// internal/migrator/migrator.go
if config.Debug {
    logger.Log(fmt.Sprintf("DEBUG: Processing %s, timestamp: %d", filePath, ts))
}
```

### Check Migration Log

After running migrate, check the detailed log:

```bash
cat takeout-helper-log/migrate-20260421-001.log
```

### Test with Dry-run

Always test destructive operations with `--dry-run`:

```bash
./bin/takeout-helper migrate --input-dir ~/Takeout --output-dir ~/Photos --dry-run
```

### Run Specific Test

```bash
go test ./internal/migrator/... -run TestProcessSingleFile -v
```

---

## Performance Tips

### Profile Memory Usage

```bash
go test -memprofile=mem.prof ./internal/migrator/...
go tool pprof mem.prof
```

### Adjust Worker Count

The worker pool defaults to `min(NumCPU, 8)`. Modify if needed:

```go
// internal/workerpool/pool.go
workers := runtime.NumCPU()  // Use all CPUs
stats, err := migrator.Run(config, workerCount)
```

### Batch Operations

exiftool calls are batched up to 1000 files per subprocess. Useful for large migrations.

---

## Common Issues

### Build Fails: "undefined reference"

Clear build cache:
```bash
go clean -cache
make build
```

### Tests Fail After Package Changes

Run full test suite to catch import errors:
```bash
make test
```

### Migrate Process Hangs

Check for deadlocks in concurrent code. Use `-race` flag:
```bash
go test -race ./...
```

---

## CI/CD

GitHub Actions workflow: `.github/workflows/ci.yml`

- Runs `go build ./...` and `go test ./...` on every push/PR
- Tests on Go 1.24
- Produces release binaries for: linux/amd64, windows/amd64, darwin/amd64, darwin/arm64

---

## Code Style

- Follow standard Go conventions (go fmt)
- Run `go vet ./...` before committing
- Use meaningful variable names
- Keep functions focused and testable
- Comment exported functions and types

---

## Resources

- [Go Documentation](https://golang.org/doc/)
- [Cobra Guide](https://github.com/spf13/cobra)
- [Google Takeout Format](https://support.google.com/photos/answer/10157233)
- [Project Architecture](architecture.md)
