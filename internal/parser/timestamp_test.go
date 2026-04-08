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
		// 1. IMG_20230302_112040
		{"IMG_20230302_112040.jpg", time.Date(2023, 3, 2, 11, 20, 40, 0, utc), true},
		{"VID_20220101_000000.mp4", time.Date(2022, 1, 1, 0, 0, 0, 0, utc), true},
		// 2. IMG20230123102606
		{"IMG20230123102606.jpg", time.Date(2023, 1, 23, 10, 26, 6, 0, utc), true},
		{"VID20200615183045.mp4", time.Date(2020, 6, 15, 18, 30, 45, 0, utc), true},
		// 3. WP_20131010_074
		{"WP_20131010_074.jpg", time.Date(2013, 10, 10, 7, 40, 0, 0, utc), true},
		{"WP_20131010_074520.jpg", time.Date(2013, 10, 10, 7, 45, 20, 0, utc), true},
		// 4. 20151120_120004
		{"20151120_120004.jpg", time.Date(2015, 11, 20, 12, 0, 4, 0, utc), true},
		// 5. 20151120_120004~2
		{"20151120_120004~2.jpg", time.Date(2015, 11, 20, 12, 0, 4, 0, utc), true},
		// 6. Screenshot_2016-02-28-13-06-34
		{"Screenshot_2016-02-28-13-06-34.png", time.Date(2016, 2, 28, 13, 6, 34, 0, utc), true},
		// 7. Screenshot_20210803-084525
		{"Screenshot_20210803-084525.png", time.Date(2021, 8, 3, 8, 45, 25, 0, utc), true},
		// 8. mmexport1491013330299
		{"mmexport1491013330299.jpg", time.Unix(1491013330, 0).UTC(), true},
		// 9. mmexport with suffix
		{"mmexport1491013330299-已修改.jpg", time.Unix(1491013330, 0).UTC(), true},
		{"mmexport1491013330299-edited.jpg", time.Unix(1491013330, 0).UTC(), true},
		// 10. mmexport with numbered suffix
		{"mmexport1491013330299(1).jpg", time.Unix(1491013330, 0).UTC(), true},
		// edge cases
		{"photo.jpg", time.Time{}, false},
		{"random_name.png", time.Time{}, false},
		{"DSC_20200101_120000.jpg", time.Date(2020, 1, 1, 12, 0, 0, 0, utc), true},
		{"PXL_20211215_093045.jpg", time.Date(2021, 12, 15, 9, 30, 45, 0, utc), true},
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
