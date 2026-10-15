-- 로컬/CI 용 DB 와 소유자 롤. 운영 CNPG 의 구성을 흉내 낸다(deploy/bootstrap/sql 참고).
--   planmate     DB 소유자 planmate        (CNPG initdb owner)
--   community    테이블 소유자 community_app (DB 는 슈퍼유저 소유, public 스키마에 CREATE 권한)
--   notification 테이블 소유자 notification_app
-- 비밀번호는 전부 'local' 이다. 로컬 전용이다.
\set ON_ERROR_STOP on

SELECT 'CREATE ROLE planmate LOGIN PASSWORD ''local'''
 WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'planmate') \gexec
SELECT 'CREATE ROLE community_app LOGIN PASSWORD ''local'''
 WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'community_app') \gexec
SELECT 'CREATE ROLE notification_app LOGIN PASSWORD ''local'''
 WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'notification_app') \gexec

SELECT 'CREATE DATABASE planmate OWNER planmate'
 WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'planmate') \gexec
SELECT 'CREATE DATABASE community'
 WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'community') \gexec
SELECT 'CREATE DATABASE notification'
 WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'notification') \gexec

-- 운영과 같게 서비스 DB 의 PUBLIC 접속을 회수한다(deploy/bootstrap/sql/20·30 참고). 그래야 새 계정은 명시적으로
-- GRANT 받은 DB 에만 붙는다 — monitor_app 이 서비스 DB 에 못 붙는지를 로컬에서도 검증할 수 있다.
REVOKE CONNECT ON DATABASE planmate FROM PUBLIC;
REVOKE CONNECT ON DATABASE community FROM PUBLIC;
REVOKE CONNECT ON DATABASE notification FROM PUBLIC;

\connect planmate
ALTER SCHEMA public OWNER TO planmate;

\connect community
GRANT CONNECT ON DATABASE community TO community_app;
GRANT USAGE, CREATE ON SCHEMA public TO community_app;
-- Community V1 이 CREATE EXTENSION pg_trgm 을 부르는데 앱 롤은 확장을 못 만든다(운영과 같은 사정).
CREATE EXTENSION IF NOT EXISTS pg_trgm;

\connect notification
GRANT CONNECT ON DATABASE notification TO notification_app;
GRANT USAGE, CREATE ON SCHEMA public TO notification_app;
