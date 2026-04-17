package renamer

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/bingzujia/g_photo_take_out_helper/internal/logutil"
	"github.com/bingzujia/g_photo_take_out_helper/internal/mediatype"
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

