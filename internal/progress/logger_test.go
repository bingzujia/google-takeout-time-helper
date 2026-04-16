package progress

import (
	"bytes"
	"strings"
	"testing"
)

func TestShouldUpdate(t *testing.T) {
	t.Run("small totals always update", func(t *testing.T) {
		if !ShouldUpdate(37, 999) {
			t.Fatal("ShouldUpdate(37, 999) = false, want true")
		}
	})

	t.Run("large totals throttle intermediate updates", func(t *testing.T) {
		if ShouldUpdate(37, 1000) {
			t.Fatal("ShouldUpdate(37, 1000) = true, want false")
		}
		if !ShouldUpdate(40, 1000) {
			t.Fatal("ShouldUpdate(40, 1000) = false, want true")
		}
		if !ShouldUpdate(1000, 1000) {
			t.Fatal("ShouldUpdate(1000, 1000) = false, want true")
		}
	})
}

func TestReporterRendersProgressAndFinishesLine(t *testing.T) {
	var buf bytes.Buffer
	restore := setOutput(&buf)
	defer restore()

	reporter := NewReporter(3, true)
	reporter.Update(1)
	reporter.Update(2)
	reporter.Update(3)
	reporter.Close()

	out := buf.String()
	if !strings.Contains(out, "(3/3)") {
		t.Fatalf("output %q does not contain final progress count", out)
	}
	if !strings.HasSuffix(out, "\n") {
		t.Fatalf("output %q does not end with newline", out)
	}
}
