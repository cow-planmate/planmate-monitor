package stats

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cow-planmate/planmate-monitor/server/internal/platform/postgres"
	"github.com/cow-planmate/planmate-monitor/server/internal/timeseries"
)

// 이 파일의 SQL 은 다른 서비스(Backend-v2, Community)의 스키마에 대한 계약이다.
// 그쪽 마이그레이션이 바뀌면 여기가 깨진다 — CI 의 계약 테스트(store_integration_test.go)가
// 각 서비스의 최신 Flyway 마이그레이션으로 만든 DB 에서 이 쿼리들을 실행해 확인한다.
//
// 시간 규칙: 아래 테이블의 created_at/updated_at 은 모두 TIMESTAMP(시간대 없음)이고 KST 벽시계
// 값이다(앱 JVM 이 -Duser.timezone=Asia/Seoul). 그래서 비교 인자는 postgres.Wall 로 바꿔 넘기고,
// 읽은 값은 postgres.FromWall 로 해석한다.
//
// 소프트 삭제: users 는 탈퇴 시 하드 삭제지만 하드 삭제 전환 이전 행이 남아 있을 수 있어
// deleted_at 조건을 유지한다. plan, feed_post, community_post, *_comment 는 실제로 소프트 삭제된다.

const (
	sqlPlanmateCounts = `
SELECT
  (SELECT count(*) FROM users WHERE deleted_at IS NULL AND role <> 'ADMIN'),
  (SELECT count(*) FROM users WHERE deleted_at IS NULL AND role <> 'ADMIN' AND created_at >= $1),
  (SELECT count(*) FROM plan  WHERE deleted_at IS NULL),
  (SELECT count(*) FROM plan  WHERE deleted_at IS NULL AND created_at >= $1)`

	// 일정 편집은 생성만큼 중요한 활동이라 updated_at 도 본다.
	sqlPlanmateActive = `
SELECT DISTINCT user_id::text FROM plan
WHERE (created_at >= $1 AND created_at < $2) OR (updated_at >= $1 AND updated_at < $2)`

	sqlPlanmateAdmins = `SELECT user_id::text FROM users WHERE role = 'ADMIN'`

	sqlPlanSummary = `
SELECT count(*),
       count(*) FILTER (WHERE m.members > 1),
       coalesce(avg(m.members), 0)
FROM plan p
CROSS JOIN LATERAL (SELECT count(*) AS members FROM plan_member pm WHERE pm.plan_id = p.plan_id) m
WHERE p.deleted_at IS NULL`

	sqlCollaborationByStatus = `SELECT status, count(*) FROM collaboration_request GROUP BY status`

	sqlRecentFeedback = `SELECT feedback_id, content, created_at FROM beta_feedback ORDER BY created_at DESC LIMIT $1`

	sqlCommunityCounts = `
SELECT
  (SELECT count(*) FROM feed_post      WHERE deleted_at IS NULL),
  (SELECT count(*) FROM feed_post      WHERE deleted_at IS NULL AND created_at >= $1),
  (SELECT count(*) FROM community_post WHERE deleted_at IS NULL),
  (SELECT count(*) FROM community_post WHERE deleted_at IS NULL AND created_at >= $1)`

	sqlCommunityActive = `
SELECT user_id::text FROM feed_post         WHERE created_at >= $1 AND created_at < $2
UNION
SELECT user_id::text FROM community_post    WHERE created_at >= $1 AND created_at < $2
UNION
SELECT user_id::text FROM feed_comment      WHERE created_at >= $1 AND created_at < $2
UNION
SELECT user_id::text FROM community_comment WHERE created_at >= $1 AND created_at < $2`
)

// seriesSQL 은 ($1 from, $2 to, $3 date_trunc 단위) 를 받아 (bucket, count) 를 돌려주는 SQL 을 만든다.
// 인자는 모두 이 파일의 상수라 사용자 입력이 섞이지 않는다.
func seriesSQL(table, timeCol, cond string) string {
	if cond != "" {
		cond = " AND " + cond
	}
	return fmt.Sprintf(`
SELECT date_trunc($3, %[2]s) AS bucket, count(*)
FROM %[1]s
WHERE %[2]s >= $1 AND %[2]s < $2%[3]s
GROUP BY bucket`, table, timeCol, cond)
}

// 추이는 "생긴 건수" 다. 나중에 소프트 삭제된 일정·글·댓글도 그때 만들어진 것이므로 센다.
// 반응은 취소하면 하드 삭제라 지금 남아 있는 반응만 보인다.
var (
	planmateSeries = map[Metric]string{
		MetricSignups:               seriesSQL("users", "created_at", "deleted_at IS NULL AND role <> 'ADMIN'"),
		MetricPlans:                 seriesSQL("plan", "created_at", ""),
		MetricPlanJoins:             seriesSQL("plan_member", "joined_at", "role = 'EDITOR'"),
		MetricCollaborationRequests: seriesSQL("collaboration_request", "created_at", ""),
	}
	communitySeries = map[Metric]string{
		MetricFeedPosts:          seriesSQL("feed_post", "created_at", ""),
		MetricCommunityPosts:     seriesSQL("community_post", "created_at", ""),
		MetricFeedComments:       seriesSQL("feed_comment", "created_at", ""),
		MetricCommunityComments:  seriesSQL("community_comment", "created_at", ""),
		MetricFeedReactions:      seriesSQL("feed_reaction", "created_at", ""),
		MetricCommunityReactions: seriesSQL("community_reaction", "created_at", ""),
	}
)

type PlanmatePostgres struct {
	pool *pgxpool.Pool
	loc  *time.Location
}

