package heicconv

import (
	"fmt"
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

	// Quality flag must be present with the default value "75".
	assertContainsSequence(t, args, "-q", "75")

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

func TestBuildHeifEncArgsCustomQuality(t *testing.T) {
	args := buildHeifEncArgs("in.jpg", "out.heic", EncodeOptions{Quality: 50})
	assertContainsSequence(t, args, "-q", "50")
}

func TestBuildHeifEncArgsZeroQualityFallsBackToDefault(t *testing.T) {
	args := buildHeifEncArgs("in.jpg", "out.heic", EncodeOptions{Quality: 0})
	assertContainsSequence(t, args, "-q", fmt.Sprintf("%d", heifEncQuality))
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

// TestDetectChromaSubsamplingAlwaysReturns420 verifies that chroma subsampling is always
// 4:2:0 to ensure the HEVC encoder uses Main or Main Still Picture profile, which is
// required for the "heic" brand. Passing 4:4:4 would produce Range Extensions profile
// (profile_idc=4), causing all major platform decoders to reject the output as corrupt.
func TestDetectChromaSubsamplingAlwaysReturns420(t *testing.T) {
	if got := detectChromaSubsampling(); got != "420" {
		t.Errorf("detectChromaSubsampling() = %q, want 420", got)
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
