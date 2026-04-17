package matcher

import (
	"os"
	"sync"
)

// DirCache caches os.ReadDir results keyed by directory path. It is safe for
// concurrent use: each unique directory is read from disk at most once.
type DirCache struct {
	m sync.Map // key: string (dir path), value: dirEntry
}

type dirEntry struct {
	entries []os.DirEntry
	err     error
}

// ReadDir returns the directory entries for dir. Results are cached after the
// first call for each directory. When c is nil, os.ReadDir is called directly.
func (c *DirCache) ReadDir(dir string) ([]os.DirEntry, error) {
	if c == nil {
		return os.ReadDir(dir)
	}

	if v, ok := c.m.Load(dir); ok {
		de := v.(dirEntry)
		return de.entries, de.err
	}

	entries, err := os.ReadDir(dir)
	// Store even on error so we don't retry repeatedly for a bad directory.
	c.m.Store(dir, dirEntry{entries: entries, err: err})
	return entries, err
}
