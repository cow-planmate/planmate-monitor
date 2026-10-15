-- 로컬 대시보드 확인용 더미 데이터 (planmate DB, 소유자 planmate 로 실행).
-- 시각은 운영과 같게 KST 벽시계 값을 TIMESTAMP 에 넣는다. 재실행하면 아무것도 하지 않는다.
\set ON_ERROR_STOP on

SELECT EXISTS (SELECT 1 FROM users WHERE nickname LIKE 'seed-user-%') AS seeded \gset
\if :seeded
  \echo 'planmate 시드가 이미 있다 — 건너뛴다'
  \quit
\endif

BEGIN;

-- 최근 90일 회원 300명. 최근일수록 많이 가입한 모양(random()^2)으로 만든다.
INSERT INTO users (user_id, provider, nickname, role, created_at, updated_at)
SELECT gen_random_uuid(), 'LOCAL', 'seed-user-' || g, 'USER', ts, ts
FROM (
  SELECT g, (now() AT TIME ZONE 'Asia/Seoul') - (90 * random() ^ 2) * interval '1 day' AS ts
  FROM generate_series(1, 300) g
) s;

-- 관리자 1명. 대시보드 집계에서 빠져야 한다.
INSERT INTO users (user_id, provider, nickname, role, created_at, updated_at)
VALUES (gen_random_uuid(), 'LOCAL', 'seed-user-admin', 'ADMIN',
        (now() AT TIME ZONE 'Asia/Seoul'), (now() AT TIME ZONE 'Asia/Seoul'));

-- 일정 700개. 만든 사람의 가입 이후 시각에 만들어지고, 10%는 소프트 삭제돼 있다.
INSERT INTO plan (plan_id, plan_name, adult_count, child_count, user_id, destination_id, created_at, updated_at, deleted_at)
SELECT gen_random_uuid(), 'seed-plan-' || g, 2, 0, u.user_id,
       (SELECT min(destination_id) FROM destination),
       c.created, c.created + (random() * ((now() AT TIME ZONE 'Asia/Seoul') - c.created)),
       CASE WHEN random() < 0.1 THEN (now() AT TIME ZONE 'Asia/Seoul') END
FROM generate_series(1, 700) g
CROSS JOIN LATERAL (
  SELECT user_id, created_at FROM users
  WHERE nickname LIKE 'seed-user-%' AND role = 'USER' AND g > 0
  ORDER BY random() LIMIT 1
) u
CROSS JOIN LATERAL (
  SELECT u.created_at + random() * ((now() AT TIME ZONE 'Asia/Seoul') - u.created_at) AS created
) c;

COMMIT;

\echo '== planmate 시드'
SELECT (SELECT count(*) FROM users WHERE nickname LIKE 'seed-user-%') AS users,
       (SELECT count(*) FROM plan  WHERE plan_name LIKE 'seed-plan-%') AS plans;
