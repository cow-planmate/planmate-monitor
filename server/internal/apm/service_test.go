package apm

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cow-planmate/planmate-monitor/server/internal/cache"
	"github.com/cow-planmate/planmate-monitor/server/internal/timeseries"
)

var kst = time.FixedZone("KST", 9*3600)

type fakeStore struct {
	services []ServiceSummary
	overall  []ServiceSummary

	seriesBy, seriesAll []SeriesRow
	seriesCalls         int
	jvm                 []JVMRow
	gc                  []GCRow
}

func (f *fakeStore) Summarize(_ context.Context, _, _ time.Time, byService bool) ([]ServiceSummary, error) {
	if byService {
		return append([]ServiceSummary(nil), f.services...), nil
	}
	return append([]ServiceSummary(nil), f.overall...), nil
}

func (f *fakeStore) Series(_ context.Context, _ timeseries.Range, _ string, byService bool) ([]SeriesRow, error) {
	f.seriesCalls++
	if byService {
		return f.seriesBy, nil
	}
	return f.seriesAll, nil
}

func (f *fakeStore) Endpoints(context.Context, time.Time, time.Time, string, int) ([]Endpoint, error) {
	return nil, nil
}

func (f *fakeStore) ErrorGroups(context.Context, time.Time, time.Time, string, int) ([]ErrorGroup, error) {
	return nil, nil
}

func (f *fakeStore) JVM(context.Context, timeseries.Range, string) ([]JVMRow, []GCRow, error) {
	return f.jvm, f.gc, nil
}

func ptr(v float64) *float64 { return &v }

