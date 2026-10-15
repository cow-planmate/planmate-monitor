// Package health 는 데이터 소스(DB, ES) 연결 상태를 점검한다.
//
// 이 결과는 k8s 프로브에 쓰지 않는다. ES 가 죽었다고 monitor 파드를 내리거나 재시작하면,
// 장애를 봐야 하는 바로 그 순간에 대시보드가 사라진다. 프로브(/livez, /readyz)는 프로세스만 보고,
// 이 점검은 화면에 "어느 소스가 왜 안 되는지" 보여주는 데만 쓴다.
package health

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/cow-planmate/planmate-monitor/server/internal/cache"
)

type Check struct {
	Name string
	Ping func(context.Context) error
}

type SourceStatus struct {
	Name      string `json:"name"`
	OK        bool   `json:"ok"`
	LatencyMs int64  `json:"latencyMs"`
	Error     string `json:"error,omitempty"`
}

type Report struct {
	CheckedAt time.Time      `json:"checkedAt"`
	Sources   []SourceStatus `json:"sources"`
}

type Checker struct {
	checks  []Check
	cache   *cache.Cache
	timeout time.Duration
}

func NewChecker(c *cache.Cache, checks ...Check) *Checker {
	return &Checker{checks: checks, cache: c, timeout: 3 * time.Second}
}

func (h *Checker) Report(ctx context.Context) (Report, error) {
	return cache.Get(ctx, h.cache, "health", h.run)
}

func (h *Checker) run(ctx context.Context) (Report, error) {
	out := Report{CheckedAt: time.Now().UTC(), Sources: make([]SourceStatus, len(h.checks))}
	var wg sync.WaitGroup
	for i, c := range h.checks {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(ctx, h.timeout)
			defer cancel()
			start := time.Now()
			err := c.Ping(ctx)
			st := SourceStatus{Name: c.Name, OK: err == nil, LatencyMs: time.Since(start).Milliseconds()}
			if err != nil {
				slog.Warn("source unhealthy", "source", c.Name, "err", err)
				st.Error = "연결 실패"
				if ctx.Err() != nil {
					st.Error = "응답 시간 초과"
				}
			}
			out.Sources[i] = st
		}()
	}
	wg.Wait()
	return out, nil
}
