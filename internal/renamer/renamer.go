package renamer

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/bingzujia/google-takeout-time-helper/internal/logutil"
	"github.com/bingzujia/google-takeout-time-helper/internal/mediatype"
)

// Config holds renamer settings.
type Config struct {
	Dir    string
	DryRun bool
	Logger *logutil.Logger // structured log; if nil a Nop logger is used
}

// Result holds counts after a Run.
type Result struct {
	Renamed int
	Skipped int
	Errors  int
}

// burstRe matches filenames like 20190207_184125_007.jpg
var burstRe = regexp.MustCompile(`^(\d{8}_\d{6})_(\d{3})\.(\w+)$`)

// Screenshot timestamp format regexes (6 formats)
var (
	// Format 1: YYYY-MM-DD-HH-MM-SS-MS (e.g., "Screenshot_2025-07-18-09-23-54-65.png")
	screenshotTimestampRe1 = regexp.MustCompile(`(\d{4})-(\d{2})-(\d{2})-(\d{2})-(\d{2})-(\d{2})-(\d{2})`)
	// Format 2: YYYYMMDD_HHMMSS (e.g., "screenshot20250718_092354.jpg")
	screenshotTimestampRe2 = regexp.MustCompile(`(\d{8})_(\d{6})`)
	// Format 3: YYYY-MM-DD_HH-MM-SS (e.g., "Screenshot_2025-07-18_09-23-54.png")
	screenshotTimestampRe3 = regexp.MustCompile(`(\d{4})-(\d{2})-(\d{2})_(\d{2})-(\d{2})-(\d{2})`)
	// Format 4: YYYY_M_D_H_M_S with auto zero-padding (e.g., "screenshot_2025_7_18_9_23_54.png")
	screenshotTimestampRe4 = regexp.MustCompile(`(\d{4})_(\d{1,2})_(\d{1,2})_(\d{1,2})_(\d{1,2})_(\d{1,2})`)
	// Format 5: YYYY-MM-DD (date only, e.g., "screenshot_2025-07-18.png")
	screenshotTimestampRe5 = regexp.MustCompile(`(\d{4})-(\d{2})-(\d{2})(?:[^0-9]|$)`)
	// Format 6a: Unix timestamp seconds (10 digits, e.g., "screenshot1634560000.jpg")
	screenshotTimestampRe6a = regexp.MustCompile(`(\d{10})(?:\.|$)`)
	// Format 6b: Unix timestamp milliseconds (13 digits, e.g., "mmscreenshot1727421404387.jpg")
	screenshotTimestampRe6b = regexp.MustCompile(`(\d{13})(?:\.|$)`)
)

type burstFile struct {
	name string
	seq  string
	ext  string
}

// buildName generates the target filename (including extension) for a normal file.
//   - HEIC/HEIF:        IMG{YYYYMMDD}{HHMMSS}.{ext}
//   - Other images:     IMG_{YYYYMMDD}_{HHMMSS}.{ext}
//   - Standalone video: VID{YYYYMMDD}{HHMMSS}.{ext}
func buildName(ext string, t time.Time) string {
	date := t.Format("20060102")
	tp := t.Format("150405")
	if mediatype.IsHEIC(ext) {
		return fmt.Sprintf("IMG%s%s.%s", date, tp, ext)
	}
	if mediatype.IsVideo(ext) {
		return fmt.Sprintf("VID%s%s.%s", date, tp, ext)
	}
	return fmt.Sprintf("IMG_%s_%s.%s", date, tp, ext)
}

// buildBurstName generates the target filename for a burst file at the given index.
//   - HEIC/HEIF: IMG{YYYYMMDD}{HHMMSS}_BURST{NNN}.{ext}
//   - Others:    IMG_{YYYYMMDD}_{HHMMSS}_BURST{NNN}.{ext}
//
// dateTime must be in the form "YYYYMMDD_HHMMSS".
func buildBurstName(ext, dateTime string, idx int) string {
	parts := strings.SplitN(dateTime, "_", 2)
	date, tp := parts[0], parts[1]
	burst := fmt.Sprintf("BURST%03d", idx)
	if mediatype.IsHEIC(ext) {
		return fmt.Sprintf("IMG%s%s_%s.%s", date, tp, burst, ext)
	}
	return fmt.Sprintf("IMG_%s_%s_%s.%s", date, tp, burst, ext)
}

