package tokenizer

import "testing"

func TestCountTokens(t *testing.T) {
	tests := []struct {
		name    string
		content string
		min     int
		max     int
	}{
		{"空字符串", "", 0, 0},
		{"简短英文", "Hello, world!", 2, 6},
		{"中文文本", "你好世界", 2, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CountTokens(tt.content, "gpt-4o")
			if got < tt.min || got > tt.max {
				t.Errorf("CountTokens(%q) = %d, 期望范围 [%d, %d]", tt.content, got, tt.min, tt.max)
			}
		})
	}
}
