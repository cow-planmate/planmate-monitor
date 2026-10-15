package cache

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestConcurrentMissesLoadOnce(t *testing.T) {
	c := New(time.Minute)
	var calls atomic.Int32
	release := make(chan struct{})

	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, err := Get(context.Background(), c, "k", func(context.Context) (int, error) {
				calls.Add(1)
				<-release
				return 42, nil
			})
			if err != nil || v != 42 {
				t.Errorf("v=%d err=%v", v, err)
			}
		}()
	}
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()

	if n := calls.Load(); n != 1 {
		t.Fatalf("동시 요청 10개가 load 를 %d번 불렀다, 1번이어야 한다", n)
	}
}

func TestExpiredEntryReloads(t *testing.T) {
	c := New(time.Second)
	now := time.Now()
	c.now = func() time.Time { return now }

	load := func(v int) func(context.Context) (int, error) {
		return func(context.Context) (int, error) { return v, nil }
	}
	if v, _ := Get(context.Background(), c, "k", load(1)); v != 1 {
		t.Fatalf("got %d", v)
	}
	if v, _ := Get(context.Background(), c, "k", load(2)); v != 1 {
		t.Fatalf("TTL 안에서는 캐시된 값이어야 한다, got %d", v)
	}
	now = now.Add(2 * time.Second)
	if v, _ := Get(context.Background(), c, "k", load(3)); v != 3 {
		t.Fatalf("만료 후에는 다시 load 해야 한다, got %d", v)
	}
}

func TestErrorsAreNotCached(t *testing.T) {
	c := New(time.Minute)
	_, err := Get(context.Background(), c, "k", func(context.Context) (int, error) { return 0, errors.New("boom") })
	if err == nil {
		t.Fatal("에러가 전달돼야 한다")
	}
	v, err := Get(context.Background(), c, "k", func(context.Context) (int, error) { return 7, nil })
	if err != nil || v != 7 {
		t.Fatalf("에러 뒤에는 다시 load 해야 한다, v=%d err=%v", v, err)
	}
}
