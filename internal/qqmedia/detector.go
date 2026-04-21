package qqmedia

import (
	"io"
	"net/http"
	"os"
	"strings"
)

// IsImage checks if a file is an image based on filename and/or content
func IsImage(filename string) (bool, error) {
	mediaType, err := DetectMediaType(filename)
	if err != nil {
		return false, err
	}
	return mediaType == "image", nil
}

// IsVideo checks if a file is a video based on filename and/or content
func IsVideo(filename string) (bool, error) {
	mediaType, err := DetectMediaType(filename)
	if err != nil {
		return false, err
	}
	return mediaType == "video", nil
}

// DetectMediaType detects whether a file is an image or video
// Returns "image", "video", or "" if unsupported
func DetectMediaType(filepath string) (string, error) {
	// Try extension-based detection first for performance
	ext := strings.ToLower(extractExtension(filepath))
	if ext != "" {
		if isSupportedImageType(ext) {
			return "image", nil
		}
		if isSupportedVideoType(ext) {
			return "video", nil
		}
	}

	// Try content-based MIME detection
	file, err := os.Open(filepath)
	if err != nil {
		// If file doesn't exist or can't be opened, try extension fallback
		if ext != "" {
			return "", nil
		}
		return "", err
	}
	defer file.Close()

	// Read first 512 bytes for MIME detection
	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		return "", err
	}

	if n > 0 {
		mimeType := http.DetectContentType(buf[:n])
		if strings.HasPrefix(mimeType, "image/") {
			return "image", nil
		}
		if strings.HasPrefix(mimeType, "video/") {
			return "video", nil
		}
	}

	// No supported type detected
	return "", nil
}

// extractExtension extracts file extension from path
func extractExtension(filepath string) string {
	ext := strings.ToLower(filepath[len(filepath)-len(filepath):])
	dotIndex := strings.LastIndex(filepath, ".")
	if dotIndex > 0 {
		ext = filepath[dotIndex:]
	}
	return ext
}

// isSupportedImageType checks if extension is a supported image type
func isSupportedImageType(ext string) bool {
	supportedImage := []string{
		".jpg", ".jpeg", ".png", ".gif", ".heic", ".heif",
		".webp", ".bmp", ".tiff", ".tif",
	}
	for _, supported := range supportedImage {
		if strings.EqualFold(ext, supported) {
			return true
		}
	}
	return false
}

// isSupportedVideoType checks if extension is a supported video type
func isSupportedVideoType(ext string) bool {
	supportedVideo := []string{
		".mp4", ".mov", ".avi", ".mkv", ".webm", ".flv",
		".3gp", ".m4v", ".mpg", ".mpeg", ".mts", ".ts",
	}
	for _, supported := range supportedVideo {
		if strings.EqualFold(ext, supported) {
			return true
		}
	}
	return false
}
