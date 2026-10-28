# planmate-monitor

PlanMate 운영 모니터링 대시보드. 서비스 DB(사용자·콘텐츠 통계)와 Elastic APM(성능·에러)을 한 화면에서 본다.
관리자 전용이며 Cloudflare Access 뒤에 둔다.

```
브라우저 ─ Cloudflare Access ─ Tunnel ─ ingress-nginx ─ planmate-monitor (Go 바이너리 1개, React 포함)
                                                          ├─ planmate-v2-db-r  (monitoring_ro, SELECT 만)
                                                          └─ elasticsearch     (APM 인덱스 읽기 전용 API 키)
```

## 왜 만들었나

- **한 화면에서 보기** — 가입·일정 생성 같은 서비스 통계는 DB에, 응답시간·에러·JVM 지표는 Elastic APM 에 흩어져 있어 매번 따로 봐야 했다. 둘을 한 화면에 모았다.
- **직접 제작** — Grafana 는 쿼리는 자유롭지만 화면 배치·흐름이 격자 틀에 묶인다. 통계 화면을 원하는 모양대로 만들기 위해 직접 만들었다.
- **Go 선택** — 클러스터 여유 메모리가 빠듯하다. Spring 앱 하나가 실측 300~670MB 인 데 비해 Go 바이너리는 15~30MB 이고, arm64 노드로 크로스 컴파일하기 쉽다.
- **운영 데이터 보호** — DB 는 읽기 전용 계정(쿼리 10초·연결 10개 제한), Elasticsearch 는 읽기 전용 API 키로만 접근한다.

## 구조

| 경로 | 내용 |
|---|---|
| `server/` | Go. `cmd/monitor` 진입점(조립만), `internal/` 기능별 패키지 |
| `server/internal/{stats,apm}` | 도메인. `service.go`(로직, store 인터페이스) / `store_*.go`(SQL·ES\|QL) / `model.go` |
| `server/internal/httpapi` | HTTP 계층. 파라미터 검증, JSON 응답, Access 검증 미들웨어 |
| `server/internal/platform` | 인프라 연결(config, postgres, elastic, cfaccess). 도메인을 모른다 |
| `web/src/features/*` | 기능별 `types` → `api` → `queries`(TanStack Query) → `components` |
| `web/src/shared` | 도메인을 모르는 공통 모듈(API 클라이언트, 차트, UI, 기간 필터) |
| `ops/` | 운영 서버에서 사람이 실행하는 SQL/요청 원본. 로컬·CI 도 같은 파일을 실행한다 |
| `local/` | 로컬 개발·계약 테스트 환경(docker compose) |

새 지표 추가 순서: `store`(쿼리) → `service`(+테스트) → `httpapi`(엔드포인트) → `web/features`(types/api/queries/components) → `pages`.

## 지표 정의

- **회원**: 현재 남아 있는 회원, ADMIN 제외. 탈퇴는 하드 삭제라 가입 추이는 "아직 남아 있는 회원" 기준이다.
- **일정·글 현황**: 소프트 삭제(`deleted_at`) 제외. **추이**는 생성 건수(나중에 삭제된 것 포함).
- **활동 사용자**: 일정 생성·수정, 글·댓글 작성자(ADMIN 제외). 둘러보기만 한 사람은 셀 수 없다.
- **시간**: 서비스 DB 의 `TIMESTAMP` 컬럼은 KST 벽시계 값이다. 버킷·"오늘"은 KST 자정 기준이다.
- **APM**: `transaction.type == request`, 헬스체크·actuator 제외. 대상은 에이전트가 붙은 4개 서비스.
  조회 기간은 최대 7일(화면에서 더 긴 기간을 고르면 7일로 줄인다). 백분위수는 서비스별 값을 합칠 수 없어 전체는 따로 집계한다.
- **JVM**: `metrics-apm.internal*` 의 `jvm.memory.heap.*`, `jvm.thread.count`, `jvm.gc.time`(누적값 → 구간별 증가분,
  값이 줄면 재시작으로 본다). 인덱스·필드는 로컬 APM Server 8.15 에서 확인했다 — **운영 인덱스에서 한 번 더 확인할 것**.

## 화면과 API

