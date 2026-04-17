package dedup

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/bingzujia/google-takeout-time-helper/internal/hashcache"
	"github.com/bingzujia/google-takeout-time-helper/internal/progress"
	"github.com/bingzujia/google-takeout-time-helper/internal/workerpool"
	"github.com/corona10/goimagehash"
)

// supported image extensions
var imageExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
	".bmp": true, ".tiff": true, ".tif": true, ".webp": true,
	".heic": true, ".heif": true,
}

// Config holds deduplication settings.
type Config struct {
	Threshold     int    // max hash distance to consider "duplicate" (lower = stricter)
	Recursive     bool   // scan subdirectories
	DryRun        bool   // don't delete, just report
	ShowProgress  bool   // display progress during per-file preparation
	NoCache       bool   // disable hash cache (always recompute)
	CacheDir      string // directory for the hash cache DB (default: <inputDir>/.gtoh_cache)
	MaxDecodeMB   int    // max file size in MB to attempt image decoding (0 = unlimited)
	DecodeWorkers int    // max concurrent image decoders (0 = unlimited)
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		Threshold:   10, // both pHash and dHash must be <= this
		Recursive:   true,
		DryRun:      true,
		MaxDecodeMB: 500,
	}
}

// ImageInfo holds metadata about a scanned image.
type ImageInfo struct {
	Path   string
	Size   int64
	Hash   string // hex-encoded hash value
	Width  int
	Height int
}

// DuplicateGroup holds a set of files considered duplicates of each other.
type DuplicateGroup struct {
	Files []ImageInfo
	// Keep is the index of the file to keep (usually the first/largest)
	Keep int
}

// Result holds the full deduplication result.
type Result struct {
	TotalScanned int
	TotalGroups  int
	TotalDupes   int // total duplicate files (excluding kept ones)
	SpaceReclaim int64
	Groups       []DuplicateGroup
	Errors       []FileError
}

// FileError holds information about a file that failed to process.
type FileError struct {
	Path  string
	Error string
}

// Run executes deduplication on the given directory.
func Run(rootDir string, cfg Config) (*Result, error) {
	imagePaths, err := collectImagePaths(rootDir, cfg.Recursive)
	if err != nil {
		return nil, err
	}

	// Open hash cache unless disabled.
	var cache *hashcache.Cache
	if !cfg.NoCache {
		cacheDir := cfg.CacheDir
		if cacheDir == "" {
			cacheDir = filepath.Join(rootDir, ".gtoh_cache")
		}
		dbPath := filepath.Join(cacheDir, "dedup_hashes.db")
		cache, err = hashcache.Open(dbPath)
		if err != nil {
			// Non-fatal: proceed without cache.
			progress.Warning("hash cache unavailable: %v", err)
			cache = nil
		} else {
			defer cache.Close()
		}
	}

	entries, errors := prepareEntries(imagePaths, cfg.ShowProgress, cache, cfg.MaxDecodeMB, cfg.DecodeWorkers)

	// Step 4: group duplicates using pHash bucket pre-filtering (O(n·k)).
	uf := newUnionFind(len(entries))
	buckets := buildBuckets(entries, 16)
	for _, indices := range buckets {
		for a := 0; a < len(indices); a++ {
			for b := a + 1; b < len(indices); b++ {
				i, j := indices[a], indices[b]
				pDist, _ := goimagehash.NewImageHash(entries[i].phash, goimagehash.PHash).Distance(
					goimagehash.NewImageHash(entries[j].phash, goimagehash.PHash))
				dDist, _ := goimagehash.NewImageHash(entries[i].dhash, goimagehash.DHash).Distance(
					goimagehash.NewImageHash(entries[j].dhash, goimagehash.DHash))
				if pDist <= cfg.Threshold && dDist <= cfg.Threshold {
					uf.union(i, j)
				}
			}
		}
	}

	// Step 5: build groups
	groups := uf.groups()
	var dupGroups []DuplicateGroup
	totalDupes := 0
	spaceReclaim := int64(0)

	for _, group := range groups {
		if len(group) < 2 {
			continue
		}

		// Sort by size descending — keep the largest
		// (group indices are already in entries order, find largest)
		keepIdx := 0
		for i := 1; i < len(group); i++ {
			if entries[group[i]].size > entries[group[keepIdx]].size {
				keepIdx = i
			}
		}

		var files []ImageInfo
		for _, idx := range group {
			files = append(files, ImageInfo{
				Path:   entries[idx].path,
				Size:   entries[idx].size,
				Width:  entries[idx].width,
				Height: entries[idx].height,
			})
		}

		// Count duplicates (excluding the kept file)
		dupes := len(files) - 1
		totalDupes += dupes

		// Calculate reclaimable space (all except kept)
		for i, f := range files {
			if i != keepIdx {
				spaceReclaim += f.Size
			}
		}

		dupGroups = append(dupGroups, DuplicateGroup{
			Files: files,
			Keep:  keepIdx,
		})
	}

	return &Result{
		TotalScanned: len(entries),
		TotalGroups:  len(dupGroups),
		TotalDupes:   totalDupes,
		SpaceReclaim: spaceReclaim,
		Groups:       dupGroups,
		Errors:       errors,
	}, nil
}

type preparedEntry struct {
	path   string
	size   int64
	width  int
	height int
	phash  uint64
	dhash  uint64
}

type preparedResult struct {
	entry preparedEntry
	err   *FileError
	ok    bool
}

