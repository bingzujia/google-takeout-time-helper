# Developer Guide

This guide provides step-by-step instructions for extending the `takeout-helper` codebase, including how to add new commands, extend packages, and follow development conventions.

## Quick Start for Developers

**New to the codebase?**
1. Read [Architecture Overview](./architecture.md) to understand command delegation
2. Run `go test ./...` to verify your environment
3. Try `takeout-helper --help` to see available commands

**Making a change?**
1. Create a feature branch or task in openspec
2. Write tests first (TDD)
3. Implement the feature
4. Run tests and linting (`make test`, `make lint`)
5. Update docs if public API changed

---

## Adding a New Command

### Step-by-Step Guide

#### 1. Design the Command

Before coding, define:
- **Purpose:** What problem does this command solve?
- **Input:** What flags/arguments does it take?
- **Output:** What does it produce?
- **Example workflow:** How do users invoke it?

Document this in an openspec proposal before implementing.

#### 2. Create the Command File

Create `cmd/takeout-helper/cmd/newcommand.go` with self-registration via `init()`:

```go
package cmd

import (
    "fmt"
    "github.com/spf13/cobra"
    "github.com/bingzujia/google-takeout-time-helper/internal/newpkg"
)

func init() {
    rootCmd.AddCommand(newcommandCmd)  // Self-register
}

var newcommandCmd = &cobra.Command{
    Use:   "newcommand",
    Short: "Brief description of what this command does",
    Long: `Longer description with use cases and examples.
    
Can span multiple lines and include examples.`,
    
    RunE: func(cmd *cobra.Command, args []string) error {
        // 1. Parse flags
        inputDir, _ := cmd.Flags().GetString("input-dir")
        outputDir, _ := cmd.Flags().GetString("output-dir")
        dryRun, _ := cmd.Flags().GetBool("dry-run")
        
        // 2. Validate inputs
        if inputDir == "" {
            return fmt.Errorf("--input-dir is required")
        }
        
        // 3. Create logger
        var logger logutil.Logger
        if dryRun {
            logger = logutil.Nop()
        } else {
            var err error
            logger, err = logutil.NewFileLogger(outputDir, "newcommand")
            if err != nil {
                return err
            }
            defer logger.Close()
        }
        
        // 4. Create config and call internal package
        config := newpkg.Config{
            InputDir:  inputDir,
            OutputDir: outputDir,
            Dry:       dryRun,
        }
        stats, err := newpkg.ProcessDirectory(config, logger)
        if err != nil {
            return err
        }
        
        // 5. Print summary to stdout
        fmt.Printf("Processed: %d, Skipped: %d, Failed: %d\n",
            stats.Processed, stats.Skipped, stats.Failed)
        
        return nil
    },
}

func init() {
    // Add flags
    newcommandCmd.Flags().StringP("input-dir", "i", "", "Input directory")
    newcommandCmd.Flags().StringP("output-dir", "o", "", "Output directory")
    newcommandCmd.Flags().Bool("dry-run", false, "Preview only, do not modify files")
    
    // Mark required flags
    newcommandCmd.MarkFlagRequired("input-dir")
}
```

**Key conventions:**
- Use `--input-dir` for input directory (short: `-i`)
- Use `--output-dir` for output directory (short: `-o`)
- Always support `--dry-run` for safety preview
- Validate inputs upfront
- Create logger before processing (use `logutil.Nop()` in dry-run)
- Return `Stats` struct for summary printing
- Print summary to stdout

#### 3. Create the Internal Package

Create `internal/newpkg/newpkg.go` with core logic:

```go
package newpkg

import (
    "fmt"
    "github.com/bingzujia/google-takeout-time-helper/internal/logutil"
)

// Config holds configuration for the processor
type Config struct {
    InputDir  string
    OutputDir string
    Dry       bool
}

// Stats tracks processing statistics
type Stats struct {
    Processed int
    Skipped   int
    Failed    int
}

// ProcessDirectory runs the main processing logic
func ProcessDirectory(config Config, logger logutil.Logger) (*Stats, error) {
    stats := &Stats{}
    
    // 1. Validate config
    if config.InputDir == "" {
        return nil, fmt.Errorf("InputDir is required")
    }
    
    // 2. List files
    files, err := listFiles(config.InputDir)
    if err != nil {
        return nil, err
    }
    
    // 3. Process each file
    for _, file := range files {
        if err := processFile(file, config, logger); err != nil {
            logger.Fail("Error processing %s: %v", file, err)
            stats.Failed++
        } else {
            logger.Info("Processed %s", file)
            stats.Processed++
        }
    }
    
    return stats, nil
}

func listFiles(dir string) ([]string, error) {
    // Implementation
    return nil, nil
}

func processFile(path string, config Config, logger logutil.Logger) error {
    // Implementation
    return nil
}
```

