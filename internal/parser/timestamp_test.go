package parser

import (
	"testing"
	"time"
)

func TestParseFilenameTimestamp(t *testing.T) {
	utc := time.UTC
	cases := []struct {
		filename string
		want     time.Time
		ok       bool
	}{
		// 1. IMG20250409084814 (anchored IMG/VID prefix)
		{"IMG20250409084814.jpg", time.Date(2025, 4, 9, 8, 48, 14, 0, utc), true},
		{"VID20200615183045.mp4", time.Date(2020, 6, 15, 18, 30, 45, 0, utc), true},
		{"img20220101120000.jpg", time.Date(2022, 1, 1, 12, 0, 0, 0, utc), true},

		// 2. IMG_20250727_141938 (anchored IMG/VID prefix with underscore)
		{"IMG_20230302_112040.jpg", time.Date(2023, 3, 2, 11, 20, 40, 0, utc), true},
		{"VID_20220101_000000.mp4", time.Date(2022, 1, 1, 0, 0, 0, 0, utc), true},

		// 3. (\d{8})_(\d{6}) — generic
		{"20151120_120004.jpg", time.Date(2015, 11, 20, 12, 0, 4, 0, utc), true},
		{"DSC_20200101_120000.jpg", time.Date(2020, 1, 1, 12, 0, 0, 0, utc), true},
		{"PXL_20211215_093045.jpg", time.Date(2021, 12, 15, 9, 30, 45, 0, utc), true},

		// 4. (\d{14}) — 14 consecutive digits
		{"IMG20230123102606.jpg", time.Date(2023, 1, 23, 10, 26, 6, 0, utc), true},

		// 5. (\d{8})_(\d{3,6}) — WP short time, default to 12:00:00
		{"WP_20131010_074.jpg", time.Date(2013, 10, 10, 12, 0, 0, 0, utc), true},
		{"WP_20131010_074520.jpg", time.Date(2013, 10, 10, 7, 45, 20, 0, utc), true},

		// 6. (\d{8})_(\d{6})~\d+ — burst photos
		{"20151120_120004~2.jpg", time.Date(2015, 11, 20, 12, 0, 4, 0, utc), true},

		// 7. YYYY-MM-DD-HH-mm-ss
		{"Screenshot_2016-02-28-13-06-34.png", time.Date(2016, 2, 28, 13, 6, 34, 0, utc), true},
		{"photo_2020-05-15-09-30-00.jpg", time.Date(2020, 5, 15, 9, 30, 0, 0, utc), true},

		// 8. YYYYMMDD-HHmmss
		{"Screenshot_20210803-084525.png", time.Date(2021, 8, 3, 8, 45, 25, 0, utc), true},

		// 9/10/11. mmexport (13-digit ms timestamp)
		{"mmexport1491013330299.jpg", time.Unix(1491013330, 0).UTC(), true},
		{"mmexport1491013330299-已修改.jpg", time.Unix(1491013330, 0).UTC(), true},
		{"mmexport1491013330299-edited.jpg", time.Unix(1491013330, 0).UTC(), true},
		{"mmexport1491013330299(1).jpg", time.Unix(1491013330, 0).UTC(), true},

		// edge cases — no match
		{"photo.jpg", time.Time{}, false},
		{"random_name.png", time.Time{}, false},

		// regression: hex-like long digit strings should not produce valid dates
		{"34290627826572a774e70a671.jpg", time.Time{}, false},

		// regression: invalid month/day should be rejected
		{"20251301_120000.jpg", time.Time{}, false}, // month=13
		{"20250132_120000.jpg", time.Time{}, false}, // day=32
		{"20250230_120000.jpg", time.Time{}, false}, // Feb 30 (Go normalizes, we reject)
	}

	for _, tc := range cases {
		t.Run(tc.filename, func(t *testing.T) {
			got, ok := ParseFilenameTimestamp(tc.filename)
			if ok != tc.ok {
				t.Fatalf("ok=%v want %v", ok, tc.ok)
			}
			if ok && !got.Equal(tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}
