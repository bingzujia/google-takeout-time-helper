package classifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bingzujia/g_photo_take_out_helper/internal/organizer"
	"github.com/bingzujia/g_photo_take_out_helper/internal/progress"
)

var (
	exiftoolPathOnce  sync.Once
	exiftoolPath      string
	exiftoolAvailable bool
	exiftoolWarnOnce  sync.Once
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

	files, err := scanEligibleFiles(cfg.InputDir)
	if err != nil {
		return result, fmt.Errorf("scan input dir: %w", err)
	}
	if len(files) == 0 {
		return result, nil
	}

	return runParallel(files, cfg)
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

func runParallel(files []fileJob, cfg Config) (Result, error) {
	var result Result

	workers := runtime.NumCPU()
	if workers > 8 {
		workers = 8
	}
	if workers < 1 {
		workers = 1
	}

	var wg sync.WaitGroup
	var resultMu sync.Mutex
	var completed atomic.Int64
	var firstErr error
	var errOnce sync.Once
	locker := newDestinationLocker()

	reporter := progress.NewReporter(len(files), cfg.ShowProgress)
	defer reporter.Close()

	jobCh := make(chan fileJob, workers)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobCh {
				if err := processFile(job, cfg, &result, &resultMu, locker); err != nil {
					errOnce.Do(func() {
						firstErr = err
					})
				}
				reporter.Update(int(completed.Add(1)))
			}
		}()
	}

	for _, job := range files {
		jobCh <- job
	}
	close(jobCh)
	wg.Wait()

	return result, firstErr
}

func processFile(job fileJob, cfg Config, result *Result, resultMu *sync.Mutex, locker *destinationLocker) error {
	cat, ok := classifyFile(job.Name)
	if !ok {
		hasCam, _ := exiftoolFallback(job.Path)
		if hasCam {
			cat = CategorySeemsCamera
		} else {
			resultMu.Lock()
			result.Skipped++
			resultMu.Unlock()
			return nil
		}
	}

	return moveToCategory(job.Path, job.Name, cfg.OutputDir, cat, cfg.DryRun, result, resultMu, locker)
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

// exiftoolFallback returns true if the file's EXIF Make or Model tag is non-empty.
// Returns (false, nil) gracefully when exiftool is not installed or the command fails.
func exiftoolFallback(path string) (bool, error) {
	cmdPath, ok := lookupExiftool()
	if !ok {
		exiftoolWarnOnce.Do(func() {
			progress.Warning("exiftool not found, skipping EXIF fallback")
		})
		return false, nil
	}

	cmd := exec.Command(cmdPath, "-Make", "-Model", "-j", path)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return false, nil
	}

	var results []exifDeviceOutput
	if err := json.Unmarshal(stdout.Bytes(), &results); err != nil || len(results) == 0 {
		return false, nil
	}
	r := results[0]
	return strings.TrimSpace(r.Make) != "" || strings.TrimSpace(r.Model) != "", nil
}

// moveToCategory moves src into <outputDir>/<category>/, respecting dry-run mode.
func moveToCategory(src, name, outputDir string, cat Category, dryRun bool, result *Result, resultMu *sync.Mutex, locker *destinationLocker) error {
	destDir := filepath.Join(outputDir, string(cat))

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

	destPath := resolveDestPath(destDir, name)
	if err := os.Rename(src, destPath); err != nil {
		// Try copy+delete for cross-device moves.
		if err2 := copyFile(src, destPath); err2 != nil {
			resultMu.Lock()
			result.Skipped++
			resultMu.Unlock()
			return nil
		}
		os.Remove(src)
	}
	resultMu.Lock()
	incrementResult(result, cat)
	resultMu.Unlock()
	return nil
}

type destinationLocker struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

func newDestinationLocker() *destinationLocker {
	return &destinationLocker{locks: make(map[string]*sync.Mutex)}
}

func (d *destinationLocker) Lock(destDir string) func() {
	d.mu.Lock()
	lock, ok := d.locks[destDir]
	if !ok {
		lock = &sync.Mutex{}
		d.locks[destDir] = lock
	}
	d.mu.Unlock()

	lock.Lock()
	return lock.Unlock
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

func resolveDestPath(destDir, name string) string {
	target := filepath.Join(destDir, name)
	if _, err := os.Stat(target); os.IsNotExist(err) {
		return target
	}
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	suffix := time.Now().Format("20060102150405")
	return filepath.Join(destDir, fmt.Sprintf("%s_%s%s", stem, suffix, ext))
}

func copyFile(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	if err = out.Close(); err != nil {
		return err
	}
	return os.Chtimes(dst, info.ModTime(), info.ModTime())
}

func lookupExiftool() (string, bool) {
	exiftoolPathOnce.Do(func() {
		path, err := exec.LookPath("exiftool")
		if err == nil {
			exiftoolPath = path
			exiftoolAvailable = true
		}
	})
	return exiftoolPath, exiftoolAvailable
}
