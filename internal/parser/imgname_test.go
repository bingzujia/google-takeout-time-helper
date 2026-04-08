package parser

import (
	"testing"
	"time"
)

func TestParseIMGVIDFilename(t *testing.T) {
	utc := time.UTC
	cases := []struct {
		filename string
		want     time.Time
		ok       bool
	}{
		{"IMG20250409084814.MOV", time.Date(2025, 4, 9, 8, 48, 14, 0, utc), true},
		{"IMG_20250727_141938.MOV", time.Date(2025, 7, 27, 14, 19, 38, 0, utc), true},
		{"VID20201231235959.mp4", time.Date(2020, 12, 31, 23, 59, 59, 0, utc), true},
		{"VID_20211015_100000.mp4", time.Date(2021, 10, 15, 10, 0, 0, 0, utc), true},
		// lowercase
		{"img20220101120000.jpg", time.Date(2022, 1, 1, 12, 0, 0, 0, utc), true},
		// no match
		{"photo.jpg", time.Time{}, false},
		{"Screenshot_2021-01-01.png", time.Time{}, false},
	}

	for _, tc := range cases {
		t.Run(tc.filename, func(t *testing.T) {
			got, ok := ParseIMGVIDFilename(tc.filename)
			if ok != tc.ok {
				t.Fatalf("ok=%v want %v for %q", ok, tc.ok, tc.filename)
			}
			if ok && !got.Equal(tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}
