-- planmate-monitor 읽기 전용 계정. 운영(CNPG)과 로컬/CI 가 이 파일을 그대로 실행한다.
--
-- 실행: 슈퍼유저(운영은 CNPG primary 파드 안 peer 인증)로
--   psql -v ON_ERROR_STOP=1 -v monitor_password=<비밀번호> -f monitoring-ro-account.sql
-- 비밀번호는 argv 에 남기지 않으려면 \set 한 줄과 이 파일을 stdin 으로 이어 보낸다
-- (deploy/bootstrap/db-bootstrap.sh 와 같은 방식). 재실행해도 안전하다.
--
-- 각 DB 의 테이블 소유자: planmate → planmate, community → community_app, notification → notification_app
-- (deploy/bootstrap/sql/20·30·50 참고). 기본 권한은 "그 소유자가 앞으로 만들 테이블" 에 걸어야
-- Flyway 가 새 테이블을 만들 때마다 SELECT 권한이 자동으로 붙는다.
\set ON_ERROR_STOP on

SELECT format('CREATE ROLE monitoring_ro LOGIN PASSWORD %L', :'monitor_password')
 WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'monitoring_ro')
\gexec
ALTER ROLE monitoring_ro WITH LOGIN PASSWORD :'monitor_password';

-- 안전장치. 잘못된 쿼리가 운영 DB 를 오래 붙잡거나 무언가를 쓰는 일을 DB 쪽에서 막는다.
ALTER ROLE monitoring_ro SET statement_timeout = '10s';
ALTER ROLE monitoring_ro SET default_transaction_read_only = on;
ALTER ROLE monitoring_ro SET idle_in_transaction_session_timeout = '30s';
ALTER ROLE monitoring_ro CONNECTION LIMIT 10;

\connect planmate
GRANT CONNECT ON DATABASE planmate TO monitoring_ro;
GRANT USAGE ON SCHEMA public TO monitoring_ro;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO monitoring_ro;
ALTER DEFAULT PRIVILEGES FOR ROLE planmate IN SCHEMA public GRANT SELECT ON TABLES TO monitoring_ro;

\connect community
GRANT CONNECT ON DATABASE community TO monitoring_ro;
GRANT USAGE ON SCHEMA public TO monitoring_ro;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO monitoring_ro;
ALTER DEFAULT PRIVILEGES FOR ROLE community_app IN SCHEMA public GRANT SELECT ON TABLES TO monitoring_ro;

\connect notification
GRANT CONNECT ON DATABASE notification TO monitoring_ro;
GRANT USAGE ON SCHEMA public TO monitoring_ro;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO monitoring_ro;
ALTER DEFAULT PRIVILEGES FOR ROLE notification_app IN SCHEMA public GRANT SELECT ON TABLES TO monitoring_ro;

\echo '== monitoring_ro 설정'
SELECT rolname, rolconnlimit, rolconfig FROM pg_roles WHERE rolname = 'monitoring_ro';