func TestSummaryComputesRatesPerMinute(t *testing.T) {
	p95 := 120.0
	s := NewService(&fakeStore{
		services: []ServiceSummary{{Service: "planmate-backend-v2", Requests: 150, Failures: 3, P95Ms: &p95}},
		overall:  []ServiceSummary{{Requests: 150, Failures: 3, P95Ms: &p95}},
	}, cache.New(time.Minute))

	sum, err := s.Summary(context.Background(), 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Overall.Service != "all" || sum.Overall.RPM != 10 {
		t.Fatalf("overall = %+v, 15분 150건이면 10 rpm", sum.Overall)
	}
	if got := sum.Services[0].ErrorRate; got != 0.02 {
		t.Fatalf("에러율 = %v, 3/150 = 0.02", got)
	}
}

func TestSummaryWithNoTraffic(t *testing.T) {
	s := NewService(&fakeStore{}, cache.New(time.Minute))
	sum, err := s.Summary(context.Background(), 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Overall.Requests != 0 || sum.Overall.ErrorRate != 0 || sum.Overall.P95Ms != nil {
		t.Fatalf("요청이 없으면 0과 null 이어야 한다: %+v", sum.Overall)
	}
}

func TestSummaryRejectsWindowOutOfRange(t *testing.T) {
	s := NewService(&fakeStore{}, cache.New(time.Minute))
	for _, w := range []time.Duration{0, 30 * time.Second, 48 * time.Hour} {
		if _, err := s.Summary(context.Background(), w); !errors.Is(err, ErrInvalidWindow) {
			t.Errorf("window %s 는 거부해야 한다, err=%v", w, err)
		}
	}
}

func dayRange(t *testing.T, days int) timeseries.Range {
	t.Helper()
	from := time.Date(2026, 10, 13, 0, 0, 0, 0, kst)
	r, err := timeseries.NewRange(from, from.AddDate(0, 0, days), timeseries.Day, kst)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestSeriesAlignsRowsToBucketsAndLeavesGapsNull(t *testing.T) {
	r := dayRange(t, 3)
	day2 := time.Date(2026, 10, 14, 0, 0, 0, 0, kst).UTC() // ES 는 UTC 로 돌려준다
	f := &fakeStore{
		seriesBy: []SeriesRow{
			{Bucket: day2, Service: "b", Requests: 10, Failures: 1, P95Ms: ptr(200)},
			{Bucket: day2, Service: "a", Requests: 4},
		},
		seriesAll: []SeriesRow{{Bucket: day2, Requests: 14, Failures: 1, P95Ms: ptr(180)}},
	}
	got, err := NewService(f, cache.New(time.Minute)).Series(context.Background(), r, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Buckets) != 3 || len(got.Services) != 2 || got.Services[0].Service != "a" {
		t.Fatalf("버킷 3개, 서비스 이름순 2개여야 한다: %+v", got)
	}
	b := got.Services[1]
	if b.Requests[1] != 10 || *b.ErrorRate[1] != 0.1 || *b.P95Ms[1] != 200 {
		t.Fatalf("둘째 버킷에 값이 들어가야 한다: %+v", b)
	}
	if b.Requests[0] != 0 || b.ErrorRate[0] != nil || b.P95Ms[0] != nil {
		t.Fatalf("요청 없는 버킷의 에러율·응답시간은 null 이어야 한다: %+v", b)
	}
	if got.Overall.Requests[1] != 14 || *got.Overall.P95Ms[1] != 180 {
		t.Fatalf("전체는 따로 집계한 값이어야 한다(백분위수는 합칠 수 없다): %+v", got.Overall)
	}
}

func TestSeriesForOneServiceQueriesOnce(t *testing.T) {
	f := &fakeStore{}
	if _, err := NewService(f, cache.New(time.Minute)).Series(context.Background(), dayRange(t, 2), "planmate-gateway"); err != nil {
		t.Fatal(err)
	}
	if f.seriesCalls != 1 {
		t.Fatalf("한 서비스만 볼 때는 그 서비스가 곧 전체라 한 번만 조회해야 한다: %d번", f.seriesCalls)
	}
}

func TestSeriesRejectsLongRangeAndWeekInterval(t *testing.T) {
	s := NewService(&fakeStore{}, cache.New(time.Minute))
	if _, err := s.Series(context.Background(), dayRange(t, 10), ""); !errors.Is(err, ErrInvalidRange) {
		t.Errorf("7일을 넘는 구간은 거부해야 한다, err=%v", err)
	}
	from := time.Date(2026, 10, 1, 0, 0, 0, 0, kst)
	week, _ := timeseries.NewRange(from, from.AddDate(0, 0, 7), timeseries.Week, kst)
	if _, err := s.Series(context.Background(), week, ""); !errors.Is(err, ErrInvalidRange) {
		t.Errorf("week 간격은 거부해야 한다, err=%v", err)
	}
}

func TestJVMRequiresService(t *testing.T) {
	s := NewService(&fakeStore{}, cache.New(time.Minute))
	if _, err := s.JVM(context.Background(), dayRange(t, 2), ""); !errors.Is(err, ErrInvalidRange) {
		t.Fatalf("err=%v", err)
	}
}

func TestGCDeltasHandleRestartAndMultipleCollectors(t *testing.T) {
	r := dayRange(t, 4)
	b := r.Buckets()
	gc := []GCRow{
		{Bucket: b[0], Collector: "young", TimeMs: 100},
		{Bucket: b[1], Collector: "young", TimeMs: 160},
		{Bucket: b[2], Collector: "young", TimeMs: 30}, // 재시작: 0부터 다시 30
		{Bucket: b[3], Collector: "young", TimeMs: 50},
		{Bucket: b[1], Collector: "old", TimeMs: 500},
		{Bucket: b[3], Collector: "old", TimeMs: 540}, // b[2] 가 비어도 이전 값과 비교한다
	}
	got := buildJVM(r, "svc", nil, gc).GCTimeMs

	if got[0] != nil {
		t.Errorf("첫 버킷은 앞 값이 없어 null 이어야 한다: %v", *got[0])
	}
	want := map[int]float64{1: 60, 2: 30, 3: 20 + 40}
	for i, w := range want {
		if got[i] == nil || *got[i] != w {
			t.Errorf("버킷 %d = %v, %v 여야 한다", i, got[i], w)
		}
	}
}

func TestSeriesAcceptsLastSevenDaysTruncatedToMidnight(t *testing.T) {
	// "지금부터 7일 전" 을 하루 간격으로 자르면 시작이 KST 자정으로 내려가 7일보다 길어진다. 이건 허용해야 한다.
	now := time.Date(2026, 10, 15, 21, 30, 0, 0, kst)
	r, err := timeseries.NewRange(now.AddDate(0, 0, -7), now, timeseries.Day, kst)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewService(&fakeStore{}, cache.New(time.Minute)).Series(context.Background(), r, ""); err != nil {
		t.Fatalf("최근 7일(일 단위)은 허용해야 한다: %v", err)
	}
}