func NewPlanmatePostgres(pool *pgxpool.Pool, loc *time.Location) *PlanmatePostgres {
	return &PlanmatePostgres{pool: pool, loc: loc}
}

func (s *PlanmatePostgres) Counts(ctx context.Context, todayStart time.Time) (users, plans Count, err error) {
	err = postgres.WithRetry(ctx, func(ctx context.Context) error {
		return s.pool.QueryRow(ctx, sqlPlanmateCounts, postgres.Wall(todayStart, s.loc)).
			Scan(&users.Total, &users.Today, &plans.Total, &plans.Today)
	})
	return users, plans, err
}

func (s *PlanmatePostgres) ActiveUserIDs(ctx context.Context, from, to time.Time) ([]string, error) {
	return queryStrings(ctx, s.pool, sqlPlanmateActive, postgres.Wall(from, s.loc), postgres.Wall(to, s.loc))
}

func (s *PlanmatePostgres) AdminUserIDs(ctx context.Context) ([]string, error) {
	return queryStrings(ctx, s.pool, sqlPlanmateAdmins)
}

func (s *PlanmatePostgres) Series(ctx context.Context, m Metric, r timeseries.Range) (map[time.Time]int64, error) {
	sql, ok := planmateSeries[m]
	if !ok {
		return nil, fmt.Errorf("%w: planmate 에 %q 없음", ErrUnknownMetric, m)
	}
	return querySeries(ctx, s.pool, s.loc, sql, r)
}

func (s *PlanmatePostgres) PlanSummary(ctx context.Context) (PlanSummary, error) {
	out := PlanSummary{CollaborationByStatus: map[string]int64{}}
	err := postgres.WithRetry(ctx, func(ctx context.Context) error {
		if err := s.pool.QueryRow(ctx, sqlPlanSummary).Scan(&out.Plans, &out.SharedPlans, &out.AvgMembers); err != nil {
			return err
		}
		rows, err := s.pool.Query(ctx, sqlCollaborationByStatus)
		if err != nil {
			return err
		}
		var (
			status string
			n      int64
		)
		_, err = pgx.ForEachRow(rows, []any{&status, &n}, func() error {
			out.CollaborationByStatus[status] = n
			return nil
		})
		return err
	})
	return out, err
}

func (s *PlanmatePostgres) RecentFeedback(ctx context.Context, limit int) ([]Feedback, error) {
	var out []Feedback
	err := postgres.WithRetry(ctx, func(ctx context.Context) error {
		out = out[:0]
		rows, err := s.pool.Query(ctx, sqlRecentFeedback, limit)
		if err != nil {
			return err
		}
		var f Feedback
		_, err = pgx.ForEachRow(rows, []any{&f.ID, &f.Content, &f.CreatedAt}, func() error {
			f.CreatedAt = postgres.FromWall(f.CreatedAt, s.loc).UTC()
			out = append(out, f)
			return nil
		})
		return err
	})
	return out, err
}

type CommunityPostgres struct {
	pool *pgxpool.Pool
	loc  *time.Location
}

func NewCommunityPostgres(pool *pgxpool.Pool, loc *time.Location) *CommunityPostgres {
	return &CommunityPostgres{pool: pool, loc: loc}
}

func (s *CommunityPostgres) Counts(ctx context.Context, todayStart time.Time) (feedPosts, communityPosts Count, err error) {
	err = postgres.WithRetry(ctx, func(ctx context.Context) error {
		return s.pool.QueryRow(ctx, sqlCommunityCounts, postgres.Wall(todayStart, s.loc)).
			Scan(&feedPosts.Total, &feedPosts.Today, &communityPosts.Total, &communityPosts.Today)
	})
	return feedPosts, communityPosts, err
}

func (s *CommunityPostgres) ActiveUserIDs(ctx context.Context, from, to time.Time) ([]string, error) {
	return queryStrings(ctx, s.pool, sqlCommunityActive, postgres.Wall(from, s.loc), postgres.Wall(to, s.loc))
}

func (s *CommunityPostgres) Series(ctx context.Context, m Metric, r timeseries.Range) (map[time.Time]int64, error) {
	sql, ok := communitySeries[m]
	if !ok {
		return nil, fmt.Errorf("%w: community 에 %q 없음", ErrUnknownMetric, m)
	}
	return querySeries(ctx, s.pool, s.loc, sql, r)
}

func queryStrings(ctx context.Context, pool *pgxpool.Pool, sql string, args ...any) ([]string, error) {
	var out []string
	err := postgres.WithRetry(ctx, func(ctx context.Context) error {
		rows, err := pool.Query(ctx, sql, args...)
		if err != nil {
			return err
		}
		out, err = pgx.CollectRows(rows, pgx.RowTo[string])
		return err
	})
	return out, err
}

// querySeries 는 seriesSQL 로 만든 쿼리를 실행한다.
// date_trunc 는 KST 벽시계 값을 자르므로 결과 버킷도 KST 벽시계이고, 그대로 KST 시각으로 해석하면 된다.
func querySeries(ctx context.Context, pool *pgxpool.Pool, loc *time.Location, sql string, r timeseries.Range) (map[time.Time]int64, error) {
	out := map[time.Time]int64{}
	err := postgres.WithRetry(ctx, func(ctx context.Context) error {
		clear(out)
		rows, err := pool.Query(ctx, sql, postgres.Wall(r.From, loc), postgres.Wall(r.To, loc), string(r.Interval))
		if err != nil {
			return err
		}
		var (
			bucket time.Time
			n      int64
		)
		_, err = pgx.ForEachRow(rows, []any{&bucket, &n}, func() error {
			out[postgres.FromWall(bucket, loc)] = n
			return nil
		})
		return err
	})
	return out, err
}
