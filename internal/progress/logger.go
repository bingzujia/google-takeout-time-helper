package progress

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

var (
	outputMu       sync.Mutex
	output         io.Writer = os.Stdout
	progressActive bool
)

func Info(format string, args ...any) {
	printLine("ℹ️  ", format, args...)
}

func Success(format string, args ...any) {
	printLine("✅ ", format, args...)
}

func Warning(format string, args ...any) {
	printLine("⚠️  ", format, args...)
}

func Error(format string, args ...any) {
	printLine("❌ ", format, args...)
}

// PrintProgress prints a progress bar to stdout using carriage return.
func PrintProgress(current, total int) {
	if total == 0 {
		return
	}
	pct := current * 100 / total
	barWidth := 20
	filled := pct * barWidth / 100
	bar := strings.Repeat("+", filled) + strings.Repeat("-", barWidth-filled)
	outputMu.Lock()
	defer outputMu.Unlock()
	fmt.Fprintf(output, "\r🔄 [%s] %d%% (%d/%d)", bar, pct, current, total)
	progressActive = true
}

// ShouldUpdate determines whether to refresh the progress bar.
func ShouldUpdate(current, total int) bool {
	if total < 1000 {
		return true
	}
	return current%10 == 0 || current == total
}

type Reporter struct {
	total   int
	enabled bool
	updates chan int
	wg      sync.WaitGroup
}

func NewReporter(total int, enabled bool) *Reporter {
	r := &Reporter{
		total:   total,
		enabled: enabled && total > 0,
	}
	if !r.enabled {
		return r
	}

	r.updates = make(chan int, 64)
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		last := 0
		for cur := range r.updates {
			if cur > last {
				last = cur
				PrintProgress(cur, total)
			}
		}
		FinishProgress()
	}()
	return r
}

func (r *Reporter) Update(current int) {
	if !r.enabled || !ShouldUpdate(current, r.total) {
		return
	}
	r.updates <- current
}

func (r *Reporter) Close() {
	if !r.enabled {
		return
	}
	close(r.updates)
	r.wg.Wait()
}

func FinishProgress() {
	outputMu.Lock()
	defer outputMu.Unlock()
	if progressActive {
		fmt.Fprintln(output)
		progressActive = false
	}
}

func printLine(prefix, format string, args ...any) {
	outputMu.Lock()
	defer outputMu.Unlock()
	if progressActive {
		fmt.Fprintln(output)
		progressActive = false
	}
	fmt.Fprintf(output, prefix+format+"\n", args...)
}

func setOutput(w io.Writer) func() {
	outputMu.Lock()
	prev := output
	output = w
	progressActive = false
	outputMu.Unlock()

	return func() {
		outputMu.Lock()
		output = prev
		progressActive = false
		outputMu.Unlock()
	}
}
