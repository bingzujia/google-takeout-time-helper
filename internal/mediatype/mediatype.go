package mediatype

import "strings"

// ImageExts is the set of supported image file extensions (lowercase, without dot).
var ImageExts = map[string]bool{
	"jpg": true, "jpeg": true, "png": true, "gif": true,
	"bmp": true, "tiff": true, "tif": true, "webp": true,
	"heic": true, "heif": true,
	"avif": true, "raw": true, "cr2": true, "nef": true, "arw": true, "dng": true,
}

// VideoExts is the set of supported video file extensions (lowercase, without dot).
var VideoExts = map[string]bool{
	"mp4": true, "mov": true, "avi": true, "mkv": true,
	"wmv": true, "flv": true, "3gp": true, "m4v": true,
	"webm": true, "mpg": true, "mpeg": true, "asf": true,
	"rm": true, "rmvb": true, "vob": true, "ts": true, "mts": true, "m2ts": true,
}

// HeicExts is the set of HEIC/HEIF file extensions (lowercase, without dot).
var HeicExts = map[string]bool{
	"heic": true, "heif": true,
}

// IsImage reports whether ext (with or without leading dot, any case) is a
// supported image extension.
func IsImage(ext string) bool {
	return ImageExts[normalize(ext)]
}

// IsVideo reports whether ext (with or without leading dot, any case) is a
// supported video extension.
func IsVideo(ext string) bool {
	return VideoExts[normalize(ext)]
}

// IsHEIC reports whether ext (with or without leading dot, any case) is a
// HEIC or HEIF extension.
func IsHEIC(ext string) bool {
	return HeicExts[normalize(ext)]
}

// normalize lowercases ext and strips a leading dot if present.
func normalize(ext string) string {
	ext = strings.ToLower(ext)
	return strings.TrimPrefix(ext, ".")
}
