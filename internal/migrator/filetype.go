package migrator

import (
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// FileType holds the result of file type detection.
type FileType struct {
	MimeType  string
	NewExt    string // target extension if rename needed, empty if no rename
	Supported bool   // whether exiftool can write to this format
}

var fileTypeCacheMu sync.Mutex
var fileTypeCache = make(map[string]*FileType)

// DetectFileAll runs the `file` command once and returns type info.
// Results are cached per file path to avoid redundant calls.
func DetectFileAll(filePath string) (*FileType, error) {
	fileTypeCacheMu.Lock()
	if cached, ok := fileTypeCache[filePath]; ok {
		fileTypeCacheMu.Unlock()
		return cached, nil
	}
	fileTypeCacheMu.Unlock()

	cmd := exec.Command("file", "--brief", "--mime-type", filePath)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	mimeType := strings.TrimSpace(string(out))
	currentExt := strings.ToLower(filepath.Ext(filePath))
	targetExt := mimeToExt(mimeType)
	supported := mimeType != "video/x-ms-wmv" && mimeType != "image/webp" && mimeType != "video/x-ms-asf"

	var newExt string
	if targetExt != "" && targetExt != currentExt {
		newExt = targetExt
	}

	result := &FileType{
		MimeType:  mimeType,
		NewExt:    newExt,
		Supported: supported,
	}

	fileTypeCacheMu.Lock()
	fileTypeCache[filePath] = result
	fileTypeCacheMu.Unlock()

	return result, nil
}

// DetectFileType returns the correct extension if the actual file type
// doesn't match the current extension. Returns empty string if no rename is needed.
func DetectFileType(filePath string) (newExt string, err error) {
	ft, err := DetectFileAll(filePath)
	if err != nil {
		return "", err
	}
	return ft.NewExt, nil
}

// IsWriteSupported checks if exiftool can write to the given file type.
func IsWriteSupported(filePath string) bool {
	ft, err := DetectFileAll(filePath)
	if err != nil {
		return true // assume supported if we can't detect
	}
	return ft.Supported
}

// mimeToExt maps MIME types to file extensions.
func mimeToExt(mime string) string {
	switch {
	case mime == "image/jpeg":
		return ".jpg"
	case mime == "image/png":
		return ".png"
	case mime == "image/gif":
		return ".gif"
	case mime == "image/webp":
		return ".webp"
	case mime == "image/tiff":
		return ".tiff"
	case mime == "image/bmp":
		return ".bmp"
	case mime == "image/heic" || mime == "image/heif":
		return ".heic"
	case mime == "video/mp4":
		return ".mp4"
	case mime == "video/quicktime":
		return ".mov"
	case mime == "video/x-msvideo":
		return ".avi"
	case mime == "video/x-matroska":
		return ".mkv"
	case mime == "video/x-ms-wmv":
		return ".wmv"
	case mime == "video/x-flv":
		return ".flv"
	case mime == "video/3gpp":
		return ".3gp"
	case mime == "video/x-ms-asf":
		return ".asf"
	case mime == "video/mpeg":
		return ".mpg"
	default:
		return ""
	}
}
