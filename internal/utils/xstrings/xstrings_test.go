package xstrings

import "testing"

func TestSplitTrimCompact(t *testing.T) {
	tests := []struct {
		name  string
		sep   string
		parts []string
		want  []string
	}{
		{"空输入", ",", nil, nil},
		{"单个值", ",", []string{"a"}, []string{"a"}},
		{"逗号分隔带空格", ",", []string{"a, b, ,c,"}, []string{"a", "b", "c"}},
		{"多个 part 合并", ",", []string{"a,b", "c,d"}, []string{"a", "b", "c", "d"}},
		{"含空字符串 part", ",", []string{"", "a", ""}, []string{"a"}},
		{"全空白", ",", []string{" ", " , ", ""}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitTrimCompact(tt.sep, tt.parts...)
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("got[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestTrimCompact(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{"nil 输入", nil, nil},
		{"混合空白和空串", []string{" a ", "", "b", "  "}, []string{"a", "b"}},
		{"全部有效", []string{"a", "b"}, []string{"a", "b"}},
		{"全部空白", []string{" ", ""}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TrimCompact(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("got[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
