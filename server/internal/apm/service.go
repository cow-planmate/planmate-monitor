package apm

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/cow-planmate/planmate-monitor/server/internal/cache"
	"github.com/cow-planmate/planmate-monitor/server/internal/timeseries"
)

// SeriesRow 는 버킷 하나(서비스별이면 버킷 × 서비스 하나)의 집계 결과다.
type SeriesRow struct {
	Bucket              time.Time
	Service             string // 전체 집계면 빈 문자열
	Requests, Failures  int64
	P50Ms, P95Ms, P99Ms *float64
}

type JVMRow struct {
	Bucket                         time.Time
	HeapUsedMB, HeapMaxMB, Threads *float64
}

// GCRow 는 버킷 안에서 본 GC 누적 시간의 최댓값이다(컬렉터별).
type GCRow struct {
	Bucket    time.Time
	Collector string
	TimeMs    float64
}

// Store 는 APM 데이터를 집계한다. service 가 빈 문자열이면 모든 서비스가 대상이다.
type Store interface {
	Summarize(ctx context.Context, from, to time.Time, byService bool) ([]ServiceSummary, error)
	Series(ctx context.Context, r timeseries.Range, service string, byService bool) ([]SeriesRow, error)
	Endpoints(ctx context.Context, from, to time.Time, service string, limit int) ([]Endpoint, error)
	ErrorGroups(ctx context.Context, from, to time.Time, service string, limit int) ([]ErrorGroup, error)
	JVM(ctx context.Context, r timeseries.Range, service string) ([]JVMRow, []GCRow, error)
}

type Service struct {
	store Store
	cache *cache.Cache
	now   func() time.Time
}

func NewService(store Store, c *cache.Cache) *Service {
	return &Service{store: store, cache: c, now: time.Now}
}

// Summary 는 최근 window 동안의 전체·서비스별 지표다.
func (s *Service) Summary(ctx context.Context, window time.Duration) (Summary, error) {
	if window < MinWindow || window > MaxWindow {
		return Summary{}, fmt.Errorf("%w: %s ~ %s 사이여야 한다", ErrInvalidWindow, MinWindow, MaxWindow)
	}
	// 끝 시각을 분 단위로 내려 같은 분 안의 요청이 캐시를 함께 쓰게 한다.
	to := s.now().Truncate(time.Minute)
	from := to.Add(-window)
	key := fmt.Sprintf("apm:summary:%d:%d", from.Unix(), to.Unix())

	return cache.Get(ctx, s.cache, key, func(ctx context.Context) (Summary, error) {
		services, err := s.store.Summarize(ctx, from, to, true)
		if err != nil {
			return Summary{}, err
		}
		overall, err := s.store.Summarize(ctx, from, to, false)
		if err != nil {
			return Summary{}, err
		}
		out := Summary{From: from.UTC(), To: to.UTC(), Services: services, Overall: ServiceSummary{Service: "all"}}
		if len(overall) > 0 {
			out.Overall = overall[0]
			out.Overall.Service = "all"
		}
		minutes := window.Minutes()
		out.Overall.finish(minutes)
		for i := range out.Services {
			out.Services[i].finish(minutes)
		}
		return out, nil
	})
}

func (s *ServiceSummary) finish(minutes float64) {
	if s.Requests > 0 {
		s.ErrorRate = float64(s.Failures) / float64(s.Requests)
	}
	s.RPM = float64(s.Requests) / minutes
}

// Series 는 버킷별 요청·실패·응답시간이다. service 를 주면 그 서비스만 본다.
func (s *Service) Series(ctx context.Context, r timeseries.Range, service string) (Series, error) {
	if err := validateSeriesRange(r); err != nil {
		return Series{}, err
	}
	key := fmt.Sprintf("apm:series:%s:%d:%d:%s", service, r.From.Unix(), r.To.Unix(), r.Interval)
	return cache.Get(ctx, s.cache, key, func(ctx context.Context) (Series, error) {
		rows, err := s.store.Series(ctx, r, service, true)
		if err != nil {
			return Series{}, err
		}
		// 백분위수는 서비스별 값을 합칠 수 없어서 전체는 따로 집계한다. 한 서비스만 보면 그게 전체다.
		overallRows := rows
		if service == "" {
			if overallRows, err = s.store.Series(ctx, r, "", false); err != nil {
				return Series{}, err
			}
		}
		return buildSeries(r, rows, overallRows), nil
	})
}

