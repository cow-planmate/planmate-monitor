//go:build integration

// 계약 테스트: ops/monitor-db.sql 로 만든 monitor DB 와 monitor_app 계정의 권한을 확인한다.
//
//	MONITOR_TEST_MONITOR_DB_URL            monitor_app 으로 monitor DB 에 붙는 URL
//	MONITOR_TEST_MONITOR_TO_PLANMATE_URL   monitor_app 으로 planmate DB 에 붙는 URL(거부돼야 한다)
package postgres

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestMonitorDBContract(t *testing.T) {
	ctx := context.Background()

	pool, err := Open(ctx, os.Getenv("MONITOR_TEST_MONITOR_DB_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	// 자기 DB 에서는 테이블을 만들고 쓸 수 있어야 한다(2단계 마이그레이션이 이 권한으로 돈다).
	// 트랜잭션 안에서 하고 되돌려서 흔적을 남기지 않는다.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("monitor DB 접속: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	for _, sql := range []string{
		`CREATE TABLE contract_probe (id int PRIMARY KEY, v text)`,
		`INSERT INTO contract_probe VALUES (1, 'ok')`,
		`UPDATE contract_probe SET v = 'ok2' WHERE id = 1`,
		`CREATE INDEX contract_probe_v ON contract_probe (v)`,
	} {
		if _, err := tx.Exec(ctx, sql); err != nil {
			t.Fatalf("monitor_app 이 자기 DB 에서 %q 실패: %v", sql, err)
		}
	}

	// 서비스 DB 에는 접속조차 못 해야 한다.
	other, err := Open(ctx, os.Getenv("MONITOR_TEST_MONITOR_TO_PLANMATE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	err = other.Ping(ctx)
	if err == nil || !strings.Contains(err.Error(), "permission denied for database") {
		t.Fatalf("monitor_app 이 planmate DB 에 붙었다(또는 다른 이유로 실패): %v", err)
	}
}
