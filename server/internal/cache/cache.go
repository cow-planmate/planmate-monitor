// Package cache 는 짧은 TTL 메모리 캐시다.
//
// 대시보드는 여러 탭이 30초마다 같은 지표를 다시 묻는다. 캐시가 만료되는 순간 요청이 몰려도
// 같은 키의 조회는 singleflight 로 한 번만 나가게 해서 DB/ES 부하가 늘지 않게 한다.
package cache

import (
	"context"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

type entry struct {
	value   any
	expires time.Time
}

type Cache struct {
	ttl   time.Duration
	now   func() time.Time
	mu    sync.Mutex
	items map[string]entry
	group singleflight.Group
}

func New(ttl time.Duration) *Cache {
	return &Cache{ttl: ttl, now: time.Now, items: make(map[string]entry)}
}

// Get 은 key 의 값을 돌려주고, 없거나 만료됐으면 load 로 채운다. 에러는 캐시하지 않는다.
func Get[T any](ctx context.Context, c *Cache, key string, load func(context.Context) (T, error)) (T, error) {
	if v, ok := c.lookup(key); ok {
		return v.(T), nil
	}
	v, err, _ := c.group.Do(key, func() (any, error) {
		if v, ok := c.lookup(key); ok {
			return v, nil
		}
		// 먼저 들어온 요청이 취소돼도 함께 기다리던 요청은 결과를 받아야 한다.
		v, err := load(context.WithoutCancel(ctx))
		if err != nil {
			return nil, err
		}
		c.store(key, v)
		return v, nil
	})
	if err != nil {
		var zero T
		return zero, err
	}
	return v.(T), nil
}

func (c *Cache) lookup(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.items[key]
	if !ok || c.now().After(e.expires) {
		return nil, false
	}
	return e.value, true
}

func (c *Cache) store(key string, v any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	// 키 종류가 많지 않아 만료된 항목을 저장할 때 같이 치운다.
	for k, e := range c.items {
		if now.After(e.expires) {
			delete(c.items, k)
		}
	}
	c.items[key] = entry{value: v, expires: now.Add(c.ttl)}
}
