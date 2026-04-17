package hashcache

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS hashes (
	path  TEXT    NOT NULL PRIMARY KEY,
	mtime INTEGER NOT NULL,
	size  INTEGER NOT NULL,
	phash INTEGER NOT NULL,
	dhash INTEGER NOT NULL
);
`

// Entry holds the cached hash values for a file.
type Entry struct {
	PHash uint64
	DHash uint64
}

// Cache is a persistent SQLite-backed store for pHash/dHash values.
type Cache struct {
	db *sql.DB
}

// Open opens (or creates) the SQLite database at dbPath.
// The parent directory must already exist.
func Open(dbPath string) (*Cache, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("hashcache: create dir: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("hashcache: open db: %w", err)
	}

	// Enable WAL for better concurrent read performance.
	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		db.Close()
		return nil, fmt.Errorf("hashcache: enable WAL: %w", err)
	}

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("hashcache: create schema: %w", err)
	}

	return &Cache{db: db}, nil
}

// Get returns the cached entry for the given file identity (path + mtime + size).
// Returns (zero, false) on cache miss or stale entry.
func (c *Cache) Get(path string, mtime, size int64) (Entry, bool) {
	row := c.db.QueryRow(
		`SELECT phash, dhash FROM hashes WHERE path=? AND mtime=? AND size=?`,
		path, mtime, size,
	)
	var ph, dh int64
	if err := row.Scan(&ph, &dh); err != nil {
		return Entry{}, false
	}
	return Entry{PHash: uint64(ph), DHash: uint64(dh)}, true
}

// Set stores or updates the hash entry for the given file identity.
func (c *Cache) Set(path string, mtime, size int64, phash, dhash uint64) error {
	_, err := c.db.Exec(
		`INSERT OR REPLACE INTO hashes (path, mtime, size, phash, dhash) VALUES (?,?,?,?,?)`,
		path, mtime, size, int64(phash), int64(dhash),
	)
	if err != nil {
		return fmt.Errorf("hashcache: set %s: %w", path, err)
	}
	return nil
}

// Close closes the underlying database connection.
func (c *Cache) Close() error {
	return c.db.Close()
}
