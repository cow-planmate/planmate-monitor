-- planmate-monitor 전용 DB(monitor)와 그 소유자 monitor_app. 운영(CNPG)과 로컬/CI 가 이 파일을 그대로 실행한다.
--
-- 2단계 기능(일별 스냅샷, 알림 설정·상태)이 쓰는 DB 다. 테이블은 monitor 가 시작할 때 자기 마이그레이션으로
-- 만들므로, 여기서는 빈 DB 와 계정만 만든다. monitor_app 은 이 DB 만 소유하고 서비스 DB 에는 접속할 수 없다
-- (서비스 DB 는 PUBLIC 의 CONNECT 가 회수돼 있다 — deploy/bootstrap/sql/20·30·40 참고).
--
-- 실행: 슈퍼유저(운영은 CNPG primary 파드 안 peer 인증)로
--   psql -v ON_ERROR_STOP=1 -v monitor_app_password=<비밀번호> -f monitor-db.sql
-- 비밀번호를 argv 에 남기지 않으려면 \set 한 줄과 이 파일을 stdin 으로 이어 보낸다. 재실행해도 안전하다.
\set ON_ERROR_STOP on

SELECT format('CREATE ROLE monitor_app LOGIN PASSWORD %L', :'monitor_app_password')
 WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'monitor_app')
\gexec
ALTER ROLE monitor_app WITH LOGIN PASSWORD :'monitor_app_password';
ALTER ROLE monitor_app CONNECTION LIMIT 10;
ALTER ROLE monitor_app SET statement_timeout = '30s';
ALTER ROLE monitor_app SET idle_in_transaction_session_timeout = '60s';

-- CREATE DATABASE 는 트랜잭션 블록 안에서 못 돌려서 \gexec 로 한다.
SELECT 'CREATE DATABASE monitor OWNER monitor_app'
 WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'monitor')
\gexec
REVOKE ALL ON DATABASE monitor FROM PUBLIC;
GRANT CONNECT, TEMPORARY ON DATABASE monitor TO monitor_app;

\connect monitor
-- PG15+ 의 public 스키마 소유자는 pg_database_owner(=monitor_app)지만, 의도를 분명히 해 둔다.
ALTER SCHEMA public OWNER TO monitor_app;
REVOKE ALL ON SCHEMA public FROM PUBLIC;

\echo '== monitor_app / monitor DB'
SELECT r.rolname, r.rolconnlimit, r.rolconfig, d.datname AS owns_db
  FROM pg_roles r LEFT JOIN pg_database d ON d.datdba = r.oid
 WHERE r.rolname = 'monitor_app';
