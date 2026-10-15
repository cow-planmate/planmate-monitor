// Package postgres 는 서비스 DB 읽기 전용 커넥션 풀을 만든다.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// 운영 CNPG 의 max_connections 는 200 이고 앱 파드들이 대부분을 쓴다.
// 대시보드는 집계 쿼리 몇 개뿐이라 DB 당 3개면 충분하다.
const maxConns = 3

// Open 은 풀을 만들고 연결을 한 번 확인한다. 연결 실패는 에러로 돌려주되,
// 호출자는 이것 때문에 기동을 멈추지 않는다 — DB 가 죽어 있어도 대시보드는 떠 있어야 한다.
func Open(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("DB URL 파싱: %w", err)
	}
	cfg.MaxConns = maxConns
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.ConnConfig.RuntimeParams["application_name"] = "planmate-monitor"

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return pool, nil
}

// WithRetry 는 fn 을 실행하고, 대기 서버(standby)의 복구 충돌로 취소됐을 때만 한 번 더 시도한다.
//
// 복제본에서 도는 쿼리는 primary 의 WAL 반영과 부딪히면
// "canceling statement due to conflict with recovery"(SQLSTATE 40001)로 취소될 수 있다.
// 짧은 집계라 드물지만, 재시도하면 대부분 바로 성공한다.
func WithRetry(ctx context.Context, fn func(context.Context) error) error {
	err := fn(ctx)
	if isRecoveryConflict(err) && ctx.Err() == nil {
		return fn(ctx)
	}
	return err
}

func isRecoveryConflict(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "40001"
}

// Wall 은 t 를 loc 의 벽시계 시각으로 바꾼 뒤, 그 숫자를 그대로 가진 UTC time.Time 을 만든다.
//
// 서비스 DB 의 created_at 등은 TIMESTAMP(시간대 없음)이고, 앱 JVM 이 Asia/Seoul 이라
// KST 벽시계 값이 그대로 들어 있다. 이런 컬럼과 비교할 때는 시각(instant)이 아니라 벽시계 숫자를
// 맞춰야 한다. UTC 로 표현해 두면 드라이버가 시간대를 어떻게 다루든 숫자가 바뀌지 않는다.
func Wall(t time.Time, loc *time.Location) time.Time {
	l := t.In(loc)
	return time.Date(l.Year(), l.Month(), l.Day(), l.Hour(), l.Minute(), l.Second(), l.Nanosecond(), time.UTC)
}

// FromWall 은 Wall 의 역변환이다. TIMESTAMP 컬럼에서 읽은 벽시계 값을 loc 기준 시각으로 해석한다.
func FromWall(w time.Time, loc *time.Location) time.Time {
	return time.Date(w.Year(), w.Month(), w.Day(), w.Hour(), w.Minute(), w.Second(), w.Nanosecond(), loc)
}