**Key conventions:**
- Export `Config` and `Stats` structs
- Export main processing function (e.g., `ProcessDirectory`, `Migrate`)
- Accept `logutil.Logger` for logging
- Return `(*Stats, error)` tuple
- Keep internal functions unexported (lowercase)

#### 4. Add Tests

Create `internal/newpkg/newpkg_test.go`:

```go
package newpkg

import (
    "testing"
    "github.com/bingzujia/google-takeout-time-helper/internal/logutil"
)

func TestProcessDirectory_Success(t *testing.T) {
    config := Config{
        InputDir: "testdata/input",
        OutputDir: "testdata/output",
        Dry: true,
    }
    
    stats, err := ProcessDirectory(config, logutil.Nop())
    
    if err != nil {
        t.Fatalf("ProcessDirectory failed: %v", err)
    }
    
    if stats.Processed != 3 {
        t.Errorf("Expected 3 processed, got %d", stats.Processed)
    }
}

func TestProcessDirectory_MissingInput(t *testing.T) {
    config := Config{
        InputDir: "nonexistent",
        OutputDir: "testdata/output",
    }
    
    _, err := ProcessDirectory(config, logutil.Nop())
    
    if err == nil {
        t.Error("Expected error for missing input directory")
    }
}
```

**Test patterns:**
- Test success path with valid inputs
- Test error cases (missing files, invalid input)
- Use `testdata/` directory for test fixtures
- Use `logutil.Nop()` to suppress log output in tests
- Mock `exifrunner` if avoiding subprocess dependency

#### 5. Update Documentation

Add command documentation to `docs/commands.md`:

```markdown
## newcommand

**Purpose:** What this command does...

**Flags:**
- `--input-dir string` — Input directory (required)
- `--output-dir string` — Output directory
- `--dry-run` — Preview only

**Examples:**
```bash
# Basic usage
takeout-helper newcommand --input-dir ~/Photos --output-dir ~/Processed
```

**Output:**
- Description of what the command produces
```

Update module documentation to `docs/modules.md` if you created a new internal package.

#### 6. Run Tests and Lint

```bash
# Test your code
go test ./internal/newpkg/...

# Lint
go vet ./...

# Build and verify
go build -o bin/takeout-helper ./cmd/takeout-helper
./bin/takeout-helper newcommand --help
```

---

## Extending Existing Packages

### Example 1: Add New Timestamp Format

**Scenario:** You want to support a new timestamp format from a camera or phone.

**Steps:**

1. **Locate the package:** `internal/parser/`

2. **Add test case first (TDD):**

```go
// internal/parser/parser_test.go
func TestParseTime_NewFormat(t *testing.T) {
    t.Time, err := ParseTime("NEWFORMAT_20250101")
    
    if err != nil {
        t.Fatalf("ParseTime failed: %v", err)
    }
    
    if t.Year() != 2025 || t.Month() != 1 {
        t.Errorf("Incorrect parsing")
    }
}
```

3. **Implement the feature:**

```go
// internal/parser/parser.go
func ParseTime(s string) (*time.Time, error) {
    // ... existing code ...
    
    // Add new format
    if t, err := time.Parse("NEWFORMAT_20060102", s); err == nil {
        return &t, nil
    }
    
    // ... fallback to other formats ...
}
```

4. **Run tests:**

```bash
go test ./internal/parser/...
```

5. **Update documentation:**
   - Add format to `docs/modules.md` parser section
   - Update `docs/commands.md` if user-facing

### Example 2: Add New Media Classification Rule

**Scenario:** You want to classify images from a new source (e.g., Telegram exports).

**Steps:**

1. **Locate the package:** `internal/classifier/`

