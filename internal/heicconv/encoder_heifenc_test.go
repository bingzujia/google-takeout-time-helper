package heicconv

import (
	"slices"
	"strings"
	"testing"
	"time"
)

func TestEncodeTimeoutNormal(t *testing.T) {
	d := encodeTimeout(EncodeOptions{Oversized: false})
	if d != 5*time.Minute {
		t.Fatalf("normal timeout = %v, want 5m", d)
	}
}

func TestEncodeTimeoutOversized(t *testing.T) {
	d := encodeTimeout(EncodeOptions{Oversized: true})
	if d <= 5*time.Minute {
		t.Fatalf("oversized timeout = %v, want > 5m", d)
	}
	if d != 30*time.Minute {
		t.Fatalf("oversized timeout = %v, want 30m", d)
	}
}

func TestBuildHeifEncArgs(t *testing.T) {
	args := buildHeifEncArgs("in.jpg", "out.heic", EncodeOptions{})

	// Quality flag must be present with value "35".
	assertContainsSequence(t, args, "-q", "35")

	// Output flag must be present with the destination path.
	assertContainsSequence(t, args, "-o", "out.heic")

	// Source path must appear in the args.
	if !slices.Contains(args, "in.jpg") {
		t.Errorf("args %v do not contain source path %q", args, "in.jpg")
	}

	// Lossless flag must never be present.
	if slices.Contains(args, "-L") {
		t.Errorf("args %v must not contain lossless flag -L", args)
	}
}

func TestBuildHeifEncArgsChroma420(t *testing.T) {
	args := buildHeifEncArgs("in.jpg", "out.heic", EncodeOptions{ChromaSubsampling: "420"})
	assertContainsSequence(t, args, "-p", "chroma=420")
	if slices.Contains(args, "-L") {
		t.Errorf("args %v must not contain lossless flag -L", args)
	}
}

func TestBuildHeifEncArgsChroma422(t *testing.T) {
	args := buildHeifEncArgs("in.jpg", "out.heic", EncodeOptions{ChromaSubsampling: "422"})
	assertContainsSequence(t, args, "-p", "chroma=422")
}

func TestBuildHeifEncArgsChroma444(t *testing.T) {
	args := buildHeifEncArgs("in.jpg", "out.heic", EncodeOptions{ChromaSubsampling: "444"})
	assertContainsSequence(t, args, "-p", "chroma=444")
}

func TestBuildHeifEncArgsNoChromaWhenEmpty(t *testing.T) {
	args := buildHeifEncArgs("in.jpg", "out.heic", EncodeOptions{})
	if slices.Contains(args, "--chroma") {
		t.Errorf("args %v must not contain --chroma when ChromaSubsampling is empty", args)
	}
	for _, a := range args {
		if strings.HasPrefix(a, "chroma=") {
			t.Errorf("args %v must not contain chroma= parameter when ChromaSubsampling is empty", args)
		}
	}
}

func TestBuildHeifEncArgsLosslessNeverPresent(t *testing.T) {
	for _, opts := range []EncodeOptions{
		{},
		{ChromaSubsampling: "420"},
		{ChromaSubsampling: "422"},
		{ChromaSubsampling: "444"},
		{Oversized: true},
	} {
		args := buildHeifEncArgs("in.jpg", "out.heic", opts)
		if slices.Contains(args, "-L") {
			t.Errorf("args %v must not contain lossless flag -L (opts=%+v)", args, opts)
		}
	}
}

func TestParseChromaSubsampling(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "4:2:0 typical camera JPEG",
			input: `[{"SourceFile":"photo.jpg","YCbCrSubSampling":"YCbCr4:2:0 (2 2)"}]`,
			want:  "420",
		},
		{
			name:  "4:2:2",
			input: `[{"SourceFile":"photo.jpg","YCbCrSubSampling":"YCbCr4:2:2 (2 1)"}]`,
			want:  "422",
		},
		{
			name:  "4:4:4",
			input: `[{"SourceFile":"photo.jpg","YCbCrSubSampling":"YCbCr4:4:4 (1 1)"}]`,
			want:  "444",
		},
		{
			name:  "missing YCbCrSubSampling tag falls back to 420",
			input: `[{"SourceFile":"photo.jpg"}]`,
			want:  "420",
		},
		{
			name:  "empty JSON array falls back to 420",
			input: `[]`,
			want:  "420",
		},
		{
			name:  "invalid JSON falls back to 420",
			input: `not json`,
			want:  "420",
		},
		{
			name:  "unrecognised value falls back to 420",
			input: `[{"YCbCrSubSampling":"unknown"}]`,
			want:  "420",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseChromaSubsampling(tc.input)
			if got != tc.want {
				t.Errorf("parseChromaSubsampling(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestDetectChromaSubsamplingNonJPEG(t *testing.T) {
	formats := []string{"png", "bmp", "gif", "tiff", "webp"}
	for _, fmt := range formats {
		got := detectChromaSubsampling("any.file", fmt)
		if got != "420" {
			t.Errorf("detectChromaSubsampling for format %q = %q, want 420", fmt, got)
		}
	}
}

// assertContainsSequence checks that consecutive elements key, value appear in args.
func assertContainsSequence(t *testing.T, args []string, key, value string) {
	t.Helper()
	for i := 0; i+1 < len(args); i++ {
		if args[i] == key && args[i+1] == value {
			return
		}
	}
	t.Errorf("args %v do not contain sequence [%q %q]", args, key, value)
}
