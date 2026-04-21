# CLI Commands Reference

This document provides complete reference for all `takeout-helper` commands, including purpose, flags, examples, and typical workflows.

## Quick Reference

| Command | Purpose | Typical Use |
|---------|---------|-------------|
| `migrate` | Migrate Google Takeout photos with EXIF metadata | Initial import from Google Takeout export |
| `classify` | Classify media by type (camera, screenshot, WeChat, etc.) | Organize mixed media sources |
| `convert` | Convert images to HEIC format | Reduce file sizes with modern codec |
| `fix-exif` | Sync DateTimeOriginal → CreateDate & ModifyDate | Fix timestamp consistency |
| `fix-name` | Sync filename datetime → EXIF DateTimeOriginal | Use filenames as source of truth |
| `dedup` | Find and group duplicate images | Remove redundant files |
| `rename` | Rename files with timestamp-based naming | Standardize filenames across collection |
| `rename-screenshot` | Rename screenshot files | Standardize screenshot naming |
| `format-qq-media` | Format QQ-exported media | Standardize QQ media filenames |

---

## migrate

**Purpose:** Migrate photos from Google Takeout export to a clean directory structure with EXIF metadata restoration.

**What it does:**
- Scans year folders (`Photos from XXXX`) in input directory
- Copies files to output directory
- Restores metadata from JSON sidecars to EXIF tags using exiftool
- Generates SHA-256-based metadata JSON files
- Creates log file with per-file decisions

### Flags

```
--input-dir string     Input directory containing Google Takeout exports (required)
--output-dir string    Output directory for organized photos (required)
--year string          Only process specific year folder(s) (optional)
--dry-run              Preview migration without modifying files
--quality int          HEIC encoding quality for photos converted to HEIC (1–100, default: 75)
```

### Examples

```bash
# Basic migration
takeout-helper migrate --input-dir ~/Takeout --output-dir ~/Photos

# Preview only
takeout-helper migrate --input-dir ~/Takeout --output-dir ~/Photos --dry-run

# Process specific year only
takeout-helper migrate --input-dir ~/Takeout --output-dir ~/Photos --year 2023
```

### Output

- Organized photo files in output directory with metadata restored
- `takeout-helper-log/migrate-YYYYMMDD-NNN.log` with processing summary (Processed, Skipped, Failed counts)
- `.metadata.json` files tracking metadata sources (JSON sidecar, EXIF, filename)

### Notes

- Files without a JSON sidecar are copied as-is (counted as Processed, not Failed)
- GPS metadata is supplemented from JSON when absent from EXIF
- Requires exiftool for metadata writes (gracefully degrades if absent)
- exiftool is optional; files are still copied without it

---

## classify

**Purpose:** Classify media files into buckets (camera, screenshot, WeChat, other).

**What it does:**
- Scans top-level files in input directory (non-recursive)
- Moves files into subdirectories of output directory based on classification rules
- Creates log file with classification summary

### Classification Rules

