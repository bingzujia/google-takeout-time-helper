package hashcache_test

import (
	"path/filepath"
	"testing"

	"github.com/bingzujia/g_photo_take_out_helper/internal/hashcache"
)

func TestCacheMissAndSet(t *testing.T) {
	dir := t.TempDir()
	c, err := hashcache.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	_, ok := c.Get("/img/a.jpg", 1000, 2000)
	if ok {
		t.Fatal("expected cache miss on empty db")
	}

	if err := c.Set("/img/a.jpg", 1000, 2000, 0xDEADBEEF, 0xCAFEBABE); err != nil {
		t.Fatal(err)
	}

	entry, ok := c.Get("/img/a.jpg", 1000, 2000)
	if !ok {
		t.Fatal("expected cache hit after Set")
	}
	if entry.PHash != 0xDEADBEEF || entry.DHash != 0xCAFEBABE {
		t.Errorf("wrong hashes: phash=%x dhash=%x", entry.PHash, entry.DHash)
	}
}

func TestCacheHitExactKey(t *testing.T) {
	dir := t.TempDir()
	c, err := hashcache.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	_ = c.Set("/img/b.jpg", 111, 222, 1, 2)

	if _, ok := c.Get("/img/b.jpg", 111, 222); !ok {
		t.Error("expected hit with exact key")
	}
}

func TestStaleEntryMtimeChanged(t *testing.T) {
	dir := t.TempDir()
	c, err := hashcache.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	_ = c.Set("/img/c.jpg", 100, 500, 1, 2)

	_, ok := c.Get("/img/c.jpg", 999, 500) // different mtime
	if ok {
		t.Error("expected cache miss for changed mtime")
	}

	_, ok = c.Get("/img/c.jpg", 100, 999) // different size
	if ok {
		t.Error("expected cache miss for changed size")
	}
}

func TestSetOverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	c, err := hashcache.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	_ = c.Set("/img/d.jpg", 1, 1, 0xAAAA, 0xBBBB)
	_ = c.Set("/img/d.jpg", 1, 1, 0x1111, 0x2222)

	entry, ok := c.Get("/img/d.jpg", 1, 1)
	if !ok {
		t.Fatal("expected hit after overwrite")
	}
	if entry.PHash != 0x1111 || entry.DHash != 0x2222 {
		t.Errorf("expected overwritten values, got phash=%x dhash=%x", entry.PHash, entry.DHash)
	}
}

func TestOpenCreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "nested", "sub", "hashes.db")
	c, err := hashcache.Open(dbPath)
	if err != nil {
		t.Fatalf("expected Open to create parent dirs: %v", err)
	}
	c.Close()
}

func TestPersistenceAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "hashes.db")

	c1, _ := hashcache.Open(dbPath)
	_ = c1.Set("/img/e.jpg", 42, 100, 0xF00D, 0xBEEF)
	c1.Close()

	c2, err := hashcache.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer c2.Close()

	entry, ok := c2.Get("/img/e.jpg", 42, 100)
	if !ok {
		t.Fatal("expected data to persist after reopen")
	}
	if entry.PHash != 0xF00D || entry.DHash != 0xBEEF {
		t.Errorf("got phash=%x dhash=%x", entry.PHash, entry.DHash)
	}
}
