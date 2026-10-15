-- 사용자 화면용 추가 더미 데이터 (planmate DB, 소유자 planmate 로 실행). planmate.sql 다음에 돈다.
-- 일정 멤버(소유자 + 공동 편집자), 협업 요청, 베타 피드백. 재실행하면 아무것도 하지 않는다.
\set ON_ERROR_STOP on

SELECT EXISTS (SELECT 1 FROM beta_feedback WHERE content LIKE '[seed]%') AS seeded \gset
\if :seeded
  \echo 'planmate 활동 시드가 이미 있다 — 건너뛴다'
  \quit
\endif

BEGIN;

-- 만든 사람은 OWNER 로 들어간다(서비스 동작과 같다).
INSERT INTO plan_member (plan_id, user_id, role, joined_at)
SELECT plan_id, user_id, 'OWNER', created_at
FROM plan WHERE plan_name LIKE 'seed-plan-%'
ON CONFLICT (user_id, plan_id) DO NOTHING;

-- 일정의 35% 정도에 공동 편집자 1~3명. 합류 시각은 일정 생성 이후다.
INSERT INTO plan_member (plan_id, user_id, role, invited_by, joined_at)
SELECT p.plan_id, u.user_id, 'EDITOR', p.user_id,
       p.created_at + random() * ((now() AT TIME ZONE 'Asia/Seoul') - p.created_at)
FROM plan p
CROSS JOIN LATERAL generate_series(1, 1 + (random() * 2)::int) k
CROSS JOIN LATERAL (
  SELECT user_id FROM users
  WHERE nickname LIKE 'seed-user-%' AND role = 'USER' AND user_id <> p.user_id AND k > 0
  ORDER BY random() LIMIT 1
) u
WHERE p.plan_name LIKE 'seed-plan-%' AND random() < 0.35
ON CONFLICT (user_id, plan_id) DO NOTHING;

-- 협업 요청 400건: 초대(INVITE)와 참여 요청(REQUEST), 대부분 수락.
INSERT INTO collaboration_request (type, status, sender_id, receiver_id, plan_id, created_at, updated_at)
SELECT (ARRAY['INVITE', 'INVITE', 'REQUEST'])[1 + (random() * 2)::int],
       (ARRAY['ACCEPTED', 'ACCEPTED', 'ACCEPTED', 'PENDING', 'REJECTED'])[1 + (random() * 4)::int],
       p.user_id, u.user_id, p.plan_id, c.ts, c.ts
FROM generate_series(1, 400) g
CROSS JOIN LATERAL (SELECT plan_id, user_id, created_at FROM plan WHERE plan_name LIKE 'seed-plan-%' AND g > 0 ORDER BY random() LIMIT 1) p
CROSS JOIN LATERAL (
  SELECT user_id FROM users WHERE nickname LIKE 'seed-user-%' AND role = 'USER' AND user_id <> p.user_id AND g > 0
  ORDER BY random() LIMIT 1
) u
CROSS JOIN LATERAL (SELECT p.created_at + random() * ((now() AT TIME ZONE 'Asia/Seoul') - p.created_at) AS ts) c;

-- 베타 피드백 40건.
INSERT INTO beta_feedback (content, created_at)
SELECT '[seed] ' || (ARRAY[
         '일정 공유 링크를 카카오톡으로 보내면 미리보기가 안 떠요.',
         '여행지 검색에서 영어 지명도 찾을 수 있으면 좋겠어요.',
         '타임테이블에서 블록을 끌어 옮길 때 가끔 위치가 튀어요.',
         '공동 편집 중에 다른 사람이 수정한 내용이 늦게 보여요.',
         '체크리스트 기능 너무 좋아요! 준비물 템플릿도 있으면 좋겠어요.',
         '피드에서 다른 사람 일정을 가져오는 기능이 편해요.',
         '모바일에서 지도 확대가 잘 안 됩니다.',
         '다크 모드 지원해 주세요.'
       ])[1 + (random() * 7)::int],
       (now() AT TIME ZONE 'Asia/Seoul') - (45 * random()) * interval '1 day'
FROM generate_series(1, 40);

COMMIT;

\echo '== planmate 활동 시드'
SELECT (SELECT count(*) FROM plan_member)            AS plan_members,
       (SELECT count(*) FROM collaboration_request)  AS collaboration_requests,
       (SELECT count(*) FROM beta_feedback)          AS feedback;