// nonConflictName returns a filename (base.ext, base_001.ext, …) that does not
// currently exist in dir.  Returns "" if no candidate is found within 999 tries.
func nonConflictName(dir, base, ext string) string {
	candidate := base + "." + ext
	if _, err := os.Stat(filepath.Join(dir, candidate)); os.IsNotExist(err) {
		return candidate
	}
	for i := 1; i < 1000; i++ {
		candidate = fmt.Sprintf("%s_%03d.%s", base, i, ext)
		if _, err := os.Stat(filepath.Join(dir, candidate)); os.IsNotExist(err) {
			return candidate
		}
	}
	return ""
}

// detectBurstGroups scans entries for image files matching the burst pattern
// (YYYYMMDD_HHMMSS_NNN.ext) and returns groups keyed by "YYYYMMDD_HHMMSS".
// Only groups with ≥2 files are included.
func detectBurstGroups(entries []os.DirEntry) map[string][]burstFile {
	groups := make(map[string][]burstFile)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
		if !mediatype.IsImage(ext) {
			continue
		}
		m := burstRe.FindStringSubmatch(name)
		if m == nil {
			continue
		}
		key := m[1]
		groups[key] = append(groups[key], burstFile{name: name, seq: m[2], ext: ext})
	}
	for k, v := range groups {
		if len(v) < 2 {
			delete(groups, k)
		}
	}
	return groups
}

// detectMp4Pairs returns a map of image-base-name → MP4 filename for every MP4
// that shares a base name (without extension) with an image in the directory.
func detectMp4Pairs(entries []os.DirEntry) map[string]string {
	imageNames := make(map[string]bool)
	mp4s := make(map[string]string) // base → filename

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		extWithDot := filepath.Ext(name)
		ext := strings.ToLower(strings.TrimPrefix(extWithDot, "."))
		base := name[:len(name)-len(extWithDot)]
		if mediatype.IsImage(ext) {
			imageNames[base] = true
		} else if ext == "mp4" {
			mp4s[base] = name
		}
	}

	pairs := make(map[string]string)
	for base, mp4Name := range mp4s {
		if imageNames[base] {
			pairs[base] = mp4Name
		}
	}
	return pairs
}

// stem returns the filename without its extension.
func stem(name string) string {
	return name[:len(name)-len(filepath.Ext(name))]
}

// detectScreenshots scans entries for screenshot files and returns a map
// of screenshot filenames keyed by their names for quick lookup.
// Files whose names (case-insensitive) contain "screenshot" are included.
// This includes variations like "mmscreenshot", "wxscreenshot", "screenshot", etc.
// Unlike other file types, screenshot detection does not require a valid image extension.
func detectScreenshots(entries []os.DirEntry) map[string]bool {
	screenshots := make(map[string]bool)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		lowerName := strings.ToLower(name)
		// Match any file that contains "screenshot" (includes variations like mmscreenshot, wxscreenshot, etc.)
		if strings.Contains(lowerName, "screenshot") {
			screenshots[name] = true
		}
	}
	return screenshots
}

