# ops — 서버에서 직접 실행하는 것들

git 으로 배포되지 않고 사람이 한 번 실행해야 하는 작업의 원본이다. 로컬 docker compose 와 CI 도
같은 파일을 실행해서, 운영에서 처음 돌리는 SQL/요청이 미리 검증된 상태가 되게 한다.

| 파일 | 어디서 | 무엇 |
|---|---|---|
| `monitoring-ro-account.sql` | CNPG primary (슈퍼유저) | `monitoring_ro` 롤: 3개 DB SELECT 만, statement_timeout 10s, 읽기 전용 트랜잭션 |
| `es-monitor-api-key.json` | Elasticsearch (`POST /_security/api_key`) | APM 인덱스 읽기 전용 API 키. 응답의 `encoded` 를 Secret 에 넣는다(다시 조회 불가) |

운영 적용 순서와 Secret 생성은 deploy 레포 `bootstrap/` 에 옮겨 둔다.
