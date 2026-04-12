package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bingzujia/g_photo_take_out_helper/internal/matcher"
	"github.com/bingzujia/g_photo_take_out_helper/internal/organizer"
	"github.com/bingzujia/g_photo_take_out_helper/internal/parser"
)

func main() {
	listMissing := flag.Bool("list-missing", false, "list all files that failed to match a JSON sidecar")
	verbose := flag.Bool("v", false, "show detailed timestamp and GPS source breakdown for every file")
	showFolders := flag.Bool("folders", false, "show year/album folder classification")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Println("Usage: test_matcher [--list-missing] [-v] [-folders] <directory>")
		os.Exit(1)
	}

	dir := flag.Arg(0)

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