func buildSeries(r timeseries.Range, rows, overallRows []SeriesRow) Series {
	buckets := r.Buckets()
	index := make(map[int64]int, len(buckets))
	for i, b := range buckets {
		index[b.Unix()] = i
	}

	out := Series{Interval: r.Interval, Buckets: utc(buckets), Overall: newMetrics("all", len(buckets))}
	byName := map[string]*SeriesMetrics{}
	for _, row := range rows {
		i, ok := index[row.Bucket.Unix()]
		if !ok {
			continue
		}
		m, ok := byName[row.Service]
		if !ok {
			nm := newMetrics(row.Service, len(buckets))
			m = &nm
			byName[row.Service] = m
		}
		m.set(i, row)
	}
	for _, row := range overallRows {
		if i, ok := index[row.Bucket.Unix()]; ok {
			out.Overall.set(i, row)
		}
	}
	names := make([]string, 0, len(byName))
	for n := range byName {
		names = append(names, n)
	}
	slices.Sort(names)
	for _, n := range names {
		out.Services = append(out.Services, *byName[n])
	}
	return out
}

func newMetrics(service string, n int) SeriesMetrics {
	return SeriesMetrics{
		Service:   service,
		Requests:  make([]int64, n),
		Failures:  make([]int64, n),
		ErrorRate: make([]*float64, n),
		P50Ms:     make([]*float64, n),
		P95Ms:     make([]*float64, n),
		P99Ms:     make([]*float64, n),
	}
}

func (m *SeriesMetrics) set(i int, row SeriesRow) {
	m.Requests[i], m.Failures[i] = row.Requests, row.Failures
	m.P50Ms[i], m.P95Ms[i], m.P99Ms[i] = row.P50Ms, row.P95Ms, row.P99Ms
	if row.Requests > 0 {
		rate := float64(row.Failures) / float64(row.Requests)
		m.ErrorRate[i] = &rate
	}
}

// Endpoints 는 p95 가 느린 순서의 엔드포인트 목록이다.
func (s *Service) Endpoints(ctx context.Context, from, to time.Time, service string, limit int) ([]Endpoint, error) {
	if err := validateSpan(from, to, time.Hour); err != nil {
		return nil, err
	}
	limit = max(1, min(limit, MaxEndpoints))
	from, to = from.Truncate(time.Minute), to.Truncate(time.Minute)
	key := fmt.Sprintf("apm:endpoints:%s:%d:%d:%d", service, from.Unix(), to.Unix(), limit)
	return cache.Get(ctx, s.cache, key, func(ctx context.Context) ([]Endpoint, error) {
		return s.store.Endpoints(ctx, from, to, service, limit)
	})
}

// ErrorGroups 는 발생 횟수가 많은 순서의 에러 그룹이다.
func (s *Service) ErrorGroups(ctx context.Context, from, to time.Time, service string) ([]ErrorGroup, error) {
	if err := validateSpan(from, to, time.Hour); err != nil {
		return nil, err
	}
	from, to = from.Truncate(time.Minute), to.Truncate(time.Minute)
	key := fmt.Sprintf("apm:errors:%s:%d:%d", service, from.Unix(), to.Unix())
	return cache.Get(ctx, s.cache, key, func(ctx context.Context) ([]ErrorGroup, error) {
		return s.store.ErrorGroups(ctx, from, to, service, MaxErrorGroups)
	})
}

