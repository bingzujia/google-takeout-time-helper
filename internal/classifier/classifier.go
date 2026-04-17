package classifier

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/bingzujia/g_photo_take_out_helper/internal/destlocker"
	"github.com/bingzujia/g_photo_take_out_helper/internal/exifrunner"
	"github.com/bingzujia/g_photo_take_out_helper/internal/fileutil"
	"github.com/bingzujia/g_photo_take_out_helper/internal/logutil"
	"github.com/bingzujia/g_photo_take_out_helper/internal/organizer"
	"github.com/bingzujia/g_photo_take_out_helper/internal/progress"
	"github.com/bingzujia/g_photo_take_out_helper/internal/workerpool"
)

// Category is the destination bucket for a classified file.
type Category string

const (
	CategoryCamera      Category = "camera"
	CategoryScreenshot  Category = "screenshot"
	CategoryWechat      Category = "wechat"
	CategorySeemsCamera Category = "seemsCamera"
)

// Config holds settings for a classify run.
type Config struct {
	InputDir     string
	OutputDir    string
	DryRun       bool
	ShowProgress bool
	Logger       *logutil.Logger // structured log; if nil a Nop logger is used
}

// Result holds counts after a Run.
type Result struct {
	Camera      int
	Screenshot  int
	Wechat      int
	SeemsCamera int
	Skipped     int
}

// Run classifies media files from the root of cfg.InputDir and moves them into
// category subdirectories under cfg.OutputDir.
func Run(cfg Config) (Result, error) {
	var result Result

	if cfg.Logger == nil {
		cfg.Logger = logutil.Nop()
	}

	files, err := scanEligibleFiles(cfg.InputDir)
	if err != nil {
		return result, fmt.Errorf("scan input dir: %w", err)
	}
	if len(files) == 0 {
		return result, nil
	}

	// Pre-query Make/Model for all files in a single batched exiftool call.
	// Files that can't be classified by filename will use this map as fallback.
	paths := make([]string, len(files))
	for i, f := range files {
		paths[i] = f.Path
	}
	exifResults, _ := exifrunner.BatchQuery(paths, []string{"Make", "Model"})
	exifMap := make(map[string]exifDeviceOutput, len(files))
	for i, path := range paths {
		if exifResults != nil && i < len(exifResults) && exifResults[i] != nil {
			exifMap[path] = exifDeviceOutput{
				Make:  exifResults[i]["Make"],
				Model: exifResults[i]["Model"],
			}
		}
	}

	return runParallel(files, cfg, exifMap)
}

type fileJob struct {
	Name string
	Path string
}

func scanEligibleFiles(inputDir string) ([]fileJob, error) {
	entries, err := os.ReadDir(inputDir)
	if err != nil {
		return nil, err
	}

	var files []fileJob
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		files = append(files, fileJob{
			Name: e.Name(),
			Path: filepath.Join(inputDir, e.Name()),
		})
	}
	return files, nil
}

func runParallel(files []fileJob, cfg Config, exifMap map[string]exifDeviceOutput) (Result, error) {
	var result Result
	var resultMu sync.Mutex
	locker := destlocker.New()

	reporter := progress.NewReporter(len(files), cfg.ShowProgress)
	defer reporter.Close()

	var done int
	var doneMu sync.Mutex

	err := workerpool.Run(files, workerpool.DefaultWorkers(), func(job fileJob) error {
		err := processFile(job, cfg, exifMap, &result, &resultMu, locker)
		doneMu.Lock()
		done++
		reporter.Update(done)
		doneMu.Unlock()
		return err
	})

	return result, err
}

func processFile(job fileJob, cfg Config, exifMap map[string]exifDeviceOutput, result *Result, resultMu *sync.Mutex, locker *destlocker.Locker) error {
	cat, ok := classifyFile(job.Name)
	if !ok {
		if hasCam := exifMapFallback(job.Path, exifMap); hasCam {
			cat = CategorySeemsCamera
		} else {
			resultMu.Lock()
			result.Skipped++
			resultMu.Unlock()
			cfg.Logger.Skip("no-category-match", job.Path)
			return nil
		}
	}

	return moveToCategory(job.Path, job.Name, cfg.OutputDir, cat, cfg.DryRun, result, resultMu, locker, cfg.Logger)
}

// classifyFile maps organizer filename rules to a Category.
func classifyFile(name string) (Category, bool) {
	mode, ok := organizer.Classify(name)
	if !ok {
		return "", false
	}
	switch mode {
	case organizer.ModeCamera:
		return CategoryCamera, true
	case organizer.ModeScreenshot:
		return CategoryScreenshot, true
	case organizer.ModeWechat:
		return CategoryWechat, true
	default:
		return "", false
	}
}

// exifDeviceOutput mirrors the exiftool JSON output for Make/Model tags.
type exifDeviceOutput struct {
	Make  string `json:"Make"`
	Model string `json:"Model"`
}

// exifMapFallback returns true if the pre-queried exif map has a non-empty Make or Model for path.
func exifMapFallback(path string, exifMap map[string]exifDeviceOutput) bool {
	d, ok := exifMap[path]
	if !ok {
		return false
	}
	return strings.TrimSpace(d.Make) != "" || strings.TrimSpace(d.Model) != ""
}

// moveToCategory moves src into <outputDir>/<category>/, respecting dry-run mode.
func moveToCategory(src, name, outputDir string, cat Category, dryRun bool, result *Result, resultMu *sync.Mutex, locker *destlocker.Locker, logger *logutil.Logger) error {
	destDir := filepath.Join(outputDir, string(cat))
	destPath := fileutil.ResolveDestPath(destDir, name)
	if dryRun {
		progress.Info("  [dry-run] %s  →  %s/%s", src, string(cat), name)
		resultMu.Lock()
		incrementResult(result, cat)
		resultMu.Unlock()
		return nil
	}

	unlock := locker.Lock(destDir)
	defer unlock()

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("create dest dir %s: %w", destDir, err)
	}

	if err := os.Rename(src, destPath); err != nil {
		// Try copy+delete for cross-device moves.
		if err2 := fileutil.CopyFile(src, destPath); err2 != nil {
			resultMu.Lock()
			result.Skipped++
			resultMu.Unlock()
			logger.Fail("move-failed", src, err2.Error())
			return nil
		}
		os.Remove(src)
	}
	logger.Info(string(cat), src)
	resultMu.Lock()
	incrementResult(result, cat)
	resultMu.Unlock()
	return nil
}

func incrementResult(r *Result, cat Category) {
	switch cat {
	case CategoryCamera:
		r.Camera++
	case CategoryScreenshot:
		r.Screenshot++
	case CategoryWechat:
		r.Wechat++
	case CategorySeemsCamera:
		r.SeemsCamera++
	}
}