2. **Add test case:**

```go
// internal/classifier/classifier_test.go
func TestClassify_TelegramExports(t *testing.T) {
    config := Config{InputDir: "testdata/telegram"}
    stats, _ := Classify(config, logutil.Nop())
    
    if stats.Telegram != 2 {
        t.Errorf("Expected 2 Telegram files, got %d", stats.Telegram)
    }
}
```

3. **Create new bucket rule:**

```go
// internal/classifier/classifier.go
func matchesTelegram(filename string) bool {
    return strings.HasPrefix(filename, "photo_") && 
           strings.Contains(filename, "_export")
}

// Add to Stats struct:
type Stats struct {
    Camera    int
    Screenshot int
    WeChat     int
    Telegram   int  // ← NEW
    SeemsCamera int
    Skipped    int
}
```

4. **Integrate into Classify():**

```go
func Classify(config Config, logger logutil.Logger) (*Stats, error) {
    stats := &Stats{}
    
    for _, file := range files {
        if matchesCamera(file) {
            stats.Camera++
        } else if matchesTelegram(file) {
            stats.Telegram++  // ← NEW
        } else if matchesScreenshot(file) {
            stats.Screenshot++
        }
        // ... etc
    }
}
```

5. **Test and verify:**

```bash
go test ./internal/classifier/...
```

---

## Testing Patterns

### Unit Testing (Recommended)

Test internal package logic independently:

```go
func TestProcessFile_Success(t *testing.T) {
    // Setup
    filePath := "testdata/test_image.jpg"
    
    // Execute
    err := processFile(filePath)
    
    // Verify
    if err != nil {
        t.Fatalf("processFile failed: %v", err)
    }
}

func TestProcessFile_MissingFile(t *testing.T) {
    // Execute with nonexistent file
    err := processFile("nonexistent.jpg")
    
    // Verify error is returned
    if err == nil {
        t.Error("Expected error for missing file")
    }
}
```

### Mocking exifrunner

When testing code that calls `exifrunner`:

```go
// internal/mypackage/mypackage_test.go
func TestWithExifrunner(t *testing.T) {
    // Mock exifrunner behavior
    originalBatchWrite := exifrunner.BatchWrite
    defer func() { exifrunner.BatchWrite = originalBatchWrite }()
    
    exifrunner.BatchWrite = func(updates map[string]map[string]string) error {
        // Simulate success
        return nil
    }
    
    // Test your code
    // ...
}
```

### Test Fixtures

Store test data in `testdata/` directory:

```
internal/mypackage/testdata/
├── input/
│   ├── image1.jpg
│   └── image2.jpg
└── expected_output/
    └── result.json
```

### Integration Tests

Test CLI entry point with real internal packages:

```bash
# Build binary
go build -o /tmp/test-binary ./cmd/takeout-helper

# Run integration test
/tmp/test-binary migrate \
    --input-dir testdata/takeout \
    --output-dir /tmp/output \
    --dry-run

# Verify output
if [ -f /tmp/output/IMG_0001.JPG ]; then
    echo "Test passed"
fi
```

---

## Common Development Tasks

### Build the Binary

```bash
# Development build
go build -o bin/takeout-helper ./cmd/takeout-helper

# Or use Makefile
make build  # produces bin/takeout-helper
```

### Run All Tests

```bash
# Full test suite
go test ./...

# With verbose output
go test -v ./...

# With coverage
go test -cover ./...
```

### Run Tests for Single Package

```bash
go test ./internal/matcher/...
go test ./internal/migrator/...
go test ./cmd/takeout-helper/cmd/...
```

### Run Single Test

```bash
go test -run TestMigrate ./internal/migrator/...
```

### Run Linter

```bash
make lint  # runs go vet

# Or directly
go vet ./...
```

### Clean Build Artifacts

```bash
make clean  # removes bin/ directory
```

---

## Conventions and Best Practices

### Code Organization

- **CLI code:** `cmd/takeout-helper/cmd/` — Flag parsing, user I/O
- **Business logic:** `internal/*/` — Independently testable
- **Tests:** `*_test.go` alongside implementation
- **Test data:** `testdata/` directory

### Error Handling

**Always provide context in errors:**

