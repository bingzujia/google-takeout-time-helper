package dedup

import (
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/bingzujia/google-takeout-time-helper/internal/hashcache"
	"github.com/bingzujia/google-takeout-time-helper/internal/heicconv"
	"github.com/bingzujia/google-takeout-time-helper/internal/migrator"
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
	Auto          bool   // automatic mode: keep largest file in root dir, all files in group-xxx/
	ConvertHEIC   bool   // enable HEIC to JPEG conversion for deduplication (default: true)
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		Threshold:   8, // both pHash and dHash must be <= this; catches exact duplicates and compression variants
		Recursive:   true,
		DryRun:      true,
		MaxDecodeMB: 500,
		ConvertHEIC: true, // enable HEIC conversion by default
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

	// 【新增】条件分支：根据 cfg.Auto 选择处理模式
	if cfg.Auto {
		return handleAutoMode(rootDir, cfg, dupGroups, len(entries), errors)
	} else {
		return handleStandardMode(rootDir, cfg, dupGroups, len(entries), errors)
	}
}

// handleStandardMode 处理标准模式（所有文件在 group-xxx 子目录）
func handleStandardMode(rootDir string, cfg Config, dupGroups []DuplicateGroup, totalScanned int, initErrors []FileError) (*Result, error) {
	result := &Result{
		TotalScanned: totalScanned,
		TotalGroups:  0,
		TotalDupes:   0,
		SpaceReclaim: 0,
		Groups:       dupGroups,
		Errors:       initErrors,
	}

	dedupDir := filepath.Join(rootDir, "dedup")

	for i, group := range dupGroups {
		if len(group.Files) < 2 {
			continue
		}

		groupName := fmt.Sprintf("group-%03d", i+1)
		groupDir := filepath.Join(dedupDir, groupName)

		for _, f := range group.Files {
			dest, err := destPathWithSuffix(groupDir, filepath.Base(f.Path))
			if err != nil {
				result.Errors = append(result.Errors, FileError{
					Path:  f.Path,
					Error: fmt.Sprintf("dest path failed: %v", err),
				})
				continue
			}

			if !cfg.DryRun {
				if err := os.MkdirAll(groupDir, 0755); err != nil {
					result.Errors = append(result.Errors, FileError{
						Path:  f.Path,
						Error: fmt.Sprintf("mkdir failed: %v", err),
					})
					continue
				}
				if err := os.Rename(f.Path, dest); err != nil {
					result.Errors = append(result.Errors, FileError{
						Path:  f.Path,
						Error: fmt.Sprintf("move failed: %v", err),
					})
					continue
				}
			}
		}

		result.TotalGroups++
		result.TotalDupes += len(group.Files) - 1
		for i, f := range group.Files {
			if i != group.Keep {
				result.SpaceReclaim += f.Size
			}
		}
	}

	return result, nil
}

// destPathWithSuffix 返回目标路径，避免覆盖现有文件；如果无法找到可用文件名则返回错误
func destPathWithSuffix(dir, base string) (string, error) {
	candidate := filepath.Join(dir, base)
	if _, err := os.Stat(candidate); os.IsNotExist(err) {
		return candidate, nil
	}

	// 添加数字后缀
	ext := filepath.Ext(base)
	name := base[:len(base)-len(ext)]

	const maxAttempts = 10000
	for i := 1; i < maxAttempts; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s_%d%s", name, i, ext))
		if _, err := os.Stat(candidate); err != nil {
			if os.IsNotExist(err) {
				return candidate, nil
			}
			// 其他错误（权限、I/O）：继续尝试下一个后缀
		}
	}
	// 达到最大尝试次数，返回错误
	return "", fmt.Errorf("could not find available filename after %d attempts in %s", maxAttempts, dir)
}



// copyFile 使用 32KB 缓冲复制文件
func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source failed: %w", err)
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create destination failed: %w", err)
	}
	defer destination.Close()

	// 使用 32KB 缓冲复制
	buf := make([]byte, 32*1024)
	_, err = copyWithBuffer(destination, source, buf)
	if err != nil {
		return fmt.Errorf("copy failed: %w", err)
	}

	// 保留原文件权限
	if fi, err := os.Stat(src); err == nil {
		os.Chmod(dst, fi.Mode())
	}

	return nil
}