func collectImagePaths(rootDir string, recursive bool) ([]string, error) {
	var imagePaths []string
	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors, they'll be caught during hashing
		}
		if info.IsDir() && !recursive && path != rootDir {
			return filepath.SkipDir
		}
		if !info.IsDir() && imageExts[strings.ToLower(filepath.Ext(path))] {
			imagePaths = append(imagePaths, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk directory: %w", err)
	}
	return imagePaths, nil
}

func prepareEntries(imagePaths []string, showProgress bool, cache *hashcache.Cache, maxDecodeMB int, decodeWorkers int) ([]preparedEntry, []FileError) {
	if len(imagePaths) == 0 {
		return nil, nil
	}

	// Build semaphore to cap concurrent image decodes.
	// A nil sem means no limit (decodeWorkers <= 0).
	var sem chan struct{}
	if decodeWorkers > 0 {
		sem = make(chan struct{}, decodeWorkers)
	}

	results := make([]preparedResult, len(imagePaths))
	var completed atomic.Int64
	reporter := progress.NewReporter(len(imagePaths), showProgress)
	defer reporter.Close()

	indices := make([]int, len(imagePaths))
	for i := range indices {
		indices[i] = i
	}
	_ = workerpool.Run(indices, workerpool.DefaultWorkers(), func(idx int) error {
		results[idx] = prepareEntry(imagePaths[idx], cache, maxDecodeMB, sem)
		reporter.Update(int(completed.Add(1)))
		return nil
	})

	entries := make([]preparedEntry, 0, len(results))
	errors := make([]FileError, 0)
	for _, res := range results {
		if res.err != nil {
			errors = append(errors, *res.err)
			continue
		}
		if res.ok {
			entries = append(entries, res.entry)
		}
	}

	return entries, errors
}

func prepareEntry(path string, cache *hashcache.Cache, maxDecodeMB int, sem chan struct{}) preparedResult {
	info, err := os.Stat(path)
	if err != nil {
		return preparedResult{err: &FileError{Path: path, Error: err.Error()}}
	}

	// Big-image memory guard.
	if maxDecodeMB > 0 && info.Size() > int64(maxDecodeMB)*1024*1024 {
		return preparedResult{err: &FileError{Path: path, Error: "oversized: file too large to decode"}}
	}

	mtime := info.ModTime().Unix()
	size := info.Size()

	// Check hash cache.
	if cache != nil {
		if entry, ok := cache.Get(path, mtime, size); ok {
			// Retrieve width/height from stat approximation — we don't store them,
			// so set to 0; they are only used for "keep largest" heuristic.
			return preparedResult{
				ok: true,
				entry: preparedEntry{
					path:  path,
					size:  size,
					phash: entry.PHash,
					dhash: entry.DHash,
				},
			}
		}
	}

	f, err := os.Open(path)
	if err != nil {
		return preparedResult{err: &FileError{Path: path, Error: err.Error()}}
	}

	// Acquire decode semaphore — limits peak concurrent decoded images in memory.
	if sem != nil {
		sem <- struct{}{}
	}
	img, _, err := image.Decode(f)
	f.Close()
	if sem != nil {
		<-sem
	}
	if err != nil {
		return preparedResult{err: &FileError{Path: path, Error: "decode: " + err.Error()}}
	}

	ph, err := goimagehash.PerceptionHash(img)
	if err != nil {
		return preparedResult{err: &FileError{Path: path, Error: "phash: " + err.Error()}}
	}
	dh, err := goimagehash.DifferenceHash(img)
	if err != nil {
		return preparedResult{err: &FileError{Path: path, Error: "dhash: " + err.Error()}}
	}

	bounds := img.Bounds()

	if cache != nil {
		_ = cache.Set(path, mtime, size, ph.GetHash(), dh.GetHash())
	}

	return preparedResult{
		ok: true,
		entry: preparedEntry{
			path:   path,
			size:   size,
			width:  bounds.Dx(),
			height: bounds.Dy(),
			phash:  ph.GetHash(),
			dhash:  dh.GetHash(),
		},
	}
}

// buildBuckets groups entry indices by the top `bits` bits of their pHash.
// Only entries in the same bucket need to be compared, reducing O(n²) to O(n·k)
// where k is the average bucket size.
func buildBuckets(entries []preparedEntry, bits int) map[uint64][]int {
	shift := uint(64 - bits)
	buckets := make(map[uint64][]int, len(entries))
	for i, e := range entries {
		key := e.phash >> shift
		buckets[key] = append(buckets[key], i)
	}
	return buckets
}

// unionFind implements disjoint-set for grouping duplicates.
type unionFind struct {
	parent []int
	rank   []int
}

func newUnionFind(n int) *unionFind {
	parent := make([]int, n)
	rank := make([]int, n)
	for i := range parent {
		parent[i] = i
	}
	return &unionFind{parent: parent, rank: rank}
}

func (uf *unionFind) find(x int) int {
	if uf.parent[x] != x {
		uf.parent[x] = uf.find(uf.parent[x])
	}
	return uf.parent[x]
}

func (uf *unionFind) union(x, y int) {
	px, py := uf.find(x), uf.find(y)
	if px == py {
		return
	}
	if uf.rank[px] < uf.rank[py] {
		uf.parent[px] = py
	} else if uf.rank[px] > uf.rank[py] {
		uf.parent[py] = px
	} else {
		uf.parent[py] = px
		uf.rank[px]++
	}
}

func (uf *unionFind) groups() [][]int {
	m := make(map[int][]int)
	for i := range uf.parent {
		root := uf.find(i)
		m[root] = append(m[root], i)
	}
	var result [][]int
	for _, g := range m {
		result = append(result, g)
	}
	return result
}
