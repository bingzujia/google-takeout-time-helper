package progress

import (
	"fmt"
	"strings"
)

func Info(format string, args ...any) {
	fmt.Printf("ℹ️  "+format+"\n", args...)
}

func Success(format string, args ...any) {
	fmt.Printf("✅ "+format+"\n", args...)
}

func Warning(format string, args ...any) {
	fmt.Printf("⚠️  "+format+"\n", args...)
}

func Error(format string, args ...any) {
	fmt.Printf("❌ "+format+"\n", args...)
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
	fmt.Printf("\r🔄 [%s] %d%% (%d/%d)", bar, pct, current, total)
}