| 화면 | API |
|---|---|
| 개요 | `/api/stats/overview`, `/api/stats/timeseries`, `/api/apm/summary` |
| 사용자 | `/api/stats/timeseries?metric=`(signups, plans, plan_joins, collaboration_requests, feed_posts, community_posts, feed_comments, community_comments, feed_reactions, community_reactions), `/api/stats/plans/summary`, `/api/stats/feedback` |
| 성능 | `/api/apm/series`, `/api/apm/endpoints`, `/api/apm/jvm?service=` |
| 에러 | `/api/apm/series`, `/api/apm/errors` |

공통 파라미터: `from`, `to`(RFC3339), `interval`(통계 hour/day/week/month, APM 5m/15m/hour/day), APM 은 `service`.

## 로컬 개발

필요: Go, Node 24, Docker. 이 레포 옆에 `Backend-v2`, `Community` 가 클론돼 있어야 한다(마이그레이션을 읽는다).

```bash
scripts/local-up.sh                 # Postgres(서비스 스키마+monitoring_ro+시드), ES/Kibana/APM Server 8.15
(cd server && go run ./cmd/apmgen)  # APM 가짜 데이터 24시간치 (-live 로 계속 전송, -backfill 로 기간 조절)
scripts/dev-server.sh               # API :8080 (설정: local/.env.local)
(cd web && npm install && npm run dev)  # 화면 :5173 (/api 는 :8080 으로 프록시)
```

테스트 데이터:

- DB 시드(`local/seed/`): 회원 300명(+ADMIN 1), 일정 700(10% 소프트 삭제), 일정 멤버·협업 요청·베타 피드백,
  피드·커뮤니티 글·댓글·반응. 최근일수록 많아지는 모양이다. 재실행해도 중복으로 들어가지 않는다.
- APM(`cmd/apmgen`): 시간대별 트래픽, 헬스체크(걸러져야 함), 2시간 40분 전부터 20분간 backend-v2 장애
  (지연 4배·에러율 15%), 서비스별 에러 7종, JVM 메트릭과 notification 재시작(6시간 전).
  같은 구간을 두 번 보내면 데이터가 두 배가 된다 — 다시 채우려면 해당 구간을 지우고 보낸다.

테스트:

```bash
(cd server && go test ./...)        # 단위
scripts/test-contract.sh            # 계약: 서비스 마이그레이션으로 만든 DB 에서 store SQL 검증
(cd web && npm test)
```

이미지(운영과 같은 arm64): `docker buildx build --platform linux/arm64 -t planmate-monitor:local --load .`

## 설정 (환경변수)

| 이름 | 필수 | 설명 |
|---|---|---|
| `PLANMATE_DB_URL`, `COMMUNITY_DB_URL` | O | `monitoring_ro` 접속 URL. 운영은 `planmate-v2-db-r` + `sslmode=require` |
| `NOTIFICATION_DB_URL` | | 연결 상태 표시용 |
| `MONITOR_DB_URL` | | monitor 전용 DB(`monitor_app`). 2단계(스냅샷·알림)가 쓴다. 지금은 연결 상태 표시만 |
| `ES_URL`, `ES_API_KEY` | O | 비밀값은 `*_FILE`(파일 경로)로도 줄 수 있다 |
| `APM_ENVIRONMENT` | | `service.environment` 필터. 기본 `prod` |
| `KIBANA_URL` | | 서비스 이름 → Kibana APM 링크 |
| `ACCESS_VERIFY` | | 기본 `true`. 켜면 `ACCESS_TEAM_DOMAIN`, `ACCESS_AUD` 필요 |
| `CACHE_TTL`, `DISPLAY_TZ`, `MONITOR_ADDR` | | 기본 `30s`, `Asia/Seoul`, `:8080` |

프로브: `/livez`, `/readyz` 는 프로세스만 본다. DB/ES 상태는 `/api/health`(화면 사이드바)로 본다 —
데이터 소스 장애로 파드가 내려가면 장애를 볼 대시보드가 사라지기 때문이다.

## CI/CD

`.github/workflows/ci.yml`: 단위 테스트 + 계약 테스트(매일 09:00 KST 에도) → main 이면 arm64/amd64 이미지를
`cycle123/planmate-monitor:sha-xxxxxxx` 로 push → deploy 레포 `kustomization.yaml` 태그 커밋 → ArgoCD 동기화.

레포 Secret: `BACKEND_V2_READ_SSH_KEY`(등록됨), `DOCKERHUB_USERNAME`, `DOCKERHUB_TOKEN`, `DEPLOY_REPO_SSH_KEY`.
