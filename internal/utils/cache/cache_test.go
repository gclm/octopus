package cache

import (
	"sync"
	"testing"
)

func TestCache_基本操作(t *testing.T) {
	c := New[string, int](4)

	c.Set("a", 1)
	c.Set("b", 2)

	if v, ok := c.Get("a"); !ok || v != 1 {
		t.Fatalf("Get(a) = %d, %v; want 1, true", v, ok)
	}
	if v, ok := c.Get("missing"); ok {
		t.Fatalf("Get(missing) 应不存在，实际返回 %d", v)
	}
	if c.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", c.Len())
	}
}

func TestCache_覆盖写入(t *testing.T) {
	c := New[string, int](4)
	c.Set("a", 1)
	c.Set("a", 99)
	if v, _ := c.Get("a"); v != 99 {
		t.Fatalf("覆盖后 Get(a) = %d, want 99", v)
	}
}

func TestCache_Del(t *testing.T) {
	c := New[string, int](4)
	c.Set("a", 1)
	c.Set("b", 2)

	deleted := c.Del("a")
	if deleted != 1 {
		t.Fatalf("Del(a) 返回 %d, want 1", deleted)
	}
	if c.Exists("a") {
		t.Fatal("删除后 Exists(a) 应为 false")
	}
	if !c.Exists("b") {
		t.Fatal("Exists(b) 应为 true")
	}
	// 删除不存在的 key
	if c.Del("missing") != 0 {
		t.Fatal("Del 不存在的 key 应返回 0")
	}
}

func TestCache_GetAll(t *testing.T) {
	c := New[string, int](4)
	c.Set("a", 1)
	c.Set("b", 2)

	all := c.GetAll()
	if len(all) != 2 {
		t.Fatalf("GetAll 长度 = %d, want 2", len(all))
	}
	if all["a"] != 1 || all["b"] != 2 {
		t.Fatalf("GetAll 内容错误: %v", all)
	}
}

func TestCache_Clear(t *testing.T) {
	c := New[string, int](4)
	c.Set("a", 1)
	c.Set("b", 2)
	c.Clear()
	if c.Len() != 0 {
		t.Fatalf("Clear 后 Len() = %d, want 0", c.Len())
	}
}

func TestCache_并发安全(t *testing.T) {
	c := New[int, int](4)
	const n = 200
	var wg sync.WaitGroup
	wg.Add(n * 3)

	// 并发写
	for i := 0; i < n; i++ {
		go func(i int) { c.Set(i, i); wg.Done() }(i)
	}
	// 并发读
	for i := 0; i < n; i++ {
		go func(i int) { c.Get(i); wg.Done() }(i)
	}
	// 并发删
	for i := 0; i < n; i++ {
		go func(i int) { c.Del(i); wg.Done() }(i)
	}
	wg.Wait()
}

func TestCache_默认分片数(t *testing.T) {
	c := New[string, int](0)
	c.Set("a", 1)
	if v, ok := c.Get("a"); !ok || v != 1 {
		t.Fatal("shards=0 应回退到默认值")
	}
}
