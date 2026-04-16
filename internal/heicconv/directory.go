package heicconv

import (
	"errors"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/bingzujia/g_photo_take_out_helper/internal/progress"
)

// Config controls a root-level directory HEIC conversion run.
type Config struct {
	InputDir     string
	DryRun       bool
	ShowProgress bool
	Workers      int
	Converter    *Converter
	Infof        func(format string, args ...any)
	Warnf        func(format string, args ...any)
	Errorf       func(format string, args ...any)
}

// Stats summarizes a directory HEIC conversion run.
type Stats struct {
	Scanned            int
	Planned            int
	Converted          int
	RenamedExtensions  int
	SkippedUnsupported int
	SkippedAlreadyHEIC int
	SkippedConflicts   int
	Failed             int
	Failures           []Failure
	Conflicts          []Conflict
}

// Failure records a file-level failure that did not stop the overall run.
type Failure struct {
	Path string
	Err  error
}

// Conflict records a file skipped because an in-place rename or output target conflicted.
type Conflict struct {
	Path   string
	Target string
	Reason string
}

type fileJob struct {
	Name string
	Path string
}

type destinationLocker struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

func newDestinationLocker() *destinationLocker {
	return &destinationLocker{locks: make(map[string]*sync.Mutex)}
}

func (d *destinationLocker) Lock(path string) func() {
	d.mu.Lock()
	lock, ok := d.locks[path]
	if !ok {
		lock = &sync.Mutex{}
		d.locks[path] = lock
	}
	d.mu.Unlock()

	lock.Lock()
	return lock.Unlock
}

// Run converts eligible root-level files under cfg.InputDir to HEIC in place.
func Run(cfg Config) (*Stats, error) {
	files, err := scanRootFiles(cfg.InputDir)
	if err != nil {
		return nil, fmt.Errorf("scan input dir: %w", err)
	}

	stats := &Stats{Scanned: len(files)}
	if len(files) == 0 {
		return stats, nil
	}

	converter := cfg.Converter
	if converter == nil {
		converter = New()
	}

	workers := cfg.Workers
	if workers <= 0 {
		// Default to 2 rather than CPU count: HEIC encoding via libx265 is
		// memory-intensive, and too many parallel encodes risk OOM kills.
		workers = 2
	}

	infof := cfg.Infof
	if infof == nil {
		infof = progress.Info
	}
	warnf := cfg.Warnf
	if warnf == nil {
		warnf = progress.Warning
	}
	errorf := cfg.Errorf
	if errorf == nil {
		errorf = progress.Error
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var completed atomic.Int64
	reporter := progress.NewReporter(len(files), cfg.ShowProgress)
	defer reporter.Close()

	// oversizedSem serialises oversized HEIC encodes: at most one runs at a time
	// across all workers, preventing simultaneous multi-GB encoder processes.
	oversizedSem := make(chan struct{}, 1)

	locker := newDestinationLocker()
	jobCh := make(chan fileJob, workers)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobCh {
				processFile(job, cfg, converter, stats, &mu, locker, oversizedSem, infof, warnf, errorf)
				reporter.Update(int(completed.Add(1)))
			}
		}()
	}

	for _, job := range files {
		jobCh <- job
	}
	close(jobCh)
	wg.Wait()

	return stats, nil
}

func scanRootFiles(inputDir string) ([]fileJob, error) {
	entries, err := os.ReadDir(inputDir)
	if err != nil {
		return nil, err
	}

	files := make([]fileJob, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if entry.Type()&os.ModeType != 0 && !entry.Type().IsRegular() {
			continue
		}

		path := filepath.Join(inputDir, entry.Name())
		if !entry.Type().IsRegular() {
			info, err := entry.Info()
			if err != nil || !info.Mode().IsRegular() {
				continue
			}
		}

		files = append(files, fileJob{
			Name: entry.Name(),
			Path: path,
		})
	}
	return files, nil
}

