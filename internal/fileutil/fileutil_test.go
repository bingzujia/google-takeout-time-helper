package fileutil_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bingzujia/g_photo_take_out_helper/internal/fileutil"
)

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")

	if err := os.WriteFile(src, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := fileutil.CopyFile(src, dst); err != nil {
		t.Fatalf("CopyFile: %v", err)
	}

	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Errorf("content mismatch: %q", data)
	}

	// mtime should be preserved (within 2s tolerance)
	srcInfo, _ := os.Stat(src)
	dstInfo, _ := os.Stat(dst)
	diff := srcInfo.ModTime().Sub(dstInfo.ModTime())
	if diff < 0 {
		diff = -diff
	}
	if diff.Seconds() > 2 {
		t.Errorf("mtime not preserved: src=%v dst=%v", srcInfo.ModTime(), dstInfo.ModTime())
	}
}

func TestCopyFileSrcNotFound(t *testing.T) {
	dir := t.TempDir()
	err := fileutil.CopyFile(filepath.Join(dir, "missing.txt"), filepath.Join(dir, "dst.txt"))
	if err == nil {
		t.Error("expected error for missing src, got nil")
	}
}

func TestResolveDestPath_NoConflict(t *testing.T) {
	dir := t.TempDir()
	got := fileutil.ResolveDestPath(dir, "IMG001.jpg")
	want := filepath.Join(dir, "IMG001.jpg")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveDestPath_Conflict(t *testing.T) {
	dir := t.TempDir()
	// Create the file so there is a conflict
	if err := os.WriteFile(filepath.Join(dir, "IMG001.jpg"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	got := fileutil.ResolveDestPath(dir, "IMG001.jpg")
	// Should not be the same path
	if got == filepath.Join(dir, "IMG001.jpg") {
		t.Error("expected a different path due to conflict")
	}
	// Should still be in the same dir and have .jpg extension
	if filepath.Dir(got) != dir {
		t.Errorf("unexpected dir in result: %q", got)
	}
	if !strings.HasSuffix(got, ".jpg") {
		t.Errorf("expected .jpg suffix, got %q", got)
	}
}
