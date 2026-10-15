package stats

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/cow-planmate/planmate-monitor/server/internal/cache"
	"github.com/cow-planmate/planmate-monitor/server/internal/timeseries"
)

// PlanmateStore 는 planmate DB(Backend-v2)에서 읽는다.
type PlanmateStore interface {
	Counts(ctx context.Context, todayStart time.Time) (users, plans Count, err error)
	ActiveUserIDs(ctx context.Context, from, to time.Time) ([]string, error)
	AdminUserIDs(ctx context.Context) ([]string, error)
	Series(ctx context.Context, m Metric, r timeseries.Range) (map[time.Time]int64, error)
	PlanSummary(ctx context.Context) (PlanSummary, error)
	RecentFeedback(ctx context.Context, limit int) ([]Feedback, error)
}

// CommunityStore 는 community DB 에서 읽는다.
type CommunityStore interface {
	Counts(ctx context.Context, todayStart time.Time) (feedPosts, communityPosts Count, err error)
	ActiveUserIDs(ctx context.Context, from, to time.Time) ([]string, error)
	Series(ctx context.Context, m Metric, r timeseries.Range) (map[time.Time]int64, error)
}

type Service struct {
	planmate  PlanmateStore
	community CommunityStore
	cache     *cache.Cache
	loc       *time.Location
	now       func() time.Time
}

func NewService(p PlanmateStore, c CommunityStore, ch *cache.Cache, loc *time.Location) *Service {
	return &Service{planmate: p, community: c, cache: ch, loc: loc, now: time.Now}
}

func (s *Service) Overview(ctx context.Context) (Overview, error) {
	return cache.Get(ctx, s.cache, "stats:overview", s.overview)
}

func (s *Service) overview(ctx context.Context) (Overview, error) {
	now := s.now()
	todayStart := timeseries.Day.Truncate(now, s.loc)
	out := Overview{GeneratedAt: now.UTC(), Errors: map[string]string{}}

	var (
		wg                       sync.WaitGroup
		pErr, cErr               error
		pActive, cActive, admins []string
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		users, plans, err := s.planmate.Counts(ctx, todayStart)
		if err == nil {
			out.Users, out.Plans = &users, &plans
			pActive, err = s.planmate.ActiveUserIDs(ctx, todayStart, now)
		}
		if err == nil {
			admins, err = s.planmate.AdminUserIDs(ctx)
		}
		pErr = err
	}()
	go func() {
		defer wg.Done()
		feeds, posts, err := s.community.Counts(ctx, todayStart)
		if err == nil {
			out.FeedPosts, out.CommunityPosts = &feeds, &posts
			cActive, err = s.community.ActiveUserIDs(ctx, todayStart, now)
		}
		cErr = err
	}()
	wg.Wait()

	if pErr != nil {
		out.Errors[SourcePlanmate] = sourceFailed(SourcePlanmate, pErr)
	}
	if cErr != nil {
		out.Errors[SourceCommunity] = sourceFailed(SourceCommunity, cErr)
	}
	// 활동 사용자는 두 DB 를 합쳐야 의미가 있다. 한쪽이라도 실패하면 과소 집계가 되므로 비워 둔다.
	if pErr == nil && cErr == nil {
		n := countActive(pActive, cActive, admins)
		out.ActiveUsersToday = &n
	}
	if len(out.Errors) == 0 {
		out.Errors = nil
	}
	return out, nil
}

// countActive 는 두 DB 의 사용자 ID 를 합쳐 중복을 없애고 관리자를 뺀다.
// 회원 ID 는 Backend-v2 가 발급한 UUID 이고 Community 는 그 값을 그대로 복제해 쓴다.
func countActive(planmate, community, admins []string) int64 {
	set := make(map[string]struct{}, len(planmate)+len(community))
	for _, id := range planmate {
		set[id] = struct{}{}
	}
	for _, id := range community {
		set[id] = struct{}{}
	}
	for _, id := range admins {
		delete(set, id)
	}
	return int64(len(set))
}

func (s *Service) Timeseries(ctx context.Context, m Metric, r timeseries.Range) ([]timeseries.Point, error) {
	// DB 의 date_trunc 가 받는 단위만 허용한다. 5분·15분 간격은 APM 에만 있다.
	switch r.Interval {
	case timeseries.Hour, timeseries.Day, timeseries.Week, timeseries.Month:
	default:
		return nil, fmt.Errorf("%w: 사용자 통계는 hour, day, week, month 간격만 지원한다", ErrInvalidInterval)
	}

	var load func(context.Context, Metric, timeseries.Range) (map[time.Time]int64, error)
	switch m {
	case MetricSignups, MetricPlans, MetricPlanJoins, MetricCollaborationRequests:
		load = s.planmate.Series
	case MetricFeedPosts, MetricCommunityPosts, MetricFeedComments, MetricCommunityComments,
		MetricFeedReactions, MetricCommunityReactions:
		load = s.community.Series
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnknownMetric, m)
	}

	key := fmt.Sprintf("stats:series:%s:%d:%d:%s", m, r.From.Unix(), r.To.Unix(), r.Interval)
	return cache.Get(ctx, s.cache, key, func(ctx context.Context) ([]timeseries.Point, error) {
		counts, err := load(ctx, m, r)
		if err != nil {
			return nil, err
		}
		return timeseries.Fill(r, counts), nil
	})
}

func (s *Service) PlanSummary(ctx context.Context) (PlanSummary, error) {
	return cache.Get(ctx, s.cache, "stats:plan-summary", s.planmate.PlanSummary)
}

// RecentFeedback 은 최근 베타 피드백이다. limit 은 1 ~ MaxFeedbackLimit 으로 맞춘다.
func (s *Service) RecentFeedback(ctx context.Context, limit int) ([]Feedback, error) {
	limit = max(1, min(limit, MaxFeedbackLimit))
	return cache.Get(ctx, s.cache, fmt.Sprintf("stats:feedback:%d", limit), func(ctx context.Context) ([]Feedback, error) {
		return s.planmate.RecentFeedback(ctx, limit)
	})
}

// sourceFailed 는 원인을 로그에 남기고, 화면에는 출처만 알려준다.
// DB 에러 원문에는 호스트나 쿼리 일부가 들어갈 수 있어 응답으로 내보내지 않는다.
func sourceFailed(source string, err error) string {
	slog.Error("stats source failed", "source", source, "err", err)
	return source + " DB 조회 실패"
}
