//go:build integration

// 계약 테스트: 각 서비스의 최신 Flyway 마이그레이션으로 만든 DB 에서 store 의 SQL 을 실행한다.
// Backend-v2/Community 가 스키마를 바꿔 이 쿼리가 깨지면 여기서 드러난다.
//
// 실행: go test -tags integration ./internal/stats/
// 필요한 환경변수(local/ 의 docker compose 와 CI 가 같은 값을 쓴다):
//
//	MONITOR_TEST_PLANMATE_ADMIN_URL   픽스처를 넣을 계정(테이블 소유자)
//	MONITOR_TEST_PLANMATE_RO_URL      monitoring_ro — store 는 운영과 같이 이 계정으로 읽는다
//	MONITOR_TEST_COMMUNITY_ADMIN_URL
//	MONITOR_TEST_COMMUNITY_RO_URL
//
// 다른 데이터가 이미 있어도 되도록 절대값이 아니라 픽스처를 넣기 전후의 차이를 본다.
package stats

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cow-planmate/planmate-monitor/server/internal/platform/postgres"
	"github.com/cow-planmate/planmate-monitor/server/internal/timeseries"
)

var seoul = mustLoc("Asia/Seoul")

func mustLoc(name string) *time.Location {
	l, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return l
}

func openPool(t *testing.T, env string) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv(env)
	if url == "" {
		t.Fatalf("%s 가 비어 있다", env)
	}
	pool, err := postgres.Open(context.Background(), url)
	if err != nil {
		t.Fatalf("%s: %v", env, err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func newUUID(t *testing.T) string {
	t.Helper()
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	h := hex.EncodeToString(b)
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}

// 픽스처 시각. 오늘 KST 자정 직후와 어제 자정 직전 — 시간대 처리가 틀리면 버킷이 뒤바뀐다.
func fixtureTimes() (now, todayStart, justAfterMidnight, justBeforeMidnight time.Time) {
	now = time.Now()
	todayStart = timeseries.Day.Truncate(now, seoul)
	return now, todayStart, todayStart.Add(30 * time.Second), todayStart.Add(-30 * time.Second)
}

func TestPlanmateContract(t *testing.T) {
	ctx := context.Background()
	admin := openPool(t, "MONITOR_TEST_PLANMATE_ADMIN_URL")
	ro := openPool(t, "MONITOR_TEST_PLANMATE_RO_URL")
	store := NewPlanmatePostgres(ro, seoul)

	now, todayStart, today, yesterday := fixtureTimes()
	if now.Before(today) {
		t.Skip("KST 자정 30초 안에는 픽스처가 미래 시각이 돼 건너뛴다")
	}
	series := func() map[time.Time]int64 {
		r, err := timeseries.NewRange(todayStart.AddDate(0, 0, -1), todayStart.AddDate(0, 0, 1), timeseries.Day, seoul)
		if err != nil {
			t.Fatal(err)
		}
		m, err := store.Series(ctx, MetricSignups, r)
		if err != nil {
			t.Fatalf("signups series: %v", err)
		}
		return m
	}

	usersBefore, plansBefore, err := store.Counts(ctx, todayStart)
	if err != nil {
		t.Fatalf("Counts: %v", err)
	}
	seriesBefore := series()

	uToday, uYesterday, uAdmin, uDeleted := newUUID(t), newUUID(t), newUUID(t), newUUID(t)
	ids := []string{uToday, uYesterday, uAdmin, uDeleted}
	t.Cleanup(func() {
		// plan 은 users 의 ON DELETE CASCADE 로 같이 지워진다.
		_, _ = admin.Exec(context.Background(), `DELETE FROM users WHERE user_id = ANY($1::uuid[])`, ids)
	})

	insertUser := func(id, role string, created time.Time, deleted bool) {
		t.Helper()
		var deletedAt any
		if deleted {
			deletedAt = postgres.Wall(created, seoul)
		}
		_, err := admin.Exec(ctx, `
INSERT INTO users (user_id, provider, nickname, role, created_at, updated_at, deleted_at)
VALUES ($1, 'LOCAL', $2, $3, $4, $4, $5)`,
			id, "mon-test-"+id[:8], role, postgres.Wall(created, seoul), deletedAt)
		if err != nil {
			t.Fatalf("insert user: %v", err)
		}
	}
	insertUser(uToday, "USER", today, false)
	insertUser(uYesterday, "USER", yesterday, false)
	insertUser(uAdmin, "ADMIN", today, false)
	insertUser(uDeleted, "USER", today, true)

	insertPlan := func(owner string, created time.Time, deleted bool) {
		t.Helper()
		var deletedAt any
		if deleted {
			deletedAt = postgres.Wall(created, seoul)
		}
		_, err := admin.Exec(ctx, `
INSERT INTO plan (plan_id, plan_name, adult_count, child_count, user_id, destination_id, created_at, updated_at, deleted_at)
VALUES ($1, 'mon-test', 1, 0, $2, (SELECT min(destination_id) FROM destination), $3, $3, $4)`,
			newUUID(t), owner, postgres.Wall(created, seoul), deletedAt)
		if err != nil {
			t.Fatalf("insert plan: %v", err)
		}
	}
	insertPlan(uToday, today, false)
	insertPlan(uToday, today, true) // 소프트 삭제 — 현황에서 빠져야 한다

	users, plans, err := store.Counts(ctx, todayStart)
	if err != nil {
		t.Fatalf("Counts: %v", err)
	}
	if d := users.Total - usersBefore.Total; d != 2 {
		t.Errorf("회원 Total 증가분 = %d, 2 여야 한다(ADMIN·삭제 제외)", d)
	}
	if d := users.Today - usersBefore.Today; d != 1 {
		t.Errorf("회원 Today 증가분 = %d, 1 이어야 한다(어제 23:59:30 KST 가입자는 어제)", d)
	}
	if d := plans.Total - plansBefore.Total; d != 1 {
		t.Errorf("일정 Total 증가분 = %d, 1 이어야 한다(소프트 삭제 제외)", d)
	}
	if d := plans.Today - plansBefore.Today; d != 1 {
		t.Errorf("일정 Today 증가분 = %d, 1 이어야 한다", d)
	}

	active, err := store.ActiveUserIDs(ctx, todayStart, now)
	if err != nil {
		t.Fatalf("ActiveUserIDs: %v", err)
	}
	if !slices.Contains(active, uToday) {
		t.Errorf("오늘 일정을 만든 회원이 활동 사용자에 있어야 한다")
	}
	admins, err := store.AdminUserIDs(ctx)
	if err != nil {
		t.Fatalf("AdminUserIDs: %v", err)
	}
	if !slices.Contains(admins, uAdmin) {
		t.Errorf("ADMIN 목록에 픽스처 관리자가 있어야 한다")
	}

	seriesAfter := series()
	yBucket, tBucket := todayStart.AddDate(0, 0, -1), todayStart
	if d := seriesAfter[yBucket] - seriesBefore[yBucket]; d != 1 {
		t.Errorf("어제 버킷 증가분 = %d, 1 이어야 한다", d)
	}
	if d := seriesAfter[tBucket] - seriesBefore[tBucket]; d != 1 {
		t.Errorf("오늘 버킷 증가분 = %d, 1 이어야 한다", d)
	}

	if _, err := ro.Exec(ctx, `DELETE FROM users WHERE user_id = $1`, uToday); err == nil {
		t.Errorf("monitoring_ro 로 DELETE 가 성공했다 — 읽기 전용이어야 한다")
	}

	// 모든 planmate 지표 쿼리가 현재 스키마에서 실행되는지.
	r, err := timeseries.NewRange(todayStart.AddDate(0, 0, -7), todayStart.AddDate(0, 0, 1), timeseries.Day, seoul)
	if err != nil {
		t.Fatal(err)
	}
	for m := range planmateSeries {
		if _, err := store.Series(ctx, m, r); err != nil {
			t.Errorf("%s series: %v", m, err)
		}
	}
	if _, err := store.PlanSummary(ctx); err != nil {
		t.Errorf("PlanSummary: %v", err)
	}

	// 피드백: KST 벽시계로 저장된 시각을 UTC 시각으로 올바르게 돌려주는지.
	var fbID int64
	if err := admin.QueryRow(ctx, `INSERT INTO beta_feedback (content, created_at) VALUES ('mon-test', $1) RETURNING feedback_id`,
		postgres.Wall(now, seoul)).Scan(&fbID); err != nil {
		t.Fatalf("insert beta_feedback: %v", err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), `DELETE FROM beta_feedback WHERE feedback_id = $1`, fbID)
	})
	fb, err := store.RecentFeedback(ctx, 5)
	if err != nil {
		t.Fatalf("RecentFeedback: %v", err)
	}
	if len(fb) == 0 || fb[0].ID != fbID {
		t.Fatalf("방금 넣은 피드백이 맨 앞이어야 한다: %+v", fb)
	}
	if d := fb[0].CreatedAt.Sub(now); d > time.Second || d < -time.Second {
		t.Errorf("피드백 시각 = %v, 넣은 시각 %v 와 같아야 한다(KST 벽시계 해석 오류)", fb[0].CreatedAt, now.UTC())
	}
}

