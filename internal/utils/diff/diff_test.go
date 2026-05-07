package diff

import "testing"

func TestDiff(t *testing.T) {
	tests := []struct {
		name        string
		old         []string
		new         []string
		wantDeleted []string
		wantAdded   []string
	}{
		{"两端为空", nil, nil, nil, nil},
		{"old 为空", nil, []string{"a", "b"}, nil, []string{"a", "b"}},
		{"new 为空", []string{"a", "b"}, nil, []string{"a", "b"}, nil},
		{"无变化", []string{"a", "b"}, []string{"a", "b"}, nil, nil},
		{"删除", []string{"a", "b", "c"}, []string{"a"}, []string{"b", "c"}, nil},
		{"新增", []string{"a"}, []string{"a", "b", "c"}, nil, []string{"b", "c"}},
		{"替换", []string{"a", "b"}, []string{"b", "c"}, []string{"a"}, []string{"c"}},
		{"重复元素", []string{"a", "a", "b"}, []string{"a", "c", "c"}, []string{"a", "b"}, []string{"c", "c"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deleted, added := Diff(tt.old, tt.new)
			if !equal(deleted, tt.wantDeleted) {
				t.Errorf("deleted = %v, want %v", deleted, tt.wantDeleted)
			}
			if !equal(added, tt.wantAdded) {
				t.Errorf("added = %v, want %v", added, tt.wantAdded)
			}
		})
	}

	// int 类型验证泛型
	del, add := Diff([]int{1, 2, 3}, []int{2, 4})
	if !equal(del, []int{1, 3}) || !equal(add, []int{4}) {
		t.Fatalf("int Diff: deleted=%v added=%v", del, add)
	}
}

func equal[T comparable](a, b []T) bool {
	if len(a) != len(b) {
		return false
	}
	// Diff 的输出不保证顺序，用计数比较
	ca := make(map[T]int, len(a))
	for _, v := range a {
		ca[v]++
	}
	cb := make(map[T]int, len(b))
	for _, v := range b {
		cb[v]++
	}
	if len(ca) != len(cb) {
		return false
	}
	for k, v := range ca {
		if cb[k] != v {
			return false
		}
	}
	return true
}
