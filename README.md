# Google Takeout Time Helper - migrate

> **Cross-platform Go binary** for organizing Google Takeout photos. Works on Windows / macOS / Linux without WSL or dependencies.

---

## What It Does

The `migrate` command organizes photos from Google Takeout exports into a clean directory structure:

- **Scans** year folders (`Photos from XXXX`) in your Google Takeout export
- **Copies** files to organized output directories
- **Sets file timestamps** from JSON sidecar metadata (photoTakenTime or creationTime)
- **Organizes by device** (folders based on camera/phone model when available)
- **Generates metadata** SHA-256-based metadata JSON files for each photo
- **Logs decisions** to a per-run migration log

No external tools required — everything runs with Go stdlib.

---

## Installation

### Option 1: Download Precompiled Binary (Recommended)

Go to [Releases](https://github.com/bingzujia/google-takeout-time-helper/releases) and download for your platform:

| Platform | Filename |
|----------|----------|
| Windows (x64) | `takeout-helper-windows-amd64.exe` |
| macOS (Intel) | `takeout-helper-darwin-amd64` |
| macOS (Apple Silicon) | `takeout-helper-darwin-arm64` |
| Linux (x64) | `takeout-helper-linux-amd64` |

Make it executable (macOS / Linux):

```bash
chmod +x takeout-helper-darwin-arm64
# Optional: add to PATH
mv takeout-helper-darwin-arm64 /usr/local/bin/takeout-helper
```

### Option 2: Build from Source

```bash
git clone https://github.com/bingzujia/google-takeout-time-helper.git
cd google-takeout-time-helper
make build          # produces: bin/takeout-helper
```

---

## Usage

### Basic Migration

```bash
./takeout-helper migrate \
  --input-dir ~/Downloads/Takeout \
  --output-dir ~/Photos/Organized
```

### Dry-run (Preview Without Modifying)

```bash
./takeout-helper migrate \
  --input-dir ~/Downloads/Takeout \
  --output-dir ~/Photos/Organized \
  --dry-run
```

Output shows what would happen without making changes.

### Classification Modes

By default, files are organized by **year** (matching the input folder structure):

```bash
./takeout-helper migrate \
  --input-dir ~/Downloads/Takeout \
  --output-dir ~/Photos/Organized
```

Output structure:
```
Output/
├── Photos_from_2024/
│   ├── IMG_1234.jpg
│   ├── IMG_5678.jpg
│   └── ...
├── Photos_from_2023/
│   └── ...
└── metadata/
```

To organize by **device** (consolidating photos from the same device across multiple years), use `--classify-by-uploadFolder`:

```bash
./takeout-helper migrate \
  --input-dir ~/Downloads/Takeout \
  --output-dir ~/Photos/Organized \
  --classify-by-uploadFolder
```

Output structure:
```
Output/
├── Pixel 6/                 # device folder name (from JSON metadata)
│   ├── IMG_2024_001.jpg     # photos from all years, same device
│   ├── IMG_2023_456.jpg
│   └── ...
├── iPhone 13/               # another device
│   └── ...
└── metadata/
```

**Note:** Files without device metadata go to the Output root directory.

### Help

```bash
./takeout-helper migrate --help
```

---

## How It Works

### Input Structure

Google Takeout exports follow this structure:

```
Takeout/
├── Photos from 2015/
│   ├── IMG_1234.jpg
│   ├── IMG_1234.jpg.json
│   ├── DSC_5678.jpg
│   ├── DSC_5678.jpg.json
│   └── ...
├── Photos from 2016/
│   └── ...
└── ...
```

Each photo typically has:
- **Photo file** (JPG, PNG, MOV, MP4, etc.)
- **JSON sidecar** containing metadata: `photoTakenTime` (capture timestamp), `creationTime` (upload timestamp), `googlePhotosOrigin` (camera model), etc.

### Output Structure

The output structure depends on the classification mode (see [Classification Modes](#classification-modes) above).

**Default mode (year-based organization):**
```
Output/
├── Photos_from_2024/                 # year folder
│   ├── IMG_1234.jpg                  # migrated photos
│   ├── IMG_5678.jpg
│   └── ...
├── Photos_from_2023/                 # year folder
│   └── ...
├── metadata/                         # centralized metadata directory
│   ├── <SHA256_hash>.json            # photo metadata indexed by file SHA-256
│   └── ...
├── error/                            # files that failed to migrate
│   └── Photos from XXXX/
│       ├── IMG_error.jpg
│       └── IMG_error.jpg.json
├── manual_review/                    # files missing timestamps or requiring review
│   ├── Photos from XXXX/
│   │   └── IMG_review.jpg
│   └── metadata/
│       └── <SHA256_hash>.json
└── takeout-helper-log/
    └── migrate-YYYYMMDD-NNN.log      # migration log with per-file decisions
```

**Device-based mode (with `--classify-by-uploadFolder`):**
```
Output/
├── <localFolderName>/                # device folder from JSON metadata
│   ├── IMG_1234.jpg                  # migrated photos (from all years)
│   ├── IMG_5678.jpg
│   └── ...
├── Pixel 6/                          # another device
│   └── ...
├── metadata/                         # centralized metadata directory (same as default)
│   ├── <SHA256_hash>.json
│   └── ...
├── error/                            # files that failed to migrate
│   └── Photos from XXXX/
│       ├── IMG_error.jpg
│       └── IMG_error.jpg.json
├── manual_review/                    # files missing timestamps or requiring review
│   ├── Photos from XXXX/
│   │   └── IMG_review.jpg
│   └── metadata/
│       └── <SHA256_hash>.json
└── takeout-helper-log/
    └── migrate-YYYYMMDD-NNN.log
```

**Key details:**
- **<localFolderName>**: Value from JSON `googlePhotosOrigin.mobileUpload.deviceFolder.localFolderName` (e.g., "Pixel 6", "iPhone 13"). In device mode, if absent, files go to Output root.
- **<SHA256_hash>**: File SHA-256 hash used as metadata index (not photo filename).
- **error/** & **manual_review/**: Handle edge cases (missing timestamps, EXIF issues, etc.)
- **metadata/** directory is **always at Output root** — not affected by classification mode

### Timestamp Handling

File modification time is set using these priorities:

1. **JSON `photoTakenTime`** (photo capture time) — preferred
2. **JSON `creationTime`** (photo upload time) — fallback
3. **Manual review** — if both missing, file is moved to `manual_review/` for manual handling

No EXIF modification. The file's `ModifyTime` is set via `os.Chtimes()` (cross-platform, no external tools).

### What Gets Logged

Each migration produces a log at `takeout-helper-log/migrate-YYYYMMDD-NNN.log` with per-file decisions:

- `INFO`: File successfully migrated (with timestamp source: json/filename/none)
- `SKIP`: File already exists at destination
- `FAIL`: Error during migration (invalid path, permission, I/O error, etc.)
- Files moved to `manual_review/` (missing timestamps) are tracked in metadata but not counted in final summary

---

## Building

```bash
make build          # Compile binary → bin/takeout-helper
make test           # Run all tests
make lint           # Run go vet
make clean          # Remove bin/
```

---

## Requirements

- **Go 1.20+** (for building from source)
- No external tool dependencies for migration

---

## Troubleshooting

### Migration skips files

Check the migration log (printed at end of migration) for reasons. Common causes:

- File already exists at destination
- Missing JSON sidecar for timestamp
- Invalid filename or path issues

### Wrong timestamps

Verify the JSON sidecars contain `photoTakenTime` or `creationTime` fields. Check migration log for which source was used.

### Permission denied errors

Ensure you have write access to the output directory and read access to the input directory.

---

## Development

See [docs/developer-guide.md](docs/developer-guide.md) for architecture and implementation details.

---

## License

See LICENSE file.
