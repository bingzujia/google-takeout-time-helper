package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bingzujia/g_photo_take_out_helper/internal/dedup"
	"github.com/bingzujia/g_photo_take_out_helper/internal/matcher"
	"github.com/bingzujia/g_photo_take_out_helper/internal/organizer"
	"github.com/bingzujia/g_photo_take_out_helper/internal/parser"
)

func main() {
	listMissing := flag.Bool("list-missing", false, "list all files that failed to match a JSON sidecar")
	verbose := flag.Bool("v", false, "show detailed timestamp and GPS source breakdown for every file")
	showFolders := flag.Bool("folders", false, "show year/album folder classification")
	dedupMode := flag.Bool("dedup", false, "run image deduplication on the directory")
	dedupThreshold := flag.Int("dedup-threshold", 10, "max hash distance for dedup (0-64, lower=stricter)")
	dedupOutput := flag.String("dedup-output", "", "write dedup results to JSON file")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Println("Usage: test_matcher [--list-missing] [-v] [-folders] [-dedup] [-dedup-threshold N] [-dedup-output FILE] <directory>")
		os.Exit(1)
	}

	dir := flag.Arg(0)

	// --- Dedup mode ---
	if *dedupMode {
		runDedup(dir, *dedupThreshold, *dedupOutput)
		return
	}

	// --- Folder classification ---
	if *showFolders {
		yearFolders, albumFolders, err := organizer.ClassifyFolder(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error classifying folders: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Scanning: %s\n\n", dir)
		fmt.Printf("Year folders (%d):\n", len(yearFolders))
		for _, f := range yearFolders {
			fmt.Printf("  %s\n", f)
		}
		fmt.Printf("\nAlbum folders (%d):\n", len(albumFolders))
		for _, f := range albumFolders {
			fmt.Printf("  %s\n", f)
		}
		fmt.Println()
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading directory: %v\n", err)
		os.Exit(1)
	}

	// Collect all non-JSON files
	var photos []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.EqualFold(filepath.Ext(e.Name()), ".json") {
			photos = append(photos, e.Name())
		}
	}

	fmt.Printf("Found %d photo files\n\n", len(photos))

	// Counters
	jsonFound := 0
	jsonNotFound := 0
	exifTimeHit := 0
	filenameTimeHit := 0
	jsonTimeHit := 0
	allZeroTime := 0
	exifGPSHit := 0
	jsonGPSHit := 0
	noGPS := 0
	var missing []string

	for _, name := range photos {
		photoPath := filepath.Join(dir, name)
		jsonResult := matcher.JSONForFile(photoPath)

		// --- Timestamp resolution ---
		exifTime, exifTimeOk := parser.ParseEXIFTimestamp(photoPath)
		filenameTime, filenameTimeOk := parser.ParseFilenameTimestamp(name)
		jsonTime := time.Time{}
		if jsonResult != nil && !jsonResult.Timestamp.IsZero() {
			jsonTime = jsonResult.Timestamp
		}

		resolvedTime := time.Time{}
		timeSource := "none"
		if exifTimeOk {
			resolvedTime = exifTime
			timeSource = "EXIF"
			exifTimeHit++
		} else if filenameTimeOk {
			resolvedTime = filenameTime
			timeSource = "Filename"
			filenameTimeHit++
		} else if !jsonTime.IsZero() {
			resolvedTime = jsonTime
			timeSource = "JSON"
			jsonTimeHit++
		} else {
			allZeroTime++
		}

		// --- GPS resolution ---
		var gps parser.GPSInfo
		gpsSource := "none"
		if jsonResult != nil && jsonResult.GooglePhoto != nil {
			gps = matcher.ResolveGPS(photoPath, jsonResult.GooglePhoto)
		} else {
			gps = parser.ParseEXIFGPS(photoPath)
		}

		if gps.Has {
			exifGPS := parser.ParseEXIFGPS(photoPath)
			if exifGPS.Has {
				gpsSource = "EXIF"
				exifGPSHit++
			} else if jsonResult != nil && jsonResult.GooglePhoto != nil && (jsonResult.GooglePhoto.GeoData.Latitude != 0 || jsonResult.GooglePhoto.GeoData.Longitude != 0) {
				gpsSource = "JSON"
				jsonGPSHit++
			}
		} else {
			noGPS++
		}

		// --- Output ---
		if jsonResult == nil {
			jsonNotFound++
			if *verbose {
				fmt.Printf("  %-50s  JSON: no  | Time: %-10s (%-8s) | GPS: %-4s (%s)\n",
					name,
					timeStr(resolvedTime),
					timeSource,
					gpsStr(gps),
					gpsSource,
				)
			} else {
				fmt.Printf("  %-50s  JSON: no  | Time: %-10s (%-8s) | GPS: %-4s\n",
					name,
					timeStr(resolvedTime),
					timeSource,
					gpsStr(gps),
				)
			}
			missing = append(missing, name)
		} else {
			jsonFound++
			jsonName := filepath.Base(jsonResult.JSONFile)
			if *verbose {
				fmt.Printf("  %-50s  JSON: yes (%-40s) | Time: %-10s (%-8s) | GPS: %-4s (%s)\n",
					name,
					trunc(jsonName, 40),
					timeStr(resolvedTime),
					timeSource,
					gpsStr(gps),
					gpsSource,
				)
			} else {
				fmt.Printf("  %-50s  JSON: yes | Time: %-10s (%-8s) | GPS: %-4s\n",
					name,
					timeStr(resolvedTime),
					timeSource,
					gpsStr(gps),
				)
			}
		}
	}

	fmt.Printf("\nResults: %d files\n", len(photos))
	fmt.Printf("  JSON matched:    %d\n", jsonFound)
	fmt.Printf("  JSON missing:    %d\n", jsonNotFound)
	fmt.Printf("\nTimestamp sources:\n")
	fmt.Printf("  EXIF:            %d\n", exifTimeHit)
	fmt.Printf("  Filename:        %d\n", filenameTimeHit)
	fmt.Printf("  JSON fallback:   %d\n", jsonTimeHit)
	fmt.Printf("  No timestamp:    %d\n", allZeroTime)
	fmt.Printf("\nGPS sources:\n")
	fmt.Printf("  EXIF:            %d\n", exifGPSHit)
	fmt.Printf("  JSON fallback:   %d\n", jsonGPSHit)
	fmt.Printf("  No GPS:          %d\n", noGPS)

	if *listMissing && len(missing) > 0 {
		fmt.Printf("\nMissing JSON sidecars (%d):\n", len(missing))
		for _, name := range missing {
			fmt.Printf("  - %s\n", name)
		}
	}
}

