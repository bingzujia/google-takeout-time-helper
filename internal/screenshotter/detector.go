package screenshotter

import (
	"os"
	"strings"
)

// detectScreenshots identifies screenshot files from directory entries
func detectScreenshots(entries []os.DirEntry) map[string]bool {
	screenshots := make(map[string]bool)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		lowerName := strings.ToLower(name)
		if strings.Contains(lowerName, "screenshot") {
			screenshots[name] = true
		}
	}
	return screenshots
}
