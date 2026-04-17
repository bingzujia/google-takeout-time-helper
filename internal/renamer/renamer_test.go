package renamer

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ── buildName ─────────────────────────────────────────────────────────────────

func TestBuildName(t *testing.T) {
	tm := time.Date(2023, 1, 23, 10, 47, 7, 0, time.UTC)
	cases := []struct {
		ext  string
		want string
	}{
		{"heic", "IMG20230123104707.heic"},
		{"heif", "IMG20230123104707.heif"},
		{"jpg", "IMG_20230123_104707.jpg"},
		{"jpeg", "IMG_20230123_104707.jpeg"},
		{"png", "IMG_20230123_104707.png"},
		{"mp4", "VID20230123104707.mp4"},
		{"mov", "VID20230123104707.mov"},
	}
	for _, c := range cases {
		got := buildName(c.ext, tm)
		if got != c.want {
			t.Errorf("buildName(%q): got %q, want %q", c.ext, got, c.want)
		}
	}
}

// ── buildBurstName ────────────────────────────────────────────────────────────

func TestBuildBurstName(t *testing.T) {
	cases := []struct {
		ext      string
		dateTime string
		idx      int
		want     string
	}{
		{"heic", "20190207_184125", 0, "IMG20190207184125_BURST000.heic"},
		{"heic", "20190207_184125", 1, "IMG20190207184125_BURST001.heic"},
		{"jpg", "20190207_184125", 0, "IMG_20190207_184125_BURST000.jpg"},
		{"jpg", "20190207_184125", 1, "IMG_20190207_184125_BURST001.jpg"},
	}
	for _, c := range cases {
		got := buildBurstName(c.ext, c.dateTime, c.idx)
		if got != c.want {
			t.Errorf("buildBurstName(%q, %q, %d): got %q, want %q", c.ext, c.dateTime, c.idx, got, c.want)
		}
	}
}

// ── detectBurstGroups ─────────────────────────────────────────────────────────

type fakeDirEntry struct {
	name string
}

func (f fakeDirEntry) Name() string               { return f.name }
func (f fakeDirEntry) IsDir() bool                { return false }
func (f fakeDirEntry) Type() os.FileMode          { return 0 }
func (f fakeDirEntry) Info() (os.FileInfo, error) { return nil, nil }

func fakeEntries(names ...string) []os.DirEntry {
	out := make([]os.DirEntry, len(names))
	for i, n := range names {
		out[i] = fakeDirEntry{n}
	}
	return out
}

func TestDetectBurstGroups(t *testing.T) {
	entries := fakeEntries(
		"20190207_184125_007.heic",
		"20190207_184125_009.heic",
		"20190207_184125_007.mp4", // video – must be ignored
		"20230101_120000_001.jpg", // single burst-like → not a group
		"photo.jpg",
	)
	groups := detectBurstGroups(entries)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	g, ok := groups["20190207_184125"]
	if !ok {
		t.Fatal("expected group key 20190207_184125")
	}
	if len(g) != 2 {
		t.Errorf("expected 2 files in group, got %d", len(g))
	}
}

// ── detectMp4Pairs ────────────────────────────────────────────────────────────

func TestDetectMp4Pairs(t *testing.T) {
	entries := fakeEntries(
		"photo.heic",
		"photo.mp4",  // paired
		"clip.mp4",   // no image companion → not paired
		"other.jpg",
	)
	pairs := detectMp4Pairs(entries)
	if len(pairs) != 1 {
		t.Fatalf("expected 1 pair, got %d", len(pairs))
	}
	if pairs["photo"] != "photo.mp4" {
		t.Errorf("expected pairs[photo]=photo.mp4, got %q", pairs["photo"])
	}
}

// ── Run (integration) ─────────────────────────────────────────────────────────

func writeFile(t *testing.T, dir, name string, mtime time.Time) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}
}

func TestRunNormalRename(t *testing.T) {
	dir := t.TempDir()
	tm := time.Date(2023, 1, 23, 10, 47, 7, 0, time.Local)
	writeFile(t, dir, "shot.heic", tm)
	writeFile(t, dir, "photo.jpg", tm)

	res, err := Run(Config{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if res.Renamed != 2 {
		t.Errorf("expected 2 renamed, got %d", res.Renamed)
	}
	if _, err := os.Stat(filepath.Join(dir, "IMG20230123104707.heic")); err != nil {
		t.Error("IMG20230123104707.heic not found")
	}
	if _, err := os.Stat(filepath.Join(dir, "IMG_20230123_104707.jpg")); err != nil {
		t.Error("IMG_20230123_104707.jpg not found")
	}
}

func TestRunBurstRename(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "20190207_184125_007.heic", time.Now())
	writeFile(t, dir, "20190207_184125_009.heic", time.Now())

	res, err := Run(Config{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if res.Renamed != 2 {
		t.Errorf("expected 2 renamed, got %d", res.Renamed)
	}
	if _, err := os.Stat(filepath.Join(dir, "IMG20190207184125_BURST000.heic")); err != nil {
		t.Error("BURST000 not found")
	}
	if _, err := os.Stat(filepath.Join(dir, "IMG20190207184125_BURST001.heic")); err != nil {
		t.Error("BURST001 not found")
	}
}

func TestRunMtimeConflict(t *testing.T) {
	dir := t.TempDir()
	tm := time.Date(2019, 4, 3, 16, 51, 10, 0, time.Local)
	writeFile(t, dir, "a.jpg", tm)
	writeFile(t, dir, "b.jpg", tm)

	res, err := Run(Config{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if res.Renamed != 2 {
		t.Errorf("expected 2 renamed, got %d", res.Renamed)
	}
	if _, err := os.Stat(filepath.Join(dir, "IMG_20190403_165110.jpg")); err != nil {
		t.Error("IMG_20190403_165110.jpg not found")
	}
	if _, err := os.Stat(filepath.Join(dir, "IMG_20190403_165110_001.jpg")); err != nil {
		t.Error("IMG_20190403_165110_001.jpg not found")
	}
}

func TestRunMp4Companion(t *testing.T) {
	dir := t.TempDir()
	tm := time.Date(2023, 1, 23, 10, 47, 7, 0, time.Local)
	writeFile(t, dir, "photo.heic", tm)
	writeFile(t, dir, "photo.mp4", tm)

	res, err := Run(Config{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if res.Renamed != 2 {
		t.Errorf("expected 2 renamed, got %d (heic+mp4)", res.Renamed)
	}
	if _, err := os.Stat(filepath.Join(dir, "IMG20230123104707.heic")); err != nil {
		t.Error("IMG20230123104707.heic not found")
	}
	if _, err := os.Stat(filepath.Join(dir, "IMG20230123104707.mp4")); err != nil {
		t.Error("IMG20230123104707.mp4 not found")
	}
}
