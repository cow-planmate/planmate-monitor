#!/bin/sh
# 로컬 ES 초기 설정: kibana_system 비밀번호, monitor 용 읽기 전용 API 키 발급.
# API 키 요청 본문은 운영과 같은 ops/es-monitor-api-key.json 을 쓴다.
set -eu

ES=http://elasticsearch:9200
AUTH="elastic:${ELASTIC_PASSWORD}"
JSON='Content-Type: application/json'

echo "Elasticsearch 기다리는 중..."
until curl -s -u "$AUTH" "$ES/_cluster/health?wait_for_status=yellow&timeout=5s" | grep -q '"status"'; do
  sleep 3
done

curl -sf -u "$AUTH" -H "$JSON" -X POST "$ES/_security/user/kibana_system/_password" \
  -d "{\"password\":\"${KIBANA_PASSWORD}\"}" >/dev/null
echo "kibana_system 비밀번호 설정"

# 재실행하면 같은 이름의 이전 키를 무효화하고 새로 발급한다.
curl -s -u "$AUTH" -H "$JSON" -X DELETE "$ES/_security/api_key" -d '{"name":"planmate-monitor"}' >/dev/null || true
resp=$(curl -sf -u "$AUTH" -H "$JSON" -X POST "$ES/_security/api_key" --data-binary @/ops/es-monitor-api-key.json)
encoded=$(echo "$resp" | sed -n 's/.*"encoded":"\([^"]*\)".*/\1/p')
if [ -z "$encoded" ]; then
  echo "API 키 발급 실패: $resp" >&2
  exit 1
fi
printf '%s' "$encoded" > /out/es_api_key
echo "monitor API 키 발급 → local/.secrets/es_api_key"