```go
// ❌ Bad
return err

// ✓ Good
if err != nil {
    return fmt.Errorf("failed to copy file %s: %w", srcPath, err)
}
```

**Handle per-file errors gracefully:**

```go
for _, file := range files {
    if err := processFile(file); err != nil {
        logger.Fail("Error processing %s: %v", file, err)
        stats.Failed++
        continue  // ← Don't stop, continue with next file
    }
    stats.Processed++
}
```

### Logging

**Use logutil.Logger consistently:**

```go
// ✓ Correct
logger.Info("Processing file %s", filename)
logger.Skip("File has no JSON sidecar")
logger.Fail("Error writing EXIF: %v", err)

// ❌ Wrong
fmt.Println("Processing file...")  // Don't use fmt.Print
fmt.Printf("Error: %v", err)       // Don't use fmt.Printf in packages
```

### Concurrency

**Use workerpool for parallel operations:**

```go
import "github.com/bingzujia/google-takeout-time-helper/internal/workerpool"

results, err := workerpool.Run(
    jobs,      // []T of work items
    workers,   // int, actual workers after capping
    func(job T) (Result, error) {
        // Process one job
        return processJob(job)
    },
)
```

**Worker count is automatically capped:**
```go
workers := workerpool.MaxWorkers(requestedWorkers)
// Results in min(requestedWorkers, min(NumCPU, 8))
```

### Naming Conventions

- **Packages:** lowercase, short, singular or plural (e.g., `migrator`, `parser`)
- **Types/Structs:** PascalCase (e.g., `Config`, `Stats`)
- **Functions:** PascalCase if exported, camelCase if unexported
- **Constants:** UPPER_CASE or PascalCase
- **Variables:** camelCase

### Dry-Run Handling

Every command must support `--dry-run` for safety preview:

```go
// In CLI command
if dryRun {
    logger = logutil.Nop()  // No file writes or logging
}

// In internal package
if config.Dry {
    return nil  // Skip side effects
}
```

---

## Project Memory and Architectural Quirks

### JSON Sidecar Matching

The `matcher` package uses a 6-step degradation strategy because:
- Google Takeout truncates sidecar names at 51 characters
- Uses locale-specific "edited" suffixes
- Rearranges bracket-numbers in filenames

**Design principle:** All matching edge cases belong in the `matcher` package. Do NOT add heuristics elsewhere.

See memory notes on "JSON sidecar matching strategy" for details.

### HEIC Encoding: Always 4:2:0 Chroma

The `heicconv` package always outputs 4:2:0 chroma (not passthrough) because:
- Some cameras produce 4:4:4 or 4:2:2 chroma
- Passing to x265 causes it to use HEVC Range Extensions profile (profile_idc=4)
- HEIC brand container requires profile_idc ≤ 3 (Main or Main Still Picture)
- All decoders reject profile_idc=4 in HEIC container

**Design principle:** Force 4:2:0 output for universal compatibility.

### exiftool is Optional

All commands gracefully degrade when exiftool is unavailable:

```go
path, err := exifrunner.LookupPath()
if err != nil {
    logger.Skip("exiftool not found; skipping EXIF writes")
    // Continue processing without metadata
}
```

**Design principle:** Prioritize file integrity over metadata completeness.

### Metadata Priority (Resolution)

Timestamp and GPS are resolved in priority order:

**Timestamp:**
1. EXIF DateTimeOriginal (most reliable)
2. Filename embedded timestamp
3. JSON photoTakenTime
4. NULL if all missing

**GPS:**
1. EXIF GPS tags
2. JSON geoData
3. NULL if all missing

**Design principle:** EXIF is most reliable; filename is fallback for missing EXIF; JSON is last resort.

### Separate EXIF Timestamp Sources

The migrate command writes **CreateDate** and **FileModifyDate** from separate JSON fields to better represent photo metadata:

**CreateDate (when photo was taken):**
- Priority 1: JSON `photoTakenTime` 
- Fallback: Move to manual_review if missing

**FileModifyDate (when added to Google Photos):**
- Priority 1: JSON `creationTime`
- Priority 2: JSON `photoTakenTime` (fallback)
- Fallback: Move to manual_review if both missing

**DateTimeOriginal:** Never modified by migrate (preserved from existing EXIF).

