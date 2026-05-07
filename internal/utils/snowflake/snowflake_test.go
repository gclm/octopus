package snowflake

import (
	"sync"
	"testing"
)

func TestGenerateID_递增(t *testing.T) {
	id1 := GenerateID()
	id2 := GenerateID()
	if id2 <= id1 {
		t.Fatalf("id2(%d) 应大于 id1(%d)", id2, id1)
	}
}

func TestGenerateID_并发唯一(t *testing.T) {
	const n = 100
	ids := make([]int64, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(idx int) {
			ids[idx] = GenerateID()
			wg.Done()
		}(i)
	}
	wg.Wait()

	seen := make(map[int64]struct{}, n)
	for _, id := range ids {
		if _, dup := seen[id]; dup {
			t.Fatalf("重复 ID: %d", id)
		}
		seen[id] = struct{}{}
	}
}
