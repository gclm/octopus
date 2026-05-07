package xurl

import "testing"

func TestIsDataURL(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"data:image/png;base64,abc", true},
		{"https://example.com", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsDataURL(tt.input); got != tt.want {
			t.Errorf("IsDataURL(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestParseDataURL(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		mediaType string
		isBase64  bool
		data      string
		isNil     bool
	}{
		{"base64 图片", "data:image/png;base64,iVBOR", "image/png", true, "iVBOR", false},
		{"纯文本", "data:text/plain,Hello", "text/plain", false, "Hello", false},
		{"默认类型", "data:,Hello", "text/plain", false, "Hello", false},
		{"非 data URL", "https://example.com", "", false, "", true},
		{"缺少逗号", "data:image/png", "", false, "", true},
		{"空字符串", "", "", false, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseDataURL(tt.input)
			if tt.isNil {
				if got != nil {
					t.Fatalf("期望 nil，实际 %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("期望非 nil")
			}
			if got.MediaType != tt.mediaType {
				t.Errorf("MediaType = %q, want %q", got.MediaType, tt.mediaType)
			}
			if got.IsBase64 != tt.isBase64 {
				t.Errorf("IsBase64 = %v, want %v", got.IsBase64, tt.isBase64)
			}
			if got.Data != tt.data {
				t.Errorf("Data = %q, want %q", got.Data, tt.data)
			}
		})
	}
}

func TestExtractBase64FromDataURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"data:image/png;base64,abc123", "abc123"},
		{"https://example.com/img.png", "https://example.com/img.png"},
		{"data:text/plain,Hello", "Hello"},
	}
	for _, tt := range tests {
		if got := ExtractBase64FromDataURL(tt.input); got != tt.want {
			t.Errorf("ExtractBase64FromDataURL(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestExtractMediaTypeFromDataURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"data:image/png;base64,abc", "image/png"},
		{"data:text/plain,Hello", "text/plain"},
		{"https://example.com", ""},
	}
	for _, tt := range tests {
		if got := ExtractMediaTypeFromDataURL(tt.input); got != tt.want {
			t.Errorf("ExtractMediaTypeFromDataURL(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
