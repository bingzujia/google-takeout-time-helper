package qqmedia

import (
	"testing"
)

// TestParseTimestamp_Pattern1_YYYYMMDDhhmmss tests _YYYYMMDD_HHMMSS format
func TestParseTimestamp_Pattern1_YYYYMMDDhhmmss(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     int64
		wantErr  bool
	}{
		{
			name:     "standard _20170709_002844",
			filename: "_20170709_002844.jpg",
			want:     1499560124000,
			wantErr:  false,
		},
		{
			name:     "pattern in middle",
			filename: "photo_20170709_002844_edited.jpg",
			want:     1499560124000,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTimestamp(tt.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTimestamp() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseTimestamp() got = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestParseTimestamp_Pattern2_13DigitUnixMs tests 13-digit Unix ms format
func TestParseTimestamp_Pattern2_13DigitUnixMs(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     int64
		wantErr  bool
	}{
		{
			name:     "standard 13 digits",
			filename: "qq_pic_merged_1688017744459.jpg",
			want:     1688017744459,
			wantErr:  false,
		},
		{
			name:     "13 digits at start",
			filename: "1688017744459_photo.jpg",
			want:     1688017744459,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTimestamp(tt.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTimestamp() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseTimestamp() got = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestParseTimestamp_Pattern3_QQVideo tests QQ video format
func TestParseTimestamp_Pattern3_QQVideo(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     int64
		wantErr  bool
	}{
		{
			name:     "QQ video standard",
			filename: "QQ视频20150720105516.mp4",
			want:     1437389716000,
			wantErr:  false,
		},
		{
			name:     "qq_video pattern",
			filename: "qq_video_20150720105516.mp4",
			want:     1437389716000,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTimestamp(tt.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTimestamp() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseTimestamp() got = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestParseTimestamp_Pattern4_Record tests Record_YYYY-MM-DD-HH-MM-SS format
func TestParseTimestamp_Pattern4_Record(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     int64
		wantErr  bool
	}{
		{
			name:     "Record standard",
			filename: "Record_2024-12-19-16-07-17.mov",
			want:     1734624437000,
			wantErr:  false,
		},
		{
			name:     "Record with suffix",
			filename: "Record_2024-12-19-16-07-17_001.mov",
			want:     1734624437000,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTimestamp(tt.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTimestamp() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseTimestamp() got = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestParseTimestamp_Pattern5_Snipaste tests Snipaste_YYYY-MM-DD_HH-MM-SS format
func TestParseTimestamp_Pattern5_Snipaste(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     int64
		wantErr  bool
	}{
		{
			name:     "Snipaste standard",
			filename: "Snipaste_2018-09-17_18-07-29.png",
			want:     1537207649000,
			wantErr:  false,
		},
		{
			name:     "Snipaste with prefix",
			filename: "my_Snipaste_2018-09-17_18-07-29.png",
			want:     1537207649000,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTimestamp(tt.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTimestamp() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseTimestamp() got = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestParseTimestamp_Pattern6_TBImageShare tests tb_image_share format
func TestParseTimestamp_Pattern6_TBImageShare(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     int64
		wantErr  bool
	}{
		{
			name:     "tb_image_share standard",
			filename: "tb_image_share_1661951220361.jpg",
			want:     1661951220361,
			wantErr:  false,
		},
		{
			name:     "tb_image_share with prefix",
			filename: "shared_tb_image_share_1661951220361.jpg",
			want:     1661951220361,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTimestamp(tt.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTimestamp() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseTimestamp() got = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestParseTimestamp_Pattern7_TIM tests TIM图片 format
func TestParseTimestamp_Pattern7_TIM(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     int64
		wantErr  bool
	}{
		{
			name:     "TIM standard",
			filename: "TIM图片20181215191143.jpg",
			want:     1544901103000,
			wantErr:  false,
		},
		{
			name:     "TIM with English",
			filename: "TIM_pic_20181215191143.jpg",
			want:     1544901103000,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTimestamp(tt.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTimestamp() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseTimestamp() got = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestParseTimestamp_Fallback_NoPattern tests no pattern match
func TestParseTimestamp_Fallback_NoPattern(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     int64
		wantErr  bool
	}{
		{
			name:     "no timestamp pattern",
			filename: "random_photo.jpg",
			want:     0,
			wantErr:  false,
		},
		{
			name:     "only extension",
			filename: ".jpg",
			want:     0,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTimestamp(tt.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTimestamp() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseTimestamp() got = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestGetFileModTime tests getting file modification time
func TestGetFileModTime(t *testing.T) {
	tests := []struct {
		name     string
		filepath string
		wantErr  bool
	}{
		// Note: requires actual file for mtime retrieval
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GetFileModTime(tt.filepath)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetFileModTime() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
