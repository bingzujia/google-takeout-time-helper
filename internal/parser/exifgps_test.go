package parser

import (
	"testing"
)

func TestParseGPSCoords(t *testing.T) {
	cases := []struct {
		input   string
		wantLat float64
		wantLon float64
		wantAlt float64
		wantOK  bool
	}{
		{"37.7749, -122.4194", 37.7749, -122.4194, 0, true},
		{"37.7749, -122.4194, 50.0", 37.7749, -122.4194, 50.0, true},
		{"0, 0", 0, 0, 0, true},
		{"37.7749", 0, 0, 0, false},
		{"", 0, 0, 0, false},
		{"abc, def", 0, 0, 0, false},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			lat, lon, alt, ok := parseGPSCoords(tc.input)
			if ok != tc.wantOK {
				t.Fatalf("ok=%v want %v", ok, tc.wantOK)
			}
			if ok && (lat != tc.wantLat || lon != tc.wantLon || alt != tc.wantAlt) {
				t.Errorf("got (%v, %v, %v), want (%v, %v, %v)", lat, lon, alt, tc.wantLat, tc.wantLon, tc.wantAlt)
			}
		})
	}
}

func TestParseEXIFGPS_NoGPS(t *testing.T) {
	// Non-image file should return Has=false
	info := ParseEXIFGPS("/dev/null")
	if info.Has {
		t.Error("expected Has=false for /dev/null")
	}
}

func TestParseEXIFGPS_RealImage(t *testing.T) {
	// Try to find a real image with GPS data
	testFiles := []string{
		"testdata/test.jpg",
		"testdata/test.jpeg",
	}

	for _, f := range testFiles {
		info := ParseEXIFGPS(f)
		if !info.Has {
			t.Logf("no GPS data in %s (may be expected)", f)
			return
		}

		// Sanity check: coordinates should be in valid ranges
		if info.Lat < -90 || info.Lat > 90 {
			t.Errorf("latitude %v out of range [-90, 90]", info.Lat)
		}
		if info.Lon < -180 || info.Lon > 180 {
			t.Errorf("longitude %v out of range [-180, 180]", info.Lon)
		}
		return
	}

	t.Log("no testdata images found, skipping EXIF GPS real-image test")
}
