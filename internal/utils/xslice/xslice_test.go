package xslice

import "testing"

func TestUnique(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{"nil 输入", nil, nil},
		{"空切片", []string{}, []string{}},
		{"无重复", []string{"a", "b", "c"}, []string{"a", "b", "c"}},
		{"有重复", []string{"a", "b", "a", "c", "b"}, []string{"a", "b", "c"}},
		{"全部相同", []string{"x", "x", "x"}, []string{"x"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Unique(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("got[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}

	// int 类型验证泛型
	intGot := Unique([]int{3, 1, 3, 2, 1})
	if len(intGot) != 3 || intGot[0] != 3 || intGot[1] != 1 || intGot[2] != 2 {
		t.Fatalf("int 去重结果: %v", intGot)
	}
}

func TestUniqueFunc(t *testing.T) {
	type user struct{ Name string }

	tests := []struct {
		name  string
		input []user
		want  []string
	}{
		{"nil 输入", nil, nil},
		{"无重复", []user{{"a"}, {"b"}}, []string{"a", "b"}},
		{"按 Name 去重", []user{{"a"}, {"b"}, {"a"}}, []string{"a", "b"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := UniqueFunc(tt.input, func(u user) string { return u.Name })
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i].Name != tt.want[i] {
					t.Fatalf("got[%d] = %q, want %q", i, got[i].Name, tt.want[i])
				}
			}
		})
	}
}
