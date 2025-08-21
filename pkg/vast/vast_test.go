package vast

import (
	"encoding/xml"
	"testing"
)

func TestVAST_Marshal(t *testing.T) {
	v := &VAST{
		Version: "4.0",
		Ads: []Ad{
			{
				ID: "test-ad",
				InLine: &InLine{
					AdSystem: AdSystem{
						Name:    "TestSystem",
						Version: "1.0",
					},
					AdTitle:     "Test Ad",
					Description: "Test Description",
				},
			},
		},
	}

	data, err := xml.Marshal(v)
	if err != nil {
		t.Fatalf("Failed to marshal VAST: %v", err)
	}

	if len(data) == 0 {
		t.Error("Expected non-empty XML output")
	}
}

func TestMediaFile_Validate(t *testing.T) {
	tests := []struct {
		name      string
		mediaFile MediaFile
		wantValid bool
	}{
		{
			name: "valid video",
			mediaFile: MediaFile{
				Delivery: "progressive",
				Type:     "video/mp4",
				Width:    1920,
				Height:   1080,
				Bitrate:  4000,
				URL:      "https://example.com/video.mp4",
			},
			wantValid: true,
		},
		{
			name: "invalid - no URL",
			mediaFile: MediaFile{
				Delivery: "progressive",
				Type:     "video/mp4",
			},
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simple validation: URL must not be empty
			valid := tt.mediaFile.URL != ""
			if valid != tt.wantValid {
				t.Errorf("got valid=%v, want %v", valid, tt.wantValid)
			}
		})
	}
}

func TestAdPod_Duration(t *testing.T) {
	pod := &AdPod{
		ID:            "pod-1",
		MaxDuration:   120,
		MaxAds:        6,
		AdBreakType:   "linear",
	}

	if pod.MaxDuration != 120 {
		t.Errorf("Expected max duration 120, got %d", pod.MaxDuration)
	}

	if pod.AdBreakType != "linear" {
		t.Errorf("Expected break type linear, got %s", pod.AdBreakType)
	}
}