- **camera/** — Filename matches known camera patterns: `IMG_`, `VID_`, `PXL_`, `DJI_`, etc.
- **screenshot/** — Filename contains "screenshot" (case-insensitive)
- **wechat/** — Filename starts with "mmexport" (WeChat export)
- **seemsCamera/** — No filename match but exiftool detects camera Make/Model in EXIF
- **(left in place)** — No match; counted as skipped

### Flags

```
--input-dir string     Input directory containing media files (required)
--output-dir string    Output directory for classified subdirectories (required)
--dry-run              Preview classification without moving files
```

### Examples

```bash
# Basic classification
takeout-helper classify --input-dir ~/Photos --output-dir ~/Classified

# Preview only
takeout-helper classify --input-dir ~/Photos --output-dir ~/Classified --dry-run
```

### Output

- Organized media in subdirectories: camera/, screenshot/, wechat/, seemsCamera/
- Unclassified files remain in input directory
- `takeout-helper-log/classify-YYYYMMDD-NNN.log` with classification counts

---

## convert

**Purpose:** Convert images to HEIC format in place with metadata preservation.

**What it does:**
- Scans top-level image files in input directory
- Converts to .heic with configurable quality
- Corrects file extensions if they don't match actual content
- Migrates EXIF metadata to HEIC file
- Deletes original only after successful conversion

### Requirements

- `heif-enc` binary (install: `sudo apt-get install -y libheif-examples`)
- `exiftool` (for metadata copy)

### Flags

```
--input-dir string     Input directory containing images (required)
--quality int          HEIC encoding quality, 1–100 (default: 75)
                       - 50: smaller files (~50% of original)
                       - 75: default, balanced
                       - 90: high quality, larger files
--dry-run              Preview conversions without modifying files
--workers int          Concurrent conversion workers, 1–N (default: 2)
                       Reduce to limit memory usage on large batches
```

### Examples

```bash
# Convert all images to HEIC with default quality 75
takeout-helper convert --input-dir ~/Photos

# High quality conversion
takeout-helper convert --input-dir ~/Photos --quality 90

# Smaller files with reduced quality
takeout-helper convert --input-dir ~/Photos --quality 50

# Preview only
takeout-helper convert --input-dir ~/Photos --dry-run

# Limit workers for low-memory systems
takeout-helper convert --input-dir ~/Photos --workers 1
```

### Supported Formats

- Input: JPEG, PNG, BMP, GIF, TIFF, WebP, HEIC, HEIF
- Output: HEIC (with .heic extension)

### Quality Guidance

- **35–50**: Photos only; avoid for screenshots or high-contrast text (creates visible artifacts)
- **75** (default): Balanced for mixed media
- **85–95**: High quality; use when storage is not constrained

### Limitations

- **Dimension limit:** Images with any dimension > 16,383 px are skipped (Apple ImageIO limit)
- **Oversized detection:** Images > 40 million pixels processed one-at-a-time to reduce peak memory
- Existing .heic files are skipped

### Output

- Converted .heic files with original filenames
- Original files deleted after successful conversion
- `takeout-helper-log/convert-YYYYMMDD-NNN.log` with conversion summary

---

## fix-exif

**Purpose:** Sync DateTimeOriginal → CreateDate & ModifyDate using exiftool.

**What it does:**
- Reads DateTimeOriginal from each file's EXIF tags
- Writes the same value to CreateDate and ModifyDate
- Handles file extension mismatches transparently

### Requirements

- `exiftool` (required)

### Flags

```
--input-dir string     Target directory (required)
--dry-run              Preview only, do not modify files
```

### Examples

```bash
# Fix all EXIF timestamps
takeout-helper fix-exif --input-dir ~/Photos

# Preview only
takeout-helper fix-exif --input-dir ~/Photos --dry-run
```

### Behavior

- Files without DateTimeOriginal are skipped
- Files with extension mismatch (e.g., JPEG named .png) are transparently handled
- Output: CreateDate and ModifyDate synced to DateTimeOriginal value

---

## fix-name

**Purpose:** Use filename datetime as source of truth for EXIF metadata when more reliable than embedded timestamps.

**What it does:**
- Parses datetime from media filename (e.g., IMG_20250101_120000)
- Compares with EXIF DateTimeOriginal
- Writes DateTimeOriginal + CreateDate + ModifyDate when filename is earlier or EXIF is missing

### Requirements

- `exiftool` (required)

### Flags

```
--input-dir string     Target directory (required)
--dry-run              Preview only, do not modify files
```

### Examples

```bash
# Sync filename datetime to EXIF
takeout-helper fix-name --input-dir ~/Photos

# Preview only
takeout-helper fix-name --input-dir ~/Photos --dry-run
```

### Supported Filename Formats

- `IMG_YYYYMMDD_HHMMSS` or `IMG_YYYYMMDD-HHMMSS`
- `VID_YYYYMMDD_HHMMSS` or `VID_YYYYMMDD-HHMMSS`
- `YYYYMMDDHHMMSS` (dense format)
- `YYYY-MM-DD HH-MM-SS` or `YYYY-MM-DD_HH-MM-SS`

### Behavior

- Files with no parseable datetime are skipped
- Only updates EXIF if filename timestamp is earlier than existing DateTimeOriginal
- Non-recursive (processes only top-level files)

---

## dedup

**Purpose:** Find and group duplicate images using perceptual hashing.

**What it does:**
- Computes pHash (perceptual hash) for each image
- Computes dHash for additional duplicate detection accuracy
- Caches hashes in SQLite for fast re-runs
- Groups similar images by hash distance
- Moves duplicate groups to `dedup/group-001/`, `dedup/group-002/`, etc.

### Flags

```
--input-dir string        Input directory to scan (required)
--cache-dir string        Directory for hash cache DB (default: <input_dir>/.gtoh_cache)
--threshold int           Max perceptual hash distance to consider duplicates (default: 10)
                          Lower = stricter matching; higher = more permissive
--convert-heic            Convert HEIC/HEIF to JPEG for processing (default: true)
--decode-workers int      Max concurrent image decodes (0 = unlimited)
--max-decode-mb int       Skip images > this size to prevent OOM (default: 500 MB)
--no-cache                Disable hash cache, always recompute hashes
--auto                    Automatic mode: keep largest in root, move rest to group-xxx/
--dry-run                 Preview without moving files
```

### Examples

```bash
# Find duplicates and group them
takeout-helper dedup --input-dir ~/Photos

# Preview only
takeout-helper dedup --input-dir ~/Photos --dry-run

# Stricter matching (fewer false positives)
takeout-helper dedup --input-dir ~/Photos --threshold 5

# Automatic mode (keep largest)
takeout-helper dedup --input-dir ~/Photos --auto

# Disable caching for one-off runs
takeout-helper dedup --input-dir ~/Photos --no-cache
```

### Output Structure

```
input_dir/
├── dedup/
│   ├── group-001/
│   │   ├── image1.jpg
│   │   └── image2.jpg
│   └── group-002/
│       └── similar_image.png
├── file1.jpg           (kept in root)
└── file2.png           (kept in root)
```

### Notes

- Hash cache stored at `.gtoh_cache/hashes.db` for fast re-runs
- Use `--no-cache` to rebuild hashes after replacing images
- Threshold tuning: start at default 10, adjust based on results
- HEIC files converted to JPEG for hashing (originals preserved)

---

## rename

**Purpose:** Rename media files using timestamp-based standardized naming conventions.

**What it does:**
- Parses timestamp from EXIF or filename
- Renames using pattern: `IMG_YYYYMMDD_HHMMSS` or `VID_YYYYMMDD_HHMMSS`
- Detects burst (连拍) sequences and adds `_BURST_NNN` suffix
- Detects screenshots and uses `Screenshot_YYYY-MM-DD-HH-MM-SS-MS` pattern
- Handles MP4 companions (same-name .mp4 renamed with image)
- Appends `_001`, `_002` for filename conflicts

### Flags

```
--input-dir string     Target directory (required)
--dry-run              Preview only, do not modify files
```

### Examples

```bash
# Rename all media files
takeout-helper rename --input-dir ~/Photos

# Preview only
takeout-helper rename --input-dir ~/Photos --dry-run
```

### Naming Patterns

| Category | Pattern | Example |
|----------|---------|---------|
| HEIC Images | `IMG{YYYYMMDD}{HHMMSS}.heic` | `IMG20250101120000.heic` |
| Other Images | `IMG_{YYYYMMDD}_{HHMMSS}.ext` | `IMG_20250101_120000.jpg` |
| Videos | `VID{YYYYMMDD}{HHMMSS}.ext` | `VID20250101120000.mp4` |
| Burst (HEIC) | `IMG{YYYYMMDD}{HHMMSS}_BURST{NNN}.heic` | `IMG20250101120000_BURST001.heic` |
| Burst (Other) | `IMG_{YYYYMMDD}_{HHMMSS}_BURST{NNN}.ext` | `IMG_20250101_120000_BURST001.jpg` |
| Screenshots | `Screenshot_{YYYY-MM-DD-HH-MM-SS-MS}.ext` | `Screenshot_2025-01-01-12-00-00-000.png` |

### Burst Detection

Files trigger burst naming when:
- Filename matches `YYYYMMDD_HHMMSS_NNN` pattern
- ≥ 2 files share same date/time prefix

### Conflict Resolution

If target filename already exists, appends `_001`, `_002`, etc.

### Notes

- Non-recursive (processes only top-level files)
- MP4 companions follow image name (e.g., `IMG_20250101_120000.mp4` → `IMG_20250101_120000_BURST001.mp4`)

---

## rename-screenshot

**Purpose:** Rename screenshot files with standardized datetime-based naming.

**What it does:**
- Detects screenshot timestamp in various formats
- Renames to standard format: `Screenshot_YYYY-MM-DD-HH-MM-SS-MS.ext`
- Handles Unix timestamps (seconds and milliseconds)
- Falls back to file modification time if no timestamp found
- Appends `_001`, `_002` for conflicts

### Supported Timestamp Formats

1. `YYYY-MM-DD-HH-MM-SS-MS` — e.g., `Screenshot_2025-07-18-09-23-54-65.png`
2. `YYYYMMDD_HHMMSS` — e.g., `screenshot20250718_092354.jpg`
3. `YYYY-MM-DD_HH-MM-SS` — e.g., `Screenshot_2025-07-18_09-23-54.png`
4. `YYYY_M_D_H_M_S` — e.g., `screenshot_2025_7_18_9_23_54.png`
5. `YYYY-MM-DD` (date only) — e.g., `screenshot_2025-07-18.png`
6. Unix timestamp (10 digits, seconds) — e.g., `screenshot1634560000.jpg`
7. Unix timestamp (13 digits, milliseconds) — e.g., `mmscreenshot1727421404387.jpg`

### Flags

```
--input-dir string     Target directory (required)
--dry-run              Preview only, do not modify files
```

### Examples

```bash
# Rename all screenshots
takeout-helper rename-screenshot --input-dir ~/Screenshots

# Preview only
takeout-helper rename-screenshot --input-dir ~/Screenshots --dry-run
```

### Output Format

All screenshots renamed to: `Screenshot_YYYY-MM-DD-HH-MM-SS-MS.ext`

Example transformations:
- `Screenshot_2025-07-18-09-23-54-65.png` → `Screenshot_2025-07-18-09-23-54-65.png`
- `screenshot20250718_092354.jpg` → `Screenshot_2025-07-18-09-23-54-000.jpg`
- `screenshot1634560000.jpg` → `Screenshot_2021-10-18-14-26-40-000.jpg`

---

## format-qq-media

**Purpose:** Format QQ-exported media files with standardized naming based on timestamps.

**What it does:**
- Detects timestamp in QQ filename patterns
- Renames to standardized format: `Image_<unix-ms>.ext` or `Video_<unix-ms>.ext`
- Falls back to file modification time if no timestamp found

### Supported QQ Filename Patterns

1. `_YYYYMMDD_HHMMSS` — e.g., `photo_20170709_002844.jpg`
2. 13-digit Unix milliseconds — e.g., `photo_1688017744459.jpg`
3. `QQ视频YYYYMMDDHHMMSS` — e.g., `QQ视频20150720105516.mp4`
4. `Record_YYYY-MM-DD-HH-MM-SS` — e.g., `Record_2024-12-19-16-07-17.mp4`
5. `Snipaste_YYYY-MM-DD_HH-MM-SS` — e.g., `Snipaste_2018-09-17_18-07-29.png`
6. `tb_image_share_13digits` — e.g., `tb_image_share_1661951220361.jpg`
7. `TIM图片YYYYMMDDHHMMSS` — e.g., `TIM图片20181215191143.jpg`
8. Fallback: File modification time

### Flags

```
--input-dir string     Target directory (required)
--dry-run              Preview only, do not modify files
```

### Examples

```bash
# Format QQ media
takeout-helper format-qq-media --input-dir ~/QQExport

# Preview only
takeout-helper format-qq-media --input-dir ~/QQExport --dry-run
```

### Output Format

- Images: `Image_<unix-ms>.ext`
- Videos: `Video_<unix-ms>.ext`

Example transformations:
- `photo_1688017744459.jpg` → `Image_1688017744459.jpg`
- `QQ视频20150720105516.mp4` → `Video_1437398716000.mp4`
- `photo_20170709_002844.jpg` → `Image_1499617724000.jpg`

### Notes

- Requires timestamp detection; falls back to file mtime if no pattern matches
- Non-recursive (top-level files only)

---

## Common Workflows

### Google Takeout Migration

```bash
# 1. Migrate with metadata
takeout-helper migrate --input-dir ~/Takeout --output-dir ~/Photos

# 2. Classify mixed sources
takeout-helper classify --input-dir ~/Photos --output-dir ~/Photos/Classified

# 3. Convert to HEIC to save space
takeout-helper convert --input-dir ~/Photos --quality 75

# 4. Deduplicate
takeout-helper dedup --input-dir ~/Photos

# 5. Standardize naming
takeout-helper rename --input-dir ~/Photos
```

### Screenshot Organization

```bash
# Rename all screenshots
takeout-helper rename-screenshot --input-dir ~/Screenshots

# Classify if needed
takeout-helper classify --input-dir ~/Screenshots --output-dir ~/Screenshots/Classified
```

### General Cleanup

```bash
# Fix timestamp consistency
takeout-helper fix-exif --input-dir ~/Photos

# Standardize naming
takeout-helper rename --input-dir ~/Photos

# Remove duplicates
takeout-helper dedup --input-dir ~/Photos --auto
```

---

## Tips and Tricks

- **Always preview first:** Use `--dry-run` to see what would happen before making changes
- **Check logs:** Review `takeout-helper-log/` for detailed per-file decisions
- **Batch convert:** Use `convert --quality 50` for older devices with limited storage
- **Hash caching:** Re-run `dedup` without `--no-cache` for fast re-processing after changes
- **Extension mismatch:** Commands like `fix-exif` and `convert` handle mismatched extensions transparently

---

## Need Help?

- Run `takeout-helper --help` to see all available commands
- Run `takeout-helper <command> --help` for command-specific options
- Check logs in `takeout-helper-log/` for detailed operation records
