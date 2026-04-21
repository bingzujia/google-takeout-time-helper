package cmd

import (
	"fmt"

	"github.com/bingzujia/google-takeout-time-helper/internal/logutil"
)

func printStats(renamed, skipped, errors int) {
	fmt.Printf("Renamed: %d, Skipped: %d, Errors: %d\n", renamed, skipped, errors)
}

func printLogPath(dryRun bool, logger *logutil.Logger) {
	if !dryRun {
		fmt.Printf("Log: %s\n", logger.Path())
	}
}