// JVM 은 한 서비스의 버킷별 힙·스레드·GC 시간이다.
func (s *Service) JVM(ctx context.Context, r timeseries.Range, service string) (JVMSeries, error) {
	if service == "" {
		return JVMSeries{}, fmt.Errorf("%w: JVM 지표는 service 를 지정해야 한다", ErrInvalidRange)
	}
	if err := validateSeriesRange(r); err != nil {
		return JVMSeries{}, err
	}
	key := fmt.Sprintf("apm:jvm:%s:%d:%d:%s", service, r.From.Unix(), r.To.Unix(), r.Interval)
	return cache.Get(ctx, s.cache, key, func(ctx context.Context) (JVMSeries, error) {
		jvm, gc, err := s.store.JVM(ctx, r, service)
		if err != nil {
			return JVMSeries{}, err
		}
		return buildJVM(r, service, jvm, gc), nil
	})
}

func buildJVM(r timeseries.Range, service string, jvm []JVMRow, gc []GCRow) JVMSeries {
	buckets := r.Buckets()
	n := len(buckets)
	index := make(map[int64]int, n)
	for i, b := range buckets {
		index[b.Unix()] = i
	}
	out := JVMSeries{
		Service: service, Interval: r.Interval, Buckets: utc(buckets),
		HeapUsedMB: make([]*float64, n), HeapMaxMB: make([]*float64, n),
		Threads: make([]*float64, n), GCTimeMs: gcDeltas(index, n, gc),
	}
	for _, row := range jvm {
		if i, ok := index[row.Bucket.Unix()]; ok {
			out.HeapUsedMB[i], out.HeapMaxMB[i], out.Threads[i] = row.HeapUsedMB, row.HeapMaxMB, row.Threads
		}
	}
	return out
}

// gcDeltas 는 컬렉터별 누적 GC 시간에서 버킷별 증가분을 구해 합친다.
// 값이 줄었으면 그 사이에 JVM 이 재시작한 것이다 — 카운터가 0부터 다시 쌓였으므로 현재값이 증가분이다.
// 앞 버킷 값이 없는 첫 버킷은 증가분을 알 수 없어 null 이다.
func gcDeltas(index map[int64]int, n int, rows []GCRow) []*float64 {
	byCollector := map[string]map[int]float64{}
	for _, row := range rows {
		i, ok := index[row.Bucket.Unix()]
		if !ok {
			continue
		}
		if byCollector[row.Collector] == nil {
			byCollector[row.Collector] = map[int]float64{}
		}
		byCollector[row.Collector][i] = row.TimeMs
	}

	out := make([]*float64, n)
	for _, values := range byCollector {
		var prev *float64
		for i := range n {
			v, ok := values[i]
			if !ok {
				continue
			}
			if prev != nil {
				d := v - *prev
				if d < 0 {
					d = v
				}
				if out[i] == nil {
					out[i] = new(float64)
				}
				*out[i] += d
			}
			prev = &v
		}
	}
	return out
}

// validateSpan 은 조회 구간을 검사한다. slack 은 구간 시작을 버킷 경계로 내리면서 늘어난 만큼의 여유다
// (예: "최근 7일, 하루 간격" 은 시작이 KST 자정으로 내려가 7일보다 최대 하루 길어진다).
func validateSpan(from, to time.Time, slack time.Duration) error {
	if !from.Before(to) {
		return fmt.Errorf("%w: from 은 to 보다 앞이어야 한다", ErrInvalidRange)
	}
	if to.Sub(from) > MaxRange+slack {
		return fmt.Errorf("%w: APM 데이터는 최대 %s 구간까지 조회한다", ErrInvalidRange, MaxRange)
	}
	return nil
}

func validateSeriesRange(r timeseries.Range) error {
	slack := time.Hour
	switch r.Interval {
	case timeseries.Minute5, timeseries.Minute15, timeseries.Hour:
	case timeseries.Day:
		slack = 24 * time.Hour
	default:
		return fmt.Errorf("%w: APM 시계열은 5m, 15m, hour, day 간격만 지원한다", ErrInvalidRange)
	}
	return validateSpan(r.From, r.To, slack)
}

func utc(ts []time.Time) []time.Time {
	out := make([]time.Time, len(ts))
	for i, t := range ts {
		out[i] = t.UTC()
	}
	return out
}
