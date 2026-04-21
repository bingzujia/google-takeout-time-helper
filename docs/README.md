# Documentation Index

This documentation provides guidance for using and developing the `takeout-helper` CLI tool.

## For End Users

### [Commands Reference](./commands.md)
Complete documentation for all CLI commands including:
- **migrate** — Migrate Google Takeout photos with EXIF metadata restoration
- **classify** — Classify media files by camera, screenshot, WeChat, or other categories
- **convert** — Convert images to HEIC format with configurable quality
- **fix-exif** — Sync timestamps from JSON metadata to EXIF tags
- **fix-name** — Sync filename datetime to EXIF DateTimeOriginal
- **dedup** — Find and group duplicate images using perceptual hashing
- **rename** — Rename media files using timestamp-based conventions
- **rename-screenshot** — Rename screenshots with extracted timestamp
- **format-qq-media** — Format QQ-exported media with standardized naming

Each command documentation includes:
- Purpose and typical use cases
- All available flags with descriptions
- Practical examples and workflows
- Notes on metadata handling

### [Architecture Overview](./architecture.md)
High-level system design covering:
- How commands delegate to internal packages
- Data flows through the system (metadata, images, hashes)
- Extension points for adding new features
- Project conventions (logging, error handling, concurrency, testing)
- Optional dependencies and graceful degradation

## For Developers

### [Module Guide](./modules.md)
Reference documentation for all 18 internal packages:
- **matcher** — JSON sidecar matching with 6-step degradation strategy
- **migrator** — Core migration pipeline with metadata restoration
- **parser** — Timestamp and GPS parsing from EXIF and filenames
- **heicconv** — HEIC encoding with quality tuning and metadata copy
- **exifrunner** — Batch exiftool operations with graceful degradation
- **workerpool** — Generic goroutine pool with CPU capping
- **logutil** — Thread-safe file logging to takeout-helper-log/
- **organizer** — Year-folder classification
- **classifier** — Media classification by bucket rules
- **mediatype** — Content-based media type detection
- **dedup** — Perceptual deduplication using pHash/dHash
- **hashcache** — SQLite-backed hash value caching
- **destlocker** — Concurrent write prevention
- **renamer** — Timestamp-based filename generation with burst detection
- **progress** — Terminal progress bar
- **fileutil** — File operation helpers
- **qqmedia** — QQ media format handling
- **screenshotter** — Screenshot detection

Each package documentation includes:
- Role and responsibility in the system
- Key structs and function signatures
- Extension points and configuration options
- Example usage patterns

### [Developer Guide](./developer-guide.md)
Step-by-step guidance for development including:
- How to add new CLI commands
- How to extend existing internal packages
- Testing patterns and conventions
- Common development tasks (build, test, lint)
- Debugging and profiling techniques
- Project memory and architectural quirks

## Documentation Structure

```
docs/
├── README.md                 # This file
├── commands.md              # CLI command reference (end-users)
├── modules.md               # Internal package guide (developers)
├── architecture.md          # System design and extensibility (developers)
├── developer-guide.md       # Development workflows (developers)
└── archive/
    └── analysis/            # Historical design notes (reference only)
```

## Quick Links by Audience

**Getting started?**
- Read [commands.md](./commands.md) to understand available CLI options
- Read the [Architecture Overview](./architecture.md) to see how commands work internally

**Extending the tool?**
- Read [developer-guide.md](./developer-guide.md) for step-by-step instructions
- Consult [modules.md](./modules.md) for package APIs and responsibilities
- Reference [architecture.md](./architecture.md) for conventions and data flows

**Debugging?**
- See [developer-guide.md](./developer-guide.md#debugging-and-profiling) for debugging techniques
- Review specific package documentation in [modules.md](./modules.md) for implementation details
- Check [architecture.md](./architecture.md#conventions) for logging and error handling conventions

## Historical Context

The `docs/archive/analysis/` folder contains historical design notes and analysis from earlier development phases. These are preserved for reference but not actively maintained. Use Git history for detailed implementation changes.

## Updating Documentation

When updating source code:
- Update CLI command docs if flags change
- Update module docs if package APIs change
- Update architecture docs if command delegation or data flows change
- Keep documentation focused on contracts (what commands do, what packages provide) rather than implementation details — implementation details belong in source code comments
