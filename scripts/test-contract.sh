#!/usr/bin/env bash
# 계약 테스트: 서비스 마이그레이션으로 만든 DB 에서 store SQL 을 monitoring_ro 로 실행한다.
# 로컬은 scripts/local-up.sh 로 띄운 DB 를 쓰고, CI 는 같은 변수를 워크플로에서 넘긴다.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
pg="${PG_HOSTPORT:-localhost:15432}"
export MONITOR_TEST_PLANMATE_ADMIN_URL="${MONITOR_TEST_PLANMATE_ADMIN_URL:-postgres://planmate:local@$pg/planmate?sslmode=disable}"
export MONITOR_TEST_PLANMATE_RO_URL="${MONITOR_TEST_PLANMATE_RO_URL:-postgres://monitoring_ro:local@$pg/planmate?sslmode=disable}"
export MONITOR_TEST_COMMUNITY_ADMIN_URL="${MONITOR_TEST_COMMUNITY_ADMIN_URL:-postgres://community_app:local@$pg/community?sslmode=disable}"
export MONITOR_TEST_COMMUNITY_RO_URL="${MONITOR_TEST_COMMUNITY_RO_URL:-postgres://monitoring_ro:local@$pg/community?sslmode=disable}"
export MONITOR_TEST_MONITOR_DB_URL="${MONITOR_TEST_MONITOR_DB_URL:-postgres://monitor_app:local@$pg/monitor?sslmode=disable}"
export MONITOR_TEST_MONITOR_TO_PLANMATE_URL="${MONITOR_TEST_MONITOR_TO_PLANMATE_URL:-postgres://monitor_app:local@$pg/planmate?sslmode=disable}"
cd "$root/server"
go test -tags integration -count=1 -v -run Contract ./...
