-- 로컬 대시보드 확인용 더미 데이터 (community DB, 소유자 community_app 으로 실행).
-- 작성자 ID 는 planmate DB 의 회원과 이어지지 않는다(DB 가 달라 조인할 수 없다). 집계 모양 확인용이다.
\set ON_ERROR_STOP on

SELECT EXISTS (SELECT 1 FROM feed_post WHERE title LIKE 'seed-%') AS seeded \gset
\if :seeded
  \echo 'community 시드가 이미 있다 — 건너뛴다'
  \quit
\endif

BEGIN;

CREATE TEMP TABLE seed_authors AS
SELECT gen_random_uuid() AS user_id, 'seed-author-' || g AS nickname FROM generate_series(1, 150) g;

INSERT INTO feed_post (user_id, author_nickname, title, content, region, duration_days, created_at, updated_at, deleted_at)
SELECT a.user_id, a.nickname, 'seed-feed-' || g, '{}'::jsonb, '서울', 1 + (random() * 4)::int, c.ts, c.ts,
       CASE WHEN random() < 0.08 THEN c.ts + interval '1 hour' END
FROM generate_series(1, 300) g
CROSS JOIN LATERAL (SELECT * FROM seed_authors WHERE g > 0 ORDER BY random() LIMIT 1) a
CROSS JOIN LATERAL (SELECT (now() AT TIME ZONE 'Asia/Seoul') - (60 * random() ^ 2) * interval '1 day' + g * interval '0' AS ts) c;

INSERT INTO community_post (category, user_id, author_nickname, title, content, created_at, updated_at, deleted_at)
SELECT 'FREE', a.user_id, a.nickname, 'seed-post-' || g, '{}'::jsonb, c.ts, c.ts,
       CASE WHEN random() < 0.08 THEN c.ts + interval '1 hour' END
FROM generate_series(1, 400) g
CROSS JOIN LATERAL (SELECT * FROM seed_authors WHERE g > 0 ORDER BY random() LIMIT 1) a
CROSS JOIN LATERAL (SELECT (now() AT TIME ZONE 'Asia/Seoul') - (60 * random() ^ 2) * interval '1 day' + g * interval '0' AS ts) c;

INSERT INTO feed_comment (post_id, user_id, author_nickname, content, created_at, updated_at)
SELECT p.post_id, a.user_id, a.nickname, 'seed-comment', ts, ts
FROM generate_series(1, 600) g
CROSS JOIN LATERAL (SELECT post_id, created_at FROM feed_post WHERE g > 0 ORDER BY random() LIMIT 1) p
CROSS JOIN LATERAL (SELECT * FROM seed_authors WHERE g > 0 ORDER BY random() LIMIT 1) a
CROSS JOIN LATERAL (SELECT p.created_at + random() * ((now() AT TIME ZONE 'Asia/Seoul') - p.created_at) AS ts) c;

INSERT INTO community_comment (post_id, user_id, author_nickname, content, created_at, updated_at)
SELECT p.post_id, a.user_id, a.nickname, 'seed-comment', ts, ts
FROM generate_series(1, 600) g
CROSS JOIN LATERAL (SELECT post_id, created_at FROM community_post WHERE g > 0 ORDER BY random() LIMIT 1) p
CROSS JOIN LATERAL (SELECT * FROM seed_authors WHERE g > 0 ORDER BY random() LIMIT 1) a
CROSS JOIN LATERAL (SELECT p.created_at + random() * ((now() AT TIME ZONE 'Asia/Seoul') - p.created_at) AS ts) c;

COMMIT;

\echo '== community 시드'
SELECT (SELECT count(*) FROM feed_post      WHERE title LIKE 'seed-%') AS feed_posts,
       (SELECT count(*) FROM community_post WHERE title LIKE 'seed-%') AS community_posts;
