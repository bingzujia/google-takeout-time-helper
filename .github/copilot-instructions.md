# Copilot Instructions

## Build, Test & Lint

```bash
make build          # produces bin/takeout-helper
make test           # go test ./...
make lint           # go vet ./...
make clean          # removes bin/
```

Run a single package's tests:
```bash
go test ./internal/matcher/...
go test ./internal/migrator/...
```

Run a specific test:
```bash
go test ./internal/matcher/... -run TestJSONForFile
```

Build binary directly:
```bash
go build -o bin/takeout-helper ./cmd/takeout-helper
```

## Architecture

The CLI is a [Cobra](https://github.com/spf13/cobra) app. `cmd/takeout-helper/main.go` delegates to `cmd/takeout-helper/cmd/`, where each subcommand (`migrate`, `classify`, `convert`, `fix-exif`, `fix-name`, `dedup`, `rename`) lives in its own file and **self-registers via `init()`**.

Each subcommand delegates to a corresponding `internal/` package that contains the core logic and is independently testable.

### Internal package responsibilities

| Package | Role |
|---|---|
| `matcher` | Locates the JSON sidecar for a photo using a 6-step degradation strategy (identity → shorten → bracket-swap → remove-extra → no-extension → supplemental) to handle all Google Takeout filename quirks |
| `migrator` | Core migration pipeline: scan year folders, copy files, write EXIF dates/GPS from JSON sidecar via exiftool |
| `parser` | Parses timestamps from EXIF (`DateTimeOriginal`) and filenames; parses GPS from EXIF |
| `exifrunner` | Batches all `exiftool` calls (up to 1000 files/subprocess) to avoid per-file process spawning; degrades gracefully when `exiftool` is absent |
| `workerpool` | Generic goroutine pool — `workerpool.Run[J](jobs, workers, fn)` — capped at `min(NumCPU, 8)` by default |
| `logutil` | Thread-safe file logger writing `INFO`/`SKIP`/`FAIL` entries; no-op logger (`logutil.Nop()`) used in dry-run/testing |
| `organizer` | Classifies input dirs into year folders (`Photos from XXXX` pattern) |
| `classifier` | Classifies files into camera/screenshot/wechat/seemsCamera buckets by filename rules and EXIF Make/Model |
| `mediatype` | Detects true media type from file content (not extension) |
| `dedup` | Perceptual deduplication using pHash + dHash dual check |
| `hashcache` | SQLite-backed cache (via `modernc.org/sqlite`) for pHash/dHash values |
| `heicconv` | Wraps system `heif-enc` for HEIC conversion; `encoder_heifenc.go` shells out to the binary |
| `destlocker` | Prevents concurrent writes to the same output path across goroutines |
| `renamer` | Generates standardized filenames (`IMG_YYYYMMDD_HHMMSS`, `VID...`, burst detection) |
| `progress` | Terminal progress bar |
| `fileutil` | Shared file helpers |

## Key Conventions

**`exiftool` is optional.** All commands that write EXIF gracefully degrade when `exiftool` is absent — files are still copied/processed, just without metadata writes. Check availability via `exifrunner.LookupPath()`.

**Every command supports `--dry-run`.** Dry-run must not touch the filesystem or produce log files. The `logutil.Nop()` logger is used in this mode.

**Command result structs follow the `Stats` pattern.** Each command returns a `Stats` struct (e.g., `migrator.Stats`) with named counts (Processed, Skipped, Failed, etc.) — used to print the summary to stdout.

**JSON sidecar matching is complex.** The `matcher.JSONForFile` function implements a multi-step strategy because Google Takeout truncates sidecar names at 51 characters, uses locale-specific "edited" suffixes, and moves bracket-numbers. When adding new matching edge cases, extend `json_matcher.go` — do not add heuristics elsewhere.

**Timestamp resolution priority** (used by `matcher.ResolveTimestamp`):
1. EXIF `DateTimeOriginal`
2. Filename-embedded timestamp
3. JSON `photoTakenTime.timestamp`

**GPS resolution priority** (used by `matcher.ResolveGPS`):
1. EXIF GPS tags
2. JSON `geoData` fields

**Commands only process first-level files** (non-recursive) for `classify`, `convert`, `fix-exif`, `fix-name`, `dedup`, and `rename`. Only `migrate` recurses into year sub-folders.

**Logs are written to `takeout-helper-log/`** under `--output-dir` (for `migrate`/`classify`) or `--input-dir` (for all others), named `{command}-{YYYYMMDD}-{NNN}.log`.

**Module path:** `github.com/bingzujia/google-takeout-time-helper`

**Release builds** target four platforms via CI matrix: `linux/amd64`, `windows/amd64`, `darwin/amd64`, `darwin/arm64`. Release triggers on git tags.
