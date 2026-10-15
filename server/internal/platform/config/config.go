// Package config 는 환경변수에서 서버 설정을 읽는다.
//
// 접속 정보는 코드에 두지 않는다. 로컬(.env.local)과 운영(k8s ConfigMap/Secret)은 값만 다르고
// 코드 경로는 같다. 비밀값은 NAME 또는 NAME_FILE(파일 경로) 둘 중 하나로 줄 수 있다.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Addr string

	PlanmateDBURL     string
	CommunityDBURL    string
	NotificationDBURL string // 선택. 비어 있으면 연결하지 않는다.

	ESURL    string
	ESAPIKey string

	// APM 데이터의 service.environment 필터. 운영 에이전트는 "prod" 로 보낸다.
	APMEnvironment string
	KibanaURL      string // 선택. 화면에서 Kibana 상세로 가는 링크에 쓴다.

	Access AccessConfig

	CacheTTL time.Duration
	Location *time.Location // 화면/집계 기준 시간대. 서비스 DB 의 TIMESTAMP 컬럼도 이 시간대의 벽시계다.
}

// AccessConfig 는 Cloudflare Access JWT 검증 설정이다.
type AccessConfig struct {
	Verify     bool
	TeamDomain string // 예: planmate.cloudflareaccess.com
	Audience   string // Access 애플리케이션의 AUD 태그
}

func Load() (Config, error) {
	var errs []error
	req := func(name string) string {
		v, err := secret(name)
		if err != nil {
			errs = append(errs, err)
		} else if v == "" {
			errs = append(errs, fmt.Errorf("%s 가 비어 있다", name))
		}
		return v
	}
	opt := func(name string) string {
		v, err := secret(name)
		if err != nil {
			errs = append(errs, err)
		}
		return v
	}

	cfg := Config{
		Addr:              env("MONITOR_ADDR", ":8080"),
		PlanmateDBURL:     req("PLANMATE_DB_URL"),
		CommunityDBURL:    req("COMMUNITY_DB_URL"),
		NotificationDBURL: opt("NOTIFICATION_DB_URL"),
		ESURL:             strings.TrimRight(req("ES_URL"), "/"),
		ESAPIKey:          req("ES_API_KEY"),
		APMEnvironment:    env("APM_ENVIRONMENT", "prod"),
		KibanaURL:         strings.TrimRight(os.Getenv("KIBANA_URL"), "/"),
		Access: AccessConfig{
			Verify:     envBool("ACCESS_VERIFY", true, &errs),
			TeamDomain: os.Getenv("ACCESS_TEAM_DOMAIN"),
			Audience:   os.Getenv("ACCESS_AUD"),
		},
		CacheTTL: envDuration("CACHE_TTL", 30*time.Second, &errs),
	}

	loc, err := time.LoadLocation(env("DISPLAY_TZ", "Asia/Seoul"))
	if err != nil {
		errs = append(errs, fmt.Errorf("DISPLAY_TZ: %w", err))
	}
	cfg.Location = loc

	// 검증을 켰는데 대상이 비어 있으면 모든 요청이 거부된다. 설정 실수를 기동 시점에 드러낸다.
	if cfg.Access.Verify && (cfg.Access.TeamDomain == "" || cfg.Access.Audience == "") {
		errs = append(errs, errors.New("ACCESS_VERIFY=true 이면 ACCESS_TEAM_DOMAIN 과 ACCESS_AUD 가 필요하다"))
	}

	return cfg, errors.Join(errs...)
}

// secret 은 NAME 을 읽고, 없으면 NAME_FILE 이 가리키는 파일을 읽는다.
func secret(name string) (string, error) {
	if v := os.Getenv(name); v != "" {
		return v, nil
	}
	path := os.Getenv(name + "_FILE")
	if path == "" {
		return "", nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("%s_FILE: %w", name, err)
	}
	return strings.TrimSpace(string(b)), nil
}

func env(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}

func envBool(name string, def bool, errs *[]error) bool {
	v := os.Getenv(name)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		*errs = append(*errs, fmt.Errorf("%s: %w", name, err))
		return def
	}
	return b
}

func envDuration(name string, def time.Duration, errs *[]error) time.Duration {
	v := os.Getenv(name)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		*errs = append(*errs, fmt.Errorf("%s: %w", name, err))
		return def
	}
	return d
}
