-- 사용자 화면용 반응(좋아요/싫어요) 더미 데이터 (community DB, 소유자 community_app 으로 실행).
-- community.sql 다음에 돈다. 재실행하면 아무것도 하지 않는다.
\set ON_ERROR_STOP on

SELECT EXISTS (SELECT 1 FROM feed_reaction r JOIN feed_post p USING (post_id) WHERE p.title LIKE 'seed-%') AS seeded \gset
\if :seeded
  \echo 'community 반응 시드가 이미 있다 — 건너뛴다'
  \quit
\endif

BEGIN;

-- 반응한 사람은 글쓴이 중에서 고른다(한 사람이 같은 글에 두 번 반응할 수 없다).
CREATE TEMP TABLE seed_reactors AS
SELECT DISTINCT user_id FROM feed_post WHERE title LIKE 'seed-%'
UNION
SELECT DISTINCT user_id FROM community_post WHERE title LIKE 'seed-%';

INSERT INTO feed_reaction (post_id, user_id, type, created_at)
SELECT p.post_id, u.user_id, CASE WHEN random() < 0.9 THEN 'LIKE' ELSE 'DISLIKE' END,
       p.created_at + random() * ((now() AT TIME ZONE 'Asia/Seoul') - p.created_at)
FROM generate_series(1, 1500) g
CROSS JOIN LATERAL (SELECT post_id, created_at FROM feed_post WHERE title LIKE 'seed-%' AND g > 0 ORDER BY random() LIMIT 1) p
CROSS JOIN LATERAL (SELECT user_id FROM seed_reactors WHERE g > 0 ORDER BY random() LIMIT 1) u
ON CONFLICT (post_id, user_id) DO NOTHING;

INSERT INTO community_reaction (post_id, user_id, type, created_at)
SELECT p.post_id, u.user_id, CASE WHEN random() < 0.85 THEN 'LIKE' ELSE 'DISLIKE' END,
       p.created_at + random() * ((now() AT TIME ZONE 'Asia/Seoul') - p.created_at)
FROM generate_series(1, 1500) g
CROSS JOIN LATERAL (SELECT post_id, created_at FROM community_post WHERE title LIKE 'seed-%' AND g > 0 ORDER BY random() LIMIT 1) p
CROSS JOIN LATERAL (SELECT user_id FROM seed_reactors WHERE g > 0 ORDER BY random() LIMIT 1) u
ON CONFLICT (post_id, user_id) DO NOTHING;

COMMIT;

\echo '== community 반응 시드'
SELECT (SELECT count(*) FROM feed_reaction) AS feed_reactions,
       (SELECT count(*) FROM community_reaction) AS community_reactions;
