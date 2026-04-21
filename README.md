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

```
Output/
├── camera-name/              # organized by device
│   ├── metadata/
│   │   ├── IMG_1234.json     # photo metadata with timestamp source
│   │   └── ...
│   ├── IMG_1234.jpg          # migrated photo
│   └── ...
└── takeout-helper-log/
    └── migrate-YYYYMMDD-NNN.log  # migration log
```

### Timestamp Handling

File modification time is set using these priorities:

1. **JSON `photoTakenTime`** (photo capture time) — preferred
2. **JSON `creationTime`** (photo upload time) — fallback
3. **Manual review** — if both missing, file is flagged for manual handling

No EXIF modification. The file's `ModifyTime` is what gets changed via `os.Chtimes()`.

### What Gets Logged

Each migration produces a log with per-file decisions:

- `INFO`: File successfully migrated (with timestamp source)
- `SKIP`: File already exists at destination
- `FAIL`: Error during migration (invalid path, permission, etc.)

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
