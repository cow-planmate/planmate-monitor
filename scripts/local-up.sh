#!/usr/bin/env bash
# 로컬 환경을 띄우고 초기화한다: Postgres(서비스 스키마 + monitoring_ro + 시드), ES/Kibana/APM Server.
# 재실행해도 안전하다. 시드를 빼려면 SEED=0.
set -euo pipefail
cd "$(dirname "$0")/../local"

dc() { docker compose "$@"; }

echo "== 컨테이너 시작"
dc up -d --wait postgres elasticsearch

echo "== Postgres: DB/소유자 롤 → 서비스 마이그레이션 → monitoring_ro"
dc run --rm db-bootstrap
dc run --rm flyway-planmate
dc run --rm flyway-community
dc run --rm db-monitor-role
if [ "${SEED:-1}" = "1" ]; then
  dc run --rm db-seed
fi

echo "== Elasticsearch: kibana_system 비밀번호, monitor API 키"
mkdir -p .secrets
dc run --rm es-setup

echo "== Kibana, APM Server 시작"
dc up -d kibana apm-server

cat <<MSG

완료.
  Postgres   localhost:15432  (postgres/postgres, monitoring_ro/local)
  ES         http://localhost:9200  (elastic/elastic-local)
  Kibana     http://localhost:5601  (elastic/elastic-local, 뜨는 데 1~2분)
  APM Server http://localhost:8200  (secret token: local-token)

APM 데이터 넣기:  (cd server && go run ./cmd/apmgen)
서버 실행:        scripts/dev-server.sh
화면 실행:        (cd web && npm run dev)
MSG