func TestCommunityContract(t *testing.T) {
	ctx := context.Background()
	admin := openPool(t, "MONITOR_TEST_COMMUNITY_ADMIN_URL")
	ro := openPool(t, "MONITOR_TEST_COMMUNITY_RO_URL")
	store := NewCommunityPostgres(ro, seoul)

	now, todayStart, today, yesterday := fixtureTimes()
	if now.Before(today) {
		t.Skip("KST 자정 30초 안에는 픽스처가 미래 시각이 돼 건너뛴다")
	}

	feedsBefore, postsBefore, err := store.Counts(ctx, todayStart)
	if err != nil {
		t.Fatalf("Counts: %v", err)
	}

	author, commenter := newUUID(t), newUUID(t)
	var feedIDs, postIDs []int64
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = admin.Exec(bg, `DELETE FROM feed_comment WHERE post_id = ANY($1)`, feedIDs)
		_, _ = admin.Exec(bg, `DELETE FROM feed_post WHERE post_id = ANY($1)`, feedIDs)
		_, _ = admin.Exec(bg, `DELETE FROM community_post WHERE post_id = ANY($1)`, postIDs)
	})

	insertFeed := func(created time.Time, deleted bool) int64 {
		t.Helper()
		var deletedAt any
		if deleted {
			deletedAt = postgres.Wall(created, seoul)
		}
		var id int64
		err := admin.QueryRow(ctx, `
INSERT INTO feed_post (user_id, author_nickname, title, content, region, duration_days, created_at, updated_at, deleted_at)
VALUES ($1, 'mon-test', 'mon-test', '{}'::jsonb, '서울', 1, $2, $2, $3)
RETURNING post_id`, author, postgres.Wall(created, seoul), deletedAt).Scan(&id)
		if err != nil {
			t.Fatalf("insert feed_post: %v", err)
		}
		feedIDs = append(feedIDs, id)
		return id
	}
	feed := insertFeed(today, false)
	insertFeed(today, true) // 소프트 삭제

	var postID int64
	if err := admin.QueryRow(ctx, `
INSERT INTO community_post (category, user_id, author_nickname, title, content, created_at, updated_at)
VALUES ('FREE', $1, 'mon-test', 'mon-test', '{}'::jsonb, $2, $2)
RETURNING post_id`, author, postgres.Wall(yesterday, seoul)).Scan(&postID); err != nil {
		t.Fatalf("insert community_post: %v", err)
	}
	postIDs = append(postIDs, postID)

	if _, err := admin.Exec(ctx, `
INSERT INTO feed_comment (post_id, user_id, author_nickname, content, created_at, updated_at)
VALUES ($1, $2, 'mon-test', 'mon-test', $3, $3)`, feed, commenter, postgres.Wall(today, seoul)); err != nil {
		t.Fatalf("insert feed_comment: %v", err)
	}

	feeds, posts, err := store.Counts(ctx, todayStart)
	if err != nil {
		t.Fatalf("Counts: %v", err)
	}
	if d := feeds.Total - feedsBefore.Total; d != 1 {
		t.Errorf("피드 Total 증가분 = %d, 1 이어야 한다(소프트 삭제 제외)", d)
	}
	if d := feeds.Today - feedsBefore.Today; d != 1 {
		t.Errorf("피드 Today 증가분 = %d, 1 이어야 한다", d)
	}
	if d := posts.Total - postsBefore.Total; d != 1 {
		t.Errorf("커뮤니티 글 Total 증가분 = %d, 1 이어야 한다", d)
	}
	if d := posts.Today - postsBefore.Today; d != 0 {
		t.Errorf("커뮤니티 글 Today 증가분 = %d, 0 이어야 한다(어제 글)", d)
	}

	active, err := store.ActiveUserIDs(ctx, todayStart, now)
	if err != nil {
		t.Fatalf("ActiveUserIDs: %v", err)
	}
	if !slices.Contains(active, author) || !slices.Contains(active, commenter) {
		t.Errorf("오늘 글쓴이와 댓글 작성자가 활동 사용자에 있어야 한다: %v", active)
	}

	r, err := timeseries.NewRange(todayStart.AddDate(0, 0, -1), todayStart.AddDate(0, 0, 1), timeseries.Day, seoul)
	if err != nil {
		t.Fatal(err)
	}
	for m := range communitySeries {
		if _, err := store.Series(ctx, m, r); err != nil {
			t.Errorf("%s series: %v", m, err)
		}
	}
}
