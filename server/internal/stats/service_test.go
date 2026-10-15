package stats

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cow-planmate/planmate-monitor/server/internal/cache"
	"github.com/cow-planmate/planmate-monitor/server/internal/timeseries"
)

var kst = time.FixedZone("KST", 9*60*60)

type fakePlanmate struct {
	users, plans  Count
	active        []string
	admins        []string
	err           error
	series        map[time.Time]int64
	todayStart    time.Time
	feedbackLimit int
}

func (f *fakePlanmate) Counts(_ context.Context, todayStart time.Time) (Count, Count, error) {
	f.todayStart = todayStart
	return f.users, f.plans, f.err
}
func (f *fakePlanmate) ActiveUserIDs(context.Context, time.Time, time.Time) ([]string, error) {
	return f.active, f.err
}
func (f *fakePlanmate) AdminUserIDs(context.Context) ([]string, error) { return f.admins, f.err }
func (f *fakePlanmate) Series(context.Context, Metric, timeseries.Range) (map[time.Time]int64, error) {
	return f.series, f.err
}
func (f *fakePlanmate) PlanSummary(context.Context) (PlanSummary, error) { return PlanSummary{}, f.err }
func (f *fakePlanmate) RecentFeedback(_ context.Context, limit int) ([]Feedback, error) {
	f.feedbackLimit = limit
	return nil, f.err
}

type fakeCommunity struct {
	feeds, posts Count
	active       []string
	err          error
}

func (f *fakeCommunity) Counts(context.Context, time.Time) (Count, Count, error) {
	return f.feeds, f.posts, f.err
}
func (f *fakeCommunity) ActiveUserIDs(context.Context, time.Time, time.Time) ([]string, error) {
	return f.active, f.err
}
func (f *fakeCommunity) Series(context.Context, Metric, timeseries.Range) (map[time.Time]int64, error) {
	return nil, f.err
}

func newTestService(p PlanmateStore, c CommunityStore, now time.Time) *Service {
	s := NewService(p, c, cache.New(time.Minute), kst)
	s.now = func() time.Time { return now }
	return s
}

func TestOverviewMergesActiveUsersAndExcludesAdmins(t *testing.T) {
	p := &fakePlanmate{
		users:  Count{Total: 10, Today: 2},
		active: []string{"a", "b", "admin"},
		admins: []string{"admin"},
	}
	c := &fakeCommunity{active: []string{"b", "c"}}
	ov, err := newTestService(p, c, time.Now()).Overview(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if ov.ActiveUsersToday == nil || *ov.ActiveUsersToday != 3 {
		t.Fatalf("a, b, c 3명이어야 한다(admin 제외, b 중복 제거), got %v", ov.ActiveUsersToday)
	}
	if ov.Errors != nil {
		t.Fatalf("에러가 없어야 한다: %v", ov.Errors)
	}
}

func TestOverviewPartialFailureKeepsOtherSource(t *testing.T) {
	p := &fakePlanmate{users: Count{Total: 10}}
	c := &fakeCommunity{err: errors.New("connection refused")}
	ov, err := newTestService(p, c, time.Now()).Overview(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if ov.Users == nil || ov.Users.Total != 10 {
		t.Fatalf("planmate 지표는 살아 있어야 한다: %+v", ov.Users)
	}
	if ov.FeedPosts != nil || ov.CommunityPosts != nil {
		t.Fatal("실패한 출처의 지표는 null 이어야 한다")
	}
	if ov.ActiveUsersToday != nil {
		t.Fatal("한쪽 DB 가 실패하면 활동 사용자는 과소 집계되므로 비워야 한다")
	}
	if _, ok := ov.Errors[SourceCommunity]; !ok {
		t.Fatalf("community 에러가 있어야 한다: %v", ov.Errors)
	}
}

func TestOverviewTodayStartsAtKSTMidnight(t *testing.T) {
	p := &fakePlanmate{}
	// UTC 10/14 16:30 = KST 10/15 01:30
	now := time.Date(2026, 10, 14, 16, 30, 0, 0, time.UTC)
	if _, err := newTestService(p, &fakeCommunity{}, now).Overview(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 10, 15, 0, 0, 0, 0, kst)
	if !p.todayStart.Equal(want) {
		t.Fatalf("오늘의 시작은 KST 자정이어야 한다: got %v, want %v", p.todayStart, want)
	}
}

func TestTimeseriesRejectsUnknownMetric(t *testing.T) {
	s := newTestService(&fakePlanmate{}, &fakeCommunity{}, time.Now())
	r, _ := timeseries.NewRange(time.Now().Add(-48*time.Hour), time.Now(), timeseries.Day, kst)
	if _, err := s.Timeseries(context.Background(), "nope", r); !errors.Is(err, ErrUnknownMetric) {
		t.Fatalf("err=%v", err)
	}
}

func TestTimeseriesFillsGaps(t *testing.T) {
	from := time.Date(2026, 10, 13, 0, 0, 0, 0, kst)
	p := &fakePlanmate{series: map[time.Time]int64{from.AddDate(0, 0, 1): 4}}
	s := newTestService(p, &fakeCommunity{}, time.Now())
	r, _ := timeseries.NewRange(from, from.AddDate(0, 0, 3), timeseries.Day, kst)
	pts, err := s.Timeseries(context.Background(), MetricSignups, r)
	if err != nil {
		t.Fatal(err)
	}
	if len(pts) != 3 || pts[1].Value != 4 || pts[0].Value != 0 {
		t.Fatalf("got %+v", pts)
	}
}

func TestTimeseriesRejectsMinuteIntervals(t *testing.T) {
	s := newTestService(&fakePlanmate{}, &fakeCommunity{}, time.Now())
	r, _ := timeseries.NewRange(time.Now().Add(-time.Hour), time.Now(), timeseries.Minute5, kst)
	if _, err := s.Timeseries(context.Background(), MetricSignups, r); !errors.Is(err, ErrInvalidInterval) {
		t.Fatalf("5분 간격은 DB date_trunc 가 지원하지 않으므로 거부해야 한다, err=%v", err)
	}
}

func TestRecentFeedbackClampsLimit(t *testing.T) {
	p := &fakePlanmate{}
	s := newTestService(p, &fakeCommunity{}, time.Now())
	if _, err := s.RecentFeedback(context.Background(), 10_000); err != nil {
		t.Fatal(err)
	}
	if p.feedbackLimit != MaxFeedbackLimit {
		t.Fatalf("limit = %d, %d 로 잘려야 한다", p.feedbackLimit, MaxFeedbackLimit)
	}
}
