package mediatype_test

import (
	"testing"

	"github.com/bingzujia/google-takeout-time-helper/internal/mediatype"
)

func TestIsImage(t *testing.T) {
	cases := []struct {
		ext  string
		want bool
	}{
		{"jpg", true}, {"jpeg", true}, {"png", true}, {"heic", true}, {"heif", true},
		{"webp", true}, {"tiff", true}, {"gif", true}, {"bmp", true}, {"avif", true},
		{"raw", true}, {"cr2", true}, {"nef", true}, {"arw", true}, {"dng", true},
		{"mp4", false}, {"pdf", false}, {"txt", false},
		// with dot
		{".jpg", true}, {".PNG", true},
		// uppercase
		{"JPG", true}, {"HEIC", true},
	}
	for _, c := range cases {
		got := mediatype.IsImage(c.ext)
		if got != c.want {
			t.Errorf("IsImage(%q) = %v, want %v", c.ext, got, c.want)
		}
	}
}

func TestIsVideo(t *testing.T) {
	cases := []struct {
		ext  string
		want bool
	}{
		{"mp4", true}, {"mov", true}, {"avi", true}, {"mkv", true}, {"3gp", true},
		{"m4v", true}, {"wmv", true}, {"flv", true}, {"webm", true},
		{"jpg", false}, {"pdf", false},
		{".mp4", true}, {"MP4", true},
	}
	for _, c := range cases {
		got := mediatype.IsVideo(c.ext)
		if got != c.want {
			t.Errorf("IsVideo(%q) = %v, want %v", c.ext, got, c.want)
		}
	}
}

func TestIsHEIC(t *testing.T) {
	cases := []struct {
		ext  string
		want bool
	}{
		{"heic", true}, {"heif", true},
		{"HEIC", true}, {".heic", true},
		{"jpg", false}, {"mp4", false},
	}
	for _, c := range cases {
		got := mediatype.IsHEIC(c.ext)
		if got != c.want {
			t.Errorf("IsHEIC(%q) = %v, want %v", c.ext, got, c.want)
		}
	}
}

func TestExportedMaps(t *testing.T) {
	for _, ext := range []string{"jpg", "jpeg", "png", "heic", "heif", "webp", "tiff", "gif", "bmp"} {
		if !mediatype.ImageExts[ext] {
			t.Errorf("ImageExts missing %q", ext)
		}
	}
	for _, ext := range []string{"mp4", "mov", "avi", "mkv", "3gp"} {
		if !mediatype.VideoExts[ext] {
			t.Errorf("VideoExts missing %q", ext)
		}
	}
}