// parseScreenshotTimestamp extracts the timestamp from a screenshot filename.
// It tries 6 formats in order and returns (time, ok) where ok indicates if the
// timestamp is complete (all time components present). For Format 5 (date-only),
// ok=false to indicate a partial timestamp that may need ModTime fallback.
//
// Supported formats:
// 1. YYYY-MM-DD-HH-MM-SS-MS (e.g., "Screenshot_2025-07-18-09-23-54-65.png")
// 2. YYYYMMDD_HHMMSS (e.g., "screenshot20250718_092354.jpg")
// 3. YYYY-MM-DD_HH-MM-SS (e.g., "Screenshot_2025-07-18_09-23-54.png")
// 4. YYYY_M_D_H_M_S (e.g., "screenshot_2025_7_18_9_23_54.png")
// 5. YYYY-MM-DD (e.g., "screenshot_2025-07-18.png")
// 6a. Unix timestamp seconds (e.g., "screenshot1634560000.jpg")
// 6b. Unix timestamp milliseconds (e.g., "mmscreenshot1727421404387.jpg")
func parseScreenshotTimestamp(filename string) (time.Time, bool) {
	// Format 1: YYYY-MM-DD-HH-MM-SS-MS
	if m := screenshotTimestampRe1.FindStringSubmatch(filename); m != nil {
		year := atoi(m[1])
		month := atoi(m[2])
		day := atoi(m[3])
		hour := atoi(m[4])
		min := atoi(m[5])
		sec := atoi(m[6])
		ms := atoi(m[7])
		t := time.Date(year, time.Month(month), day, hour, min, sec, ms*10_000_000, time.UTC)
		return t, true
	}

	// Format 2: YYYYMMDD_HHMMSS
	if m := screenshotTimestampRe2.FindStringSubmatch(filename); m != nil {
		dateStr := m[1]
		timeStr := m[2]
		year := atoi(dateStr[:4])
		month := atoi(dateStr[4:6])
		day := atoi(dateStr[6:8])
		hour := atoi(timeStr[:2])
		min := atoi(timeStr[2:4])
		sec := atoi(timeStr[4:6])
		t := time.Date(year, time.Month(month), day, hour, min, sec, 0, time.UTC)
		return t, true
	}

	// Format 3: YYYY-MM-DD_HH-MM-SS
	if m := screenshotTimestampRe3.FindStringSubmatch(filename); m != nil {
		year := atoi(m[1])
		month := atoi(m[2])
		day := atoi(m[3])
		hour := atoi(m[4])
		min := atoi(m[5])
		sec := atoi(m[6])
		t := time.Date(year, time.Month(month), day, hour, min, sec, 0, time.UTC)
		return t, true
	}

	// Format 4: YYYY_M_D_H_M_S (auto zero-padding)
	if m := screenshotTimestampRe4.FindStringSubmatch(filename); m != nil {
		year := atoi(m[1])
		month := atoi(m[2])
		day := atoi(m[3])
		hour := atoi(m[4])
		min := atoi(m[5])
		sec := atoi(m[6])
		t := time.Date(year, time.Month(month), day, hour, min, sec, 0, time.UTC)
		return t, true
	}

	// Format 5: YYYY-MM-DD (date only, incomplete)
	if m := screenshotTimestampRe5.FindStringSubmatch(filename); m != nil {
		year := atoi(m[1])
		month := atoi(m[2])
		day := atoi(m[3])
		t := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
		return t, false // incomplete: ok=false
	}

	// Format 6b: Unix timestamp milliseconds (13 digits, e.g., "mmscreenshot1727421404387.jpg")
	if m := screenshotTimestampRe6b.FindStringSubmatch(filename); m != nil {
		timestampMs := atoi64(m[1])
		unixSec := timestampMs / 1000
		nanoFrac := (timestampMs % 1000) * 1_000_000
		t := time.Unix(unixSec, nanoFrac).UTC()
		return t, true
	}

	// Format 6a: Unix timestamp seconds (10 digits, e.g., "screenshot1634560000.jpg")
	if m := screenshotTimestampRe6a.FindStringSubmatch(filename); m != nil {
		unixSec := int64(atoi(m[1]))
		t := time.Unix(unixSec, 0).UTC()
		return t, true
	}

	return time.Time{}, false
}

// buildScreenshotName generates the target filename for a screenshot.
// Format: Screenshot_YYYY-MM-DD-HH-MM-SS-MS.{ext}
// The millisecond component (MS) is derived from the nanosecond value: ns / 10_000_000
// (which yields a value 0-99 representing hundredths and tenths of a second).
func buildScreenshotName(ext string, t time.Time) string {
	ext = strings.ToLower(ext)
	date := t.Format("2006-01-02")
	timeStr := t.Format("15-04-05")
	ms := t.Nanosecond() / 10_000_000
	if ms > 99 {
		ms = 99 // clamp to 99
	}
	return fmt.Sprintf("Screenshot_%s-%s-%02d.%s", date, timeStr, ms, ext)
}