**Timezone Conversion:** Unix timestamps from JSON (UTC) are automatically converted to local system timezone before writing to EXIF using `time.Unix()`.

**Metadata Tracking:** The metadata JSON includes `create_date.source` and `file_modify_date.source` fields for traceability:
- `"photoTakenTime"` - Used JSON photoTakenTime
- `"creationTime"` - Used JSON creationTime
- `"photoTakenTime_fallback"` - Used JSON photoTakenTime as fallback for FileModifyDate
- `"manual_review"` - File moved to manual_review due to missing timestamps

**Implementation:** See `ResolvePhotoTimestamp()` and `ResolveModifyTimestamp()` in `internal/migrator/migrator.go` for priority logic.

### Logging Location

Logs always go to `takeout-helper-log/` under the working directory:

- **Filename:** `{command}-YYYYMMDD-NNN.log`
- **Example:** `migrate-20250115-001.log`
- **Thread-safe:** All writes protected by mutex
- **Dry-run:** Uses `logutil.Nop()` to suppress file writes

**Design principle:** Single log directory for easy discovery; dated files for rotation.

---

## Debugging Techniques

### Print Debugging

```go
// Add temporary debug logging
logger.Info("DEBUG: variable value = %+v", value)
```

### Run Single Test with Debugging

```bash
go test -run TestName -v -timeout 30s ./internal/package/...
```

### Use Delve Debugger

```bash
# Install delve
go install github.com/go-delve/delve/cmd/dlv@latest

# Debug a test
dlv test ./internal/matcher/...

# Set breakpoint and continue
(dlv) break TestJSONForFile
(dlv) continue
```

### Profile Performance

```bash
# CPU profile
go test -cpuprofile=cpu.prof ./internal/dedup/...
go tool pprof cpu.prof

# Memory profile
go test -memprofile=mem.prof ./internal/heicconv/...
go tool pprof mem.prof
```

### Trace Execution

```bash
go test -trace=trace.out ./internal/matcher/...
go tool trace trace.out
```

---

## Release and Deployment

### Version Bumping

Versions follow semantic versioning: MAJOR.MINOR.PATCH

- **MAJOR:** Breaking API changes
- **MINOR:** New features (backward compatible)
- **PATCH:** Bug fixes

Update in `go.mod` and git tag:

```bash
git tag v1.2.3
git push origin v1.2.3
```

### Creating Release Builds

The CI/CD pipeline (GitHub Actions) builds for all platforms on tag:
- linux/amd64
- darwin/amd64 (macOS Intel)
- darwin/arm64 (macOS Apple Silicon)
- windows/amd64

Releases are triggered by pushing git tags: `git push origin v1.2.3`

### Changelog

Update `CHANGELOG.md` (if exists) with each release:

```markdown
## [1.2.3] - 2025-01-15

### Added
- New feature description

### Fixed
- Bug fix description

### Changed
- Breaking change description
```

---

## Troubleshooting

### Tests Failing

1. **Check dependencies:** `go mod tidy`
2. **Clean build:** `rm -rf bin/ && go build ./...`
3. **Run failing test with verbose output:** `go test -v -run TestName ./...`
4. **Check test fixtures:** Ensure `testdata/` files exist

### Build Failing

1. **Check Go version:** `go version` (project requires Go 1.20+)
2. **Verify dependencies:** `go mod verify`
3. **Check for syntax errors:** `go vet ./...`

### Binary Not Working

1. **Verify build:** `./bin/takeout-helper --version`
2. **Check help:** `./bin/takeout-helper --help`
3. **Try basic command:** `./bin/takeout-helper migrate --help`
4. **Check for exiftool:** `which exiftool`

### Integration Tests Failing

1. **Verify test data:** Check `testdata/` directory exists
2. **Check permissions:** `ls -la testdata/`
3. **Verify temp directory:** `/tmp/` must be writable
4. **Run with verbose output:** `go test -v ./...`

---

## Further Reading

- [Architecture Overview](./architecture.md) — System design and data flows
- [Commands Reference](./commands.md) — User-facing documentation
- [Modules Guide](./modules.md) — Internal package reference
- [Go Documentation](https://golang.org/doc/) — Go language docs
- [Cobra Documentation](https://cobra.dev/) — CLI framework reference
