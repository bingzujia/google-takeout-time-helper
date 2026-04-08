package cleaner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config holds cleaner settings.
type Config struct {
	Dir    string
	DryRun bool
}

// Result holds counts after a Run.
type Result struct {
	Deleted int
	Failed  int
}

// Run recursively finds and deletes all *.json files under Dir.
func Run(cfg Config) (Result, error) {
	var result Result
	err := filepath.WalkDir(cfg.Dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.EqualFold(filepath.Ext(d.Name()), ".json") {
			return nil
		}
		if cfg.DryRun {
			fmt.Printf("  would delete: %s\n", path)
			result.Deleted++
			return nil
		}
		if err := os.Remove(path); err != nil {
			result.Failed++
		} else {
			result.Deleted++
		}
		return nil
	})
	return result, err
}
