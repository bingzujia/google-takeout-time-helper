package qqmedia

import (
	"testing"
)

// TestIsImage_ImageFiles tests IsImage function with image files
func TestIsImage_ImageFiles(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     bool
		wantErr  bool
	}{
		{
			name:     "jpg file",
			filename: "photo.jpg",
			want:     true,
			wantErr:  false,
		},
		{
			name:     "png file",
			filename: "image.png",
			want:     true,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IsImage(tt.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsImage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("IsImage() got = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestIsImage_VideoFiles tests IsImage with video files
func TestIsImage_VideoFiles(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     bool
		wantErr  bool
	}{
		{
			name:     "mp4 file",
			filename: "video.mp4",
			want:     false,
			wantErr:  false,
		},
		{
			name:     "mov file",
			filename: "video.mov",
			want:     false,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IsImage(tt.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsImage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("IsImage() got = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestIsVideo_ImageFiles tests IsVideo with image files
func TestIsVideo_ImageFiles(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     bool
		wantErr  bool
	}{
		{
			name:     "jpg file",
			filename: "photo.jpg",
			want:     false,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IsVideo(tt.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsVideo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("IsVideo() got = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestIsVideo_VideoFiles tests IsVideo with video files
func TestIsVideo_VideoFiles(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     bool
		wantErr  bool
	}{
		{
			name:     "mp4 file",
			filename: "video.mp4",
			want:     true,
			wantErr:  false,
		},
		{
			name:     "mov file",
			filename: "video.mov",
			want:     true,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IsVideo(tt.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsVideo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("IsVideo() got = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestDetectMediaType_ImageNoExt tests media type detection without extension
func TestDetectMediaType_ImageNoExt(t *testing.T) {
	tests := []struct {
		name     string
		filepath string
		want     string
		wantErr  bool
	}{
		// Note: requires actual file content for MIME detection
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DetectMediaType(tt.filepath)
			if (err != nil) != tt.wantErr {
				t.Errorf("DetectMediaType() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("DetectMediaType() got = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestDetectMediaType_UnsupportedType tests with unsupported file types
func TestDetectMediaType_UnsupportedType(t *testing.T) {
	tests := []struct {
		name     string
		filepath string
		want     string
		wantErr  bool
	}{
		// Note: requires actual file content for MIME detection
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DetectMediaType(tt.filepath)
			if (err != nil) != tt.wantErr {
				t.Errorf("DetectMediaType() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("DetectMediaType() got = %v, want %v", got, tt.want)
			}
		})
	}
}