// atoi is a helper to convert a string to int (used by timestamp parsing).
func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// atoi64 is a helper to convert a string to int64 (used for Unix timestamp parsing).
func atoi64(s string) int64 {
	var n int64
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int64(c-'0')
	}
	return n
}

// doRename performs (or previews) a single rename operation.
func doRename(dir, oldName, newName string, dryRun bool) error {
	if dryRun {
		fmt.Printf("  %s -> %s\n", oldName, newName)
		return nil
	}
	return os.Rename(filepath.Join(dir, oldName), filepath.Join(dir, newName))
}

// Run performs smart renaming in two phases:
//
// Phase 1 (scan): classify entries into burst groups, MP4 companions, and normal files.
// Phase 2 (rename): apply the appropriate naming rule to each category.
func Run(cfg Config) (Result, error) {
	if cfg.Logger == nil {
		cfg.Logger = logutil.Nop()
	}

	entries, err := os.ReadDir(cfg.Dir)
	if err != nil {
		return Result{}, fmt.Errorf("read dir: %w", err)
	}

	burstGroups := detectBurstGroups(entries)
	mp4Pairs := detectMp4Pairs(entries)
	screenshotNames := detectScreenshots(entries)

	// Build skip sets used in Phase 2b.
	burstNames := make(map[string]bool)
	for _, files := range burstGroups {
		for _, f := range files {
			burstNames[f.name] = true
		}
	}
	mp4Companions := make(map[string]bool)
	for _, mp4Name := range mp4Pairs {
		mp4Companions[mp4Name] = true
	}

	var result Result

	// ── Phase 2a: burst groups ───────────────────────────────────────────────
	for dateTime, files := range burstGroups {
		sort.Slice(files, func(i, j int) bool { return files[i].seq < files[j].seq })

		for idx, f := range files {
			ideal := buildBurstName(f.ext, dateTime, idx)
			newName := nonConflictName(cfg.Dir, stem(ideal), f.ext)
			if newName == "" {
				result.Errors++
				cfg.Logger.Fail("rename", filepath.Join(cfg.Dir, f.name), "no non-conflict name found")
				continue
			}

			if err := doRename(cfg.Dir, f.name, newName, cfg.DryRun); err != nil {
				result.Errors++
				cfg.Logger.Fail("rename", filepath.Join(cfg.Dir, f.name), err.Error())
				continue
			}
			cfg.Logger.Info("renamed", filepath.Join(cfg.Dir, f.name))
			result.Renamed++

			// Rename the paired MP4 companion (same burst index).
			if mp4Name, ok := mp4Pairs[stem(f.name)]; ok {
				newMp4 := nonConflictName(cfg.Dir, stem(newName), "mp4")
				if newMp4 == "" {
					result.Errors++
					cfg.Logger.Fail("rename", filepath.Join(cfg.Dir, mp4Name), "no non-conflict name found")
					continue
				}
				if err := doRename(cfg.Dir, mp4Name, newMp4, cfg.DryRun); err != nil {
					result.Errors++
					cfg.Logger.Fail("rename", filepath.Join(cfg.Dir, mp4Name), err.Error())
				} else {
					cfg.Logger.Info("renamed", filepath.Join(cfg.Dir, mp4Name))
					result.Renamed++
				}
			}
		}
	}

	// ── Phase 2b: normal files ───────────────────────────────────────────────
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if burstNames[name] || mp4Companions[name] {
			continue
		}

		extWithDot := filepath.Ext(name)
		ext := strings.ToLower(strings.TrimPrefix(extWithDot, "."))

		// Screenshot handling (Phase 2b)
		if screenshotNames[name] {
			// Parse screenshot timestamp from filename, fallback to ModTime
			t, ok := parseScreenshotTimestamp(name)
			if !ok {
				// Incomplete timestamp (e.g., date only), use ModTime for time portion
				info, err := e.Info()
				if err != nil {
					result.Errors++
					cfg.Logger.Fail("stat", filepath.Join(cfg.Dir, name), err.Error())
					continue
				}
				modTime := info.ModTime()
				t = time.Date(t.Year(), t.Month(), t.Day(), modTime.Hour(), modTime.Minute(), modTime.Second(), modTime.Nanosecond(), time.UTC)
			}

			ideal := buildScreenshotName(ext, t)

			// Already has the ideal name → skip
			if ideal == name {
				result.Skipped++
				cfg.Logger.Skip("already-named", filepath.Join(cfg.Dir, name))
				continue
			}

			newName := nonConflictName(cfg.Dir, stem(ideal), ext)
			if newName == "" {
				result.Errors++
				cfg.Logger.Fail("rename", filepath.Join(cfg.Dir, name), "no non-conflict name found")
				continue
			}
			if newName == name {
				result.Skipped++
				cfg.Logger.Skip("already-named", filepath.Join(cfg.Dir, name))
				continue
			}

			if err := doRename(cfg.Dir, name, newName, cfg.DryRun); err != nil {
				result.Errors++
				cfg.Logger.Fail("rename", filepath.Join(cfg.Dir, name), err.Error())
				continue
			}
			cfg.Logger.Info("renamed", filepath.Join(cfg.Dir, name))
			result.Renamed++

			// Rename the paired MP4 companion if it exists
			if mp4Name, ok := mp4Pairs[stem(name)]; ok {
				newMp4 := nonConflictName(cfg.Dir, stem(newName), "mp4")
				if newMp4 == "" {
					result.Errors++
					cfg.Logger.Fail("rename", filepath.Join(cfg.Dir, mp4Name), "no non-conflict name found")
					continue
				}
				if err := doRename(cfg.Dir, mp4Name, newMp4, cfg.DryRun); err != nil {
					result.Errors++
					cfg.Logger.Fail("rename", filepath.Join(cfg.Dir, mp4Name), err.Error())
				} else {
					cfg.Logger.Info("renamed", filepath.Join(cfg.Dir, mp4Name))
					result.Renamed++
				}
			}

			continue // Skip IMG/VID logic for screenshots
		}

		if !mediatype.IsImage(ext) && !mediatype.IsVideo(ext) {
			continue
		}

		info, err := e.Info()
		if err != nil {
			result.Errors++
			cfg.Logger.Fail("stat", filepath.Join(cfg.Dir, name), err.Error())
			continue
		}

		ideal := buildName(ext, info.ModTime())

		// Already has the ideal name → skip (also avoids false conflict on re-run).
		if ideal == name {
			result.Skipped++
			cfg.Logger.Skip("already-named", filepath.Join(cfg.Dir, name))
			continue
		}

		newName := nonConflictName(cfg.Dir, stem(ideal), ext)
		if newName == "" {
			result.Errors++
			cfg.Logger.Fail("rename", filepath.Join(cfg.Dir, name), "no non-conflict name found")
			continue
		}
		if newName == name {
			result.Skipped++
			cfg.Logger.Skip("already-named", filepath.Join(cfg.Dir, name))
			continue
		}

		if err := doRename(cfg.Dir, name, newName, cfg.DryRun); err != nil {
			result.Errors++
			cfg.Logger.Fail("rename", filepath.Join(cfg.Dir, name), err.Error())
			continue
		}
		cfg.Logger.Info("renamed", filepath.Join(cfg.Dir, name))
		result.Renamed++

		// Rename the paired MP4 companion.
		if mp4Name, ok := mp4Pairs[stem(name)]; ok {
			newMp4 := nonConflictName(cfg.Dir, stem(newName), "mp4")
			if newMp4 == "" {
				result.Errors++
				cfg.Logger.Fail("rename", filepath.Join(cfg.Dir, mp4Name), "no non-conflict name found")
				continue
			}
			if err := doRename(cfg.Dir, mp4Name, newMp4, cfg.DryRun); err != nil {
				result.Errors++
				cfg.Logger.Fail("rename", filepath.Join(cfg.Dir, mp4Name), err.Error())
			} else {
				cfg.Logger.Info("renamed", filepath.Join(cfg.Dir, mp4Name))
				result.Renamed++
			}
		}
	}

	return result, nil
}