// copyWithBuffer 使用提供的缓冲进行文件复制
func copyWithBuffer(dst, src *os.File, buf []byte) (written int64, err error) {
	for {
		nr, err := src.Read(buf)
		if nr > 0 {
			nw, err := dst.Write(buf[0:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if err != nil {
				return written, err
			}
			if nr != nw {
				return written, fmt.Errorf("short write")
			}
		}
		if err != nil {
			if err == io.EOF {
				return written, nil
			}
			return written, err
		}
	}
}

// handleAutoMode 处理自动模式（保留文件在根目录，所有文件在 group-xxx 子目录）
func handleAutoMode(rootDir string, cfg Config, dupGroups []DuplicateGroup, totalScanned int, initErrors []FileError) (*Result, error) {
	result := &Result{
		TotalScanned: totalScanned,
		TotalGroups:  0,
		TotalDupes:   0,
		SpaceReclaim: 0,
		Groups:       dupGroups,
		Errors:       initErrors,
	}

	dedupDir := filepath.Join(rootDir, "dedup-auto")
	rootDedupDir := filepath.Join(rootDir, "dedup-auto")

	for i, group := range dupGroups {
		if len(group.Files) < 2 {
			continue
		}

		// 使用 group.Keep（与 Run() 中的计算一致）保留在根目录的文件
		bestIdx := group.Keep

		groupName := fmt.Sprintf("group-%03d", i+1)
		groupDir := filepath.Join(dedupDir, groupName)

		for j, f := range group.Files {
			// 保留的文件：复制到根目录的 dedup-auto
			if j == bestIdx {
				if !cfg.DryRun {
					if err := os.MkdirAll(rootDedupDir, 0755); err != nil {
						result.Errors = append(result.Errors, FileError{
							Path:  f.Path,
							Error: fmt.Sprintf("mkdir root failed: %v", err),
						})
					} else {
						dest, err := destPathWithSuffix(rootDedupDir, filepath.Base(f.Path))
						if err != nil {
							result.Errors = append(result.Errors, FileError{
								Path:  f.Path,
								Error: fmt.Sprintf("root dest path failed: %v", err),
							})
						} else if err := copyFile(f.Path, dest); err != nil {
							result.Errors = append(result.Errors, FileError{
								Path:  f.Path,
								Error: fmt.Sprintf("copy to root failed: %v", err),
							})
						}
					}
				}
			}

			// 所有文件都复制到 group-xxx（无论根目录操作是否成功）
			dest, err := destPathWithSuffix(groupDir, filepath.Base(f.Path))
			if err != nil {
				result.Errors = append(result.Errors, FileError{
					Path:  f.Path,
					Error: fmt.Sprintf("group dest path failed: %v", err),
				})
				continue
			}
			if !cfg.DryRun {
				if err := os.MkdirAll(groupDir, 0755); err != nil {
					result.Errors = append(result.Errors, FileError{
						Path:  f.Path,
						Error: fmt.Sprintf("mkdir group failed: %v", err),
					})
					continue
				}
				if err := copyFile(f.Path, dest); err != nil {
					result.Errors = append(result.Errors, FileError{
						Path:  f.Path,
						Error: fmt.Sprintf("copy to group failed: %v", err),
					})
				}
			}
		}

		result.TotalGroups++
		result.TotalDupes += len(group.Files) - 1
		for i, f := range group.Files {
			if i != bestIdx {
				result.SpaceReclaim += f.Size
			}
		}
	}

	return result, nil
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

// prepareFileForDecode checks whether the file's extension matches its actual
// content. If there is a mismatch, the file is temporarily renamed to the
// correct extension so image.Decode can operate on it.
//
// Returns:
//   - workPath: path to pass to image.Decode (may differ from originalPath)
//   - cleanup: must be called after decoding to restore the original name
//   - err: non-nil when the actual type is known but cannot be mapped to an
//     extension — the caller should skip the file. If detection fails entirely,
//     the file is returned unchanged so image.Decode can attempt it anyway.
func prepareFileForDecode(originalPath string) (workPath string, cleanup func() error, err error) {
	ft, detErr := migrator.DetectFileAll(originalPath)
	if detErr != nil {
		// Detection unavailable; fall through and let image.Decode try.
		return originalPath, func() error { return nil }, nil
	}
	if !ft.TypeKnown {
		// Unknown file type detected.
		return "", nil, fmt.Errorf("unknown file type: %s", ft.MimeType)
	}

	// Check if this is a HEIC file
	if ft.MimeType == "image/heic" || ft.MimeType == "image/heif" {
		return prepareHEICForDecode(originalPath)
	}

	// Extension mismatch: rename to correct extension before calling image.Decode.
	if ft.NewExt != "" {
		dir := filepath.Dir(originalPath)
		stem := strings.TrimSuffix(filepath.Base(originalPath), filepath.Ext(originalPath))
		tmpPath := filepath.Join(dir, stem+ft.NewExt)
		if err := os.Rename(originalPath, tmpPath); err != nil {
			return "", nil, fmt.Errorf("rename for decode: %w", err)
		}

		return tmpPath, func() error { return os.Rename(tmpPath, originalPath) }, nil
	}

	return originalPath, func() error { return nil }, nil
}

// prepareHEICForDecode converts a HEIC file to temporary JPEG for processing.
// Returns the path to the temporary JPEG, a cleanup function to remove it, and any error.
func prepareHEICForDecode(heicPath string) (workPath string, cleanup func() error, err error) {
	// Use new heicconv.ConvertFromHEIC decoder instead of the old encoder
	workPath, cleanup, err = heicconv.ConvertFromHEIC(heicPath, "")
	if err != nil {
		// Handle different error types
		if errors.Is(err, heicconv.ErrHeifConvertNotFound) {
			return "", nil, fmt.Errorf("heif-convert tool not available: cannot decode HEIC files")
		}
		if errors.Is(err, heicconv.ErrHeifConvertVersionOld) {
			return "", nil, fmt.Errorf("heif-convert version too old: please upgrade")
		}
		if errors.Is(err, heicconv.ErrHeifConvertDecodeError) {
			return "", nil, fmt.Errorf("failed to decode HEIC: %w", err)
		}
		if errors.Is(err, heicconv.ErrHeifConvertIOError) {
			return "", nil, fmt.Errorf("IO error during HEIC decode: %w", err)
		}
		return "", nil, fmt.Errorf("heic conversion: %w", err)
	}

	return workPath, cleanup, nil
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

	// Prepare the file for decoding: fix extension mismatch if needed.
	workPath, cleanup, prepErr := prepareFileForDecode(path)
	if prepErr != nil {
		return preparedResult{err: &FileError{Path: path, Error: prepErr.Error()}}
	}
	defer func() {
		_ = cleanup()
	}()

	f, err := os.Open(workPath)
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