// --- Dedup ---

func runDedup(dir string, threshold int, outputPath string) {
	cfg := dedup.DefaultConfig()
	cfg.Threshold = threshold
	cfg.Recursive = true

	fmt.Printf("Scanning %s for duplicate images...\n", dir)
	fmt.Printf("Threshold: %d (both pHash AND dHash must match)\n\n", threshold)

	result, err := dedup.Run(dir, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Dedup error: %v\n", err)
		os.Exit(1)
	}

	if result.TotalGroups == 0 {
		fmt.Printf("No duplicates found among %d images.\n", result.TotalScanned)
		return
	}

	for i, group := range result.Groups {
		fmt.Printf("Duplicate group %d (%d files):\n", i+1, len(group.Files))
		for j, f := range group.Files {
			marker := "  "
			if j == group.Keep {
				marker = "  [KEEP] "
			} else {
				marker = "  [DUP]  "
			}
			fmt.Printf("    %s%s (%d bytes, %dx%d)\n", marker, filepath.Base(f.Path), f.Size, f.Width, f.Height)
		}
	}

	fmt.Println()
	fmt.Printf("Summary:\n")
	fmt.Printf("  Scanned:          %d images\n", result.TotalScanned)
	fmt.Printf("  Duplicate groups: %d\n", result.TotalGroups)
	fmt.Printf("  Total duplicates: %d\n", result.TotalDupes)
	fmt.Printf("  Reclaimable space: %s\n", formatBytes(result.SpaceReclaim))

	if len(result.Errors) > 0 {
		fmt.Printf("  Errors: %d files failed\n", len(result.Errors))
		for _, e := range result.Errors {
			fmt.Printf("    - %s: %s\n", filepath.Base(e.Path), e.Error)
		}
	}

	if outputPath != "" {
		if err := writeDedupJSON(outputPath, result); err != nil {
			fmt.Fprintf(os.Stderr, "Write output error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("\nResults written to %s\n", outputPath)
	}
}

func writeDedupJSON(path string, result *dedup.Result) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	type jsonGroup struct {
		Files []string `json:"files"`
		Keep  string   `json:"keep"`
	}
	type jsonResult struct {
		TotalScanned int         `json:"total_scanned"`
		TotalGroups  int         `json:"total_groups"`
		TotalDupes   int         `json:"total_duplicates"`
		SpaceReclaim int64       `json:"reclaimable_bytes"`
		Groups       []jsonGroup `json:"groups"`
		Errors       []string    `json:"errors"`
	}

	jr := jsonResult{
		TotalScanned: result.TotalScanned,
		TotalGroups:  result.TotalGroups,
		TotalDupes:   result.TotalDupes,
		SpaceReclaim: result.SpaceReclaim,
	}
	for _, g := range result.Groups {
		jg := jsonGroup{Keep: g.Files[g.Keep].Path}
		for _, f := range g.Files {
			jg.Files = append(jg.Files, f.Path)
		}
		jr.Groups = append(jr.Groups, jg)
	}
	for _, e := range result.Errors {
		jr.Errors = append(jr.Errors, fmt.Sprintf("%s: %s", e.Path, e.Error))
	}

	data, err := json.MarshalIndent(jr, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func timeStr(t time.Time) string {
	if t.IsZero() {
		return "zero"
	}
	return t.Format("2006-01-02 15:04:05")
}

func gpsStr(gps parser.GPSInfo) string {
	if !gps.Has {
		return "none"
	}
	return fmt.Sprintf("%.4f,%.4f", gps.Lat, gps.Lon)
}

func trunc(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
