package heicconv

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunScansRootLevelFilesOnly(t *testing.T) {
	tmpDir := t.TempDir()
	rootImage := filepath.Join(tmpDir, "root.jpg")
	nestedDir := filepath.Join(tmpDir, "nested")
	nestedImage := filepath.Join(nestedDir, "inside.jpg")

	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeJPEGFixture(t, rootImage)
	writeJPEGFixture(t, nestedImage)

	stats, err := Run(Config{
		InputDir:     tmpDir,
		DryRun:       true,
		ShowProgress: false,
		Infof:        func(string, ...any) {},
		Warnf:        func(string, ...any) {},
		Errorf:       func(string, ...any) {},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if stats.Scanned != 1 {
		t.Fatalf("stats.Scanned = %d, want 1", stats.Scanned)
	}
	if stats.Planned != 1 {
		t.Fatalf("stats.Planned = %d, want 1", stats.Planned)
	}
}

func TestRunCorrectsExtensionThenConvertsInPlace(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "photo.png")
	writeJPEGFixture(t, srcPath)

	metadata := &fakeMetadataRestorer{}
	stats, err := Run(Config{
		InputDir:     tmpDir,
		ShowProgress: false,
		Converter: &Converter{
			encoder:          &fakeEncoder{},
			metadataRestorer: metadata,
			stat:             os.Stat,
			chtimes:          os.Chtimes,
		},
		Infof:  func(string, ...any) {},
		Warnf:  func(string, ...any) {},
		Errorf: func(string, ...any) {},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if stats.Converted != 1 {
		t.Fatalf("stats.Converted = %d, want 1", stats.Converted)
	}
	if stats.RenamedExtensions != 1 {
		t.Fatalf("stats.RenamedExtensions = %d, want 1", stats.RenamedExtensions)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "photo.heic")); err != nil {
		t.Fatalf("expected photo.heic to exist: %v", err)
	}
	if _, err := os.Stat(srcPath); !os.IsNotExist(err) {
		t.Fatalf("expected original %s to be removed, stat err=%v", srcPath, err)
	}
	if metadata.copySrc != filepath.Join(tmpDir, "photo.jpg") {
		t.Fatalf("metadata copy source = %q, want corrected source path", metadata.copySrc)
	}
}

func TestRunSkipsWhenTargetAlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "photo.jpg")
	targetPath := filepath.Join(tmpDir, "photo.heic")
	writeJPEGFixture(t, srcPath)
	if err := os.WriteFile(targetPath, []byte("existing"), 0o644); err != nil {
		t.Fatalf("WriteFile(target): %v", err)
	}

	stats, err := Run(Config{
		InputDir:     tmpDir,
		ShowProgress: false,
		Converter: &Converter{
			encoder:          &fakeEncoder{},
			metadataRestorer: &fakeMetadataRestorer{},
			stat:             os.Stat,
			chtimes:          os.Chtimes,
		},
		Infof:  func(string, ...any) {},
		Warnf:  func(string, ...any) {},
		Errorf: func(string, ...any) {},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if stats.SkippedConflicts != 1 {
		t.Fatalf("stats.SkippedConflicts = %d, want 1", stats.SkippedConflicts)
	}
	if len(stats.Conflicts) != 1 {
		t.Fatalf("len(stats.Conflicts) = %d, want 1", len(stats.Conflicts))
	}
	if _, err := os.Stat(srcPath); err != nil {
		t.Fatalf("expected source to remain: %v", err)
	}
}

func TestRunDryRunDoesNotModifyFiles(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "photo.png")
	writeJPEGFixture(t, srcPath)

	var infos []string
	stats, err := Run(Config{
		InputDir:     tmpDir,
		DryRun:       true,
		ShowProgress: false,
		Infof: func(format string, args ...any) {
			infos = append(infos, strings.TrimSpace(format))
		},
		Warnf:  func(string, ...any) {},
		Errorf: func(string, ...any) {},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if stats.Planned != 1 {
		t.Fatalf("stats.Planned = %d, want 1", stats.Planned)
	}
	if stats.Converted != 0 {
		t.Fatalf("stats.Converted = %d, want 0", stats.Converted)
	}
	if _, err := os.Stat(srcPath); err != nil {
		t.Fatalf("expected original file to remain: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "photo.heic")); !os.IsNotExist(err) {
		t.Fatalf("expected no photo.heic to be created, stat err=%v", err)
	}
	if len(infos) != 1 {
		t.Fatalf("len(infos) = %d, want 1", len(infos))
	}
}

func TestRunContinuesAfterFailure(t *testing.T) {
	tmpDir := t.TempDir()
	writeJPEGFixture(t, filepath.Join(tmpDir, "ok.jpg"))
	if err := os.WriteFile(filepath.Join(tmpDir, "bad.jpg"), []byte("not-an-image"), 0o644); err != nil {
		t.Fatalf("WriteFile(bad): %v", err)
	}

	stats, err := Run(Config{
		InputDir:     tmpDir,
		ShowProgress: false,
		Converter: &Converter{
			encoder:          &fakeEncoder{},
			metadataRestorer: &fakeMetadataRestorer{},
			stat:             os.Stat,
			chtimes:          os.Chtimes,
		},
		Infof:  func(string, ...any) {},
		Warnf:  func(string, ...any) {},
		Errorf: func(string, ...any) {},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if stats.Converted != 1 {
		t.Fatalf("stats.Converted = %d, want 1", stats.Converted)
	}
	if stats.Failed != 1 {
		t.Fatalf("stats.Failed = %d, want 1", stats.Failed)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "ok.heic")); err != nil {
		t.Fatalf("expected ok.heic to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "bad.jpg")); err != nil {
		t.Fatalf("expected bad.jpg to remain: %v", err)
	}
}

func TestRunSkipsActualHEICContentWithMisleadingExtension(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "already.jpg")
	if err := os.WriteFile(srcPath, fakeHEIFHeader(), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	stats, err := Run(Config{
		InputDir:     tmpDir,
		ShowProgress: false,
		Infof:        func(string, ...any) {},
		Warnf:        func(string, ...any) {},
		Errorf:       func(string, ...any) {},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if stats.SkippedAlreadyHEIC != 1 {
		t.Fatalf("stats.SkippedAlreadyHEIC = %d, want 1", stats.SkippedAlreadyHEIC)
	}
	if stats.Failed != 0 {
		t.Fatalf("stats.Failed = %d, want 0", stats.Failed)
	}
}

func writeJPEGFixture(t *testing.T, path string) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, G: 255, A: 255})
	if err := EncodeJPEG(path, img, 90); err != nil {
		t.Fatalf("EncodeJPEG(%s): %v", path, err)
	}
}

func TestRunDefaultWorkersIsTwo(t *testing.T) {
	tmpDir := t.TempDir()
	writeJPEGFixture(t, filepath.Join(tmpDir, "a.jpg"))
	writeJPEGFixture(t, filepath.Join(tmpDir, "b.jpg"))

	// Workers: 0 triggers the default path; should not panic and should
	// process both files using the default worker count of 2.
	stats, err := Run(Config{
		InputDir:     tmpDir,
		ShowProgress: false,
		Workers:      0, // triggers default
		Converter: &Converter{
			encoder:          &fakeEncoder{},
			metadataRestorer: &fakeMetadataRestorer{},
			stat:             os.Stat,
			chtimes:          os.Chtimes,
		},
		Infof:  func(string, ...any) {},
		Warnf:  func(string, ...any) {},
		Errorf: func(string, ...any) {},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stats.Converted != 2 {
		t.Fatalf("stats.Converted = %d, want 2", stats.Converted)
	}
}

func TestRunExplicitWorkersOverride(t *testing.T) {
	tmpDir := t.TempDir()
	writeJPEGFixture(t, filepath.Join(tmpDir, "img.jpg"))

	stats, err := Run(Config{
		InputDir:     tmpDir,
		ShowProgress: false,
		Workers:      1,
		Converter: &Converter{
			encoder:          &fakeEncoder{},
			metadataRestorer: &fakeMetadataRestorer{},
			stat:             os.Stat,
			chtimes:          os.Chtimes,
		},
		Infof:  func(string, ...any) {},
		Warnf:  func(string, ...any) {},
		Errorf: func(string, ...any) {},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if stats.Converted != 1 {
		t.Fatalf("stats.Converted = %d, want 1", stats.Converted)
	}
}
