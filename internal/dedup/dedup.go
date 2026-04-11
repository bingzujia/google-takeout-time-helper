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
	Threshold int  // max hash distance to consider "duplicate" (lower = stricter)
	Recursive bool // scan subdirectories
	DryRun    bool // don't delete, just report
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		Threshold: 10, // both pHash and dHash must be <= this
		Recursive: true,
		DryRun:    true,
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
	// Step 1: collect all image files
	var imagePaths []string
	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors, they'll be caught during hashing
		}
		if info.IsDir() && !cfg.Recursive && path != rootDir {
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

	// Step 2: compute hashes
	type hashEntry struct {
		path string
		size int64
		img  image.Image
	}

	var entries []hashEntry
	var errors []FileError

	for _, path := range imagePaths {
		info, err := os.Stat(path)
		if err != nil {
			errors = append(errors, FileError{path, err.Error()})
			continue
		}

		f, err := os.Open(path)
		if err != nil {
			errors = append(errors, FileError{path, err.Error()})
			continue
		}

		img, _, err := image.Decode(f)
		f.Close()
		if err != nil {
			errors = append(errors, FileError{path, "decode: " + err.Error()})
			continue
		}

		entries = append(entries, hashEntry{
			path: path,
			size: info.Size(),
			img:  img,
		})
	}

	// Step 3: compute both pHash and dHash for each image
	type hashPair struct {
		phash uint64
		dhash uint64
	}
	hashPairs := make([]hashPair, len(entries))
	for i, e := range entries {
		ph, err := goimagehash.PerceptionHash(e.img)
		if err != nil {
			errors = append(errors, FileError{e.path, "phash: " + err.Error()})
			continue
		}
		dh, err := goimagehash.DifferenceHash(e.img)
		if err != nil {
			errors = append(errors, FileError{e.path, "dhash: " + err.Error()})
			continue
		}
		hashPairs[i] = hashPair{
			phash: ph.GetHash(),
			dhash: dh.GetHash(),
		}
	}

	// Step 4: group duplicates — BOTH pHash AND dHash must be within threshold
	uf := newUnionFind(len(entries))
	for i := 0; i < len(hashPairs); i++ {
		for j := i + 1; j < len(hashPairs); j++ {
			pDist, _ := goimagehash.NewImageHash(hashPairs[i].phash, goimagehash.PHash).Distance(
				goimagehash.NewImageHash(hashPairs[j].phash, goimagehash.PHash))
			dDist, _ := goimagehash.NewImageHash(hashPairs[i].dhash, goimagehash.DHash).Distance(
				goimagehash.NewImageHash(hashPairs[j].dhash, goimagehash.DHash))
			if pDist <= cfg.Threshold && dDist <= cfg.Threshold {
				uf.union(i, j)
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
			bounds := entries[idx].img.Bounds()
			files = append(files, ImageInfo{
				Path:   entries[idx].path,
				Size:   entries[idx].size,
				Width:  bounds.Dx(),
				Height: bounds.Dy(),
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