func processFile(
	job fileJob,
	cfg Config,
	converter *Converter,
	stats *Stats,
	mu *sync.Mutex,
	locker *destinationLocker,
	oversizedSem chan struct{},
	infof func(string, ...any),
	warnf func(string, ...any),
	errorf func(string, ...any),
) {
	decoded, err := decodeSourceImage(job.Path)
	if err != nil {
		handleDecodeOutcome(job, err, stats, mu, warnf, errorf)
		return
	}

	originalPath := job.Path
	correctedPath := replaceExtension(job.Path, decoded.canonicalExt)
	renamed := correctedPath != originalPath
	targetPath := strings.TrimSuffix(correctedPath, filepath.Ext(correctedPath)) + ".heic"

	unlock := locker.Lock(targetPath)
	defer unlock()

	if targetExists(targetPath, originalPath, renamed) {
		recordConflict(stats, mu, originalPath, targetPath, "target .heic already exists")
		warnf("skip %s: target already exists at %s", originalPath, targetPath)
		return
	}

	if renamed && pathExists(correctedPath) {
		recordConflict(stats, mu, originalPath, correctedPath, "corrected source path already exists")
		warnf("skip %s: corrected source path already exists at %s", originalPath, correctedPath)
		return
	}

	if cfg.DryRun {
		recordPlanned(stats, mu, renamed)
		if renamed {
			infof("[dry-run] %s -> %s -> %s", originalPath, correctedPath, targetPath)
		} else {
			infof("[dry-run] %s -> %s", originalPath, targetPath)
		}
		return
	}

	// Serialise oversized HEIC encodes: hold the semaphore for the entire
	// rename → encode → finalise sequence so at most one oversized job runs
	// at a time, keeping peak encoder memory predictable.
	if IsOversized(decoded.pixelCount) {
		oversizedSem <- struct{}{}
		defer func() { <-oversizedSem }()
	}

	sourcePath := originalPath
	if renamed {
		if err := os.Rename(originalPath, correctedPath); err != nil {
			recordFailure(stats, mu, originalPath, fmt.Errorf("rename source to %s: %w", correctedPath, err))
			errorf("failed %s: rename source to %s: %v", originalPath, correctedPath, err)
			return
		}
		sourcePath = correctedPath
	}

	revertSource := func() error {
		if !renamed {
			return nil
		}
		if err := os.Rename(sourcePath, originalPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("revert source rename: %w", err)
		}
		return nil
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(targetPath), filepath.Base(targetPath)+".tmp-*.heic")
	if err != nil {
		revertErr := revertSource()
		recordFailure(stats, mu, originalPath, joinErrors(fmt.Errorf("create temp file: %w", err), revertErr))
		errorf("failed %s: create temp file: %v", originalPath, err)
		return
	}
	tmpPath := tmpFile.Name()
	if closeErr := tmpFile.Close(); closeErr != nil {
		_ = os.Remove(tmpPath)
		revertErr := revertSource()
		recordFailure(stats, mu, originalPath, joinErrors(fmt.Errorf("close temp file: %w", closeErr), revertErr))
		errorf("failed %s: close temp file: %v", originalPath, closeErr)
		return
	}
	defer os.Remove(tmpPath)

	srcInfo, err := converter.stat(sourcePath)
	if err != nil {
		revertErr := revertSource()
		recordFailure(stats, mu, originalPath, joinErrors(fmt.Errorf("stat source: %w", err), revertErr))
		errorf("failed %s: stat source: %v", originalPath, err)
		return
	}

	if err := converter.convertDecoded(sourcePath, tmpPath, srcInfo, decoded); err != nil {
		revertErr := revertSource()
		recordFailure(stats, mu, originalPath, joinErrors(err, revertErr))
		errorf("failed %s: %v", originalPath, err)
		return
	}

	if err := os.Rename(tmpPath, targetPath); err != nil {
		revertErr := revertSource()
		recordFailure(stats, mu, originalPath, joinErrors(fmt.Errorf("finalize target: %w", err), revertErr))
		errorf("failed %s: finalize target %s: %v", originalPath, targetPath, err)
		return
	}

	if err := os.Remove(sourcePath); err != nil {
		removeTargetErr := os.Remove(targetPath)
		revertErr := revertSource()
		recordFailure(stats, mu, originalPath, joinErrors(fmt.Errorf("delete source: %w", err), removeTargetErr, revertErr))
		errorf("failed %s: delete source: %v", originalPath, err)
		return
	}

	mu.Lock()
	stats.Converted++
	if renamed {
		stats.RenamedExtensions++
		infof("converted %s via corrected source extension -> %s", originalPath, targetPath)
	}
	mu.Unlock()
}

func handleDecodeOutcome(job fileJob, err error, stats *Stats, mu *sync.Mutex, warnf, errorf func(string, ...any)) {
	switch {
	case errors.Is(err, ErrAlreadyHEIC):
		mu.Lock()
		stats.SkippedAlreadyHEIC++
		mu.Unlock()
		warnf("skip %s: already HEIC/HEIF content", job.Path)
	case errors.Is(err, image.ErrFormat):
		if hasKnownImageExtension(job.Path) {
			recordFailure(stats, mu, job.Path, err)
			errorf("failed %s: %v", job.Path, err)
			return
		}
		mu.Lock()
		stats.SkippedUnsupported++
		mu.Unlock()
	default:
		recordFailure(stats, mu, job.Path, err)
		errorf("failed %s: %v", job.Path, err)
	}
}

func hasKnownImageExtension(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tif", ".tiff", ".webp", ".heic", ".heif":
		return true
	default:
		return false
	}
}

func replaceExtension(path, ext string) string {
	if ext == "" {
		return path
	}
	currentExt := filepath.Ext(path)
	if currentExt == "" {
		return path + ext
	}
	return strings.TrimSuffix(path, currentExt) + ext
}

func targetExists(targetPath, originalPath string, renamed bool) bool {
	if !pathExists(targetPath) {
		return false
	}
	return !(renamed && targetPath == originalPath)
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func recordPlanned(stats *Stats, mu *sync.Mutex, renamed bool) {
	mu.Lock()
	defer mu.Unlock()
	stats.Planned++
	if renamed {
		stats.RenamedExtensions++
	}
}

func recordConflict(stats *Stats, mu *sync.Mutex, path, target, reason string) {
	mu.Lock()
	defer mu.Unlock()
	stats.SkippedConflicts++
	stats.Conflicts = append(stats.Conflicts, Conflict{
		Path:   path,
		Target: target,
		Reason: reason,
	})
}

func recordFailure(stats *Stats, mu *sync.Mutex, path string, err error) {
	mu.Lock()
	defer mu.Unlock()
	stats.Failed++
	stats.Failures = append(stats.Failures, Failure{
		Path: path,
		Err:  err,
	})
}

func joinErrors(errs ...error) error {
	var filtered []error
	for _, err := range errs {
		if err != nil {
			filtered = append(filtered, err)
		}
	}
	return errors.Join(filtered...)
}
