// planmate-monitor 서버. 설정을 읽고 의존성을 조립해 HTTP 서버를 띄운다. 조립은 여기서만 한다.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	// distroless 이미지에는 시간대 DB 가 없다. Asia/Seoul 을 읽으려면 바이너리에 넣어야 한다.
	_ "time/tzdata"

	"github.com/cow-planmate/planmate-monitor/server/internal/apm"
	"github.com/cow-planmate/planmate-monitor/server/internal/cache"
	"github.com/cow-planmate/planmate-monitor/server/internal/health"
	"github.com/cow-planmate/planmate-monitor/server/internal/httpapi"
	"github.com/cow-planmate/planmate-monitor/server/internal/platform/cfaccess"
	"github.com/cow-planmate/planmate-monitor/server/internal/platform/config"
	"github.com/cow-planmate/planmate-monitor/server/internal/platform/elastic"
	"github.com/cow-planmate/planmate-monitor/server/internal/platform/postgres"
	"github.com/cow-planmate/planmate-monitor/server/internal/stats"
	"github.com/cow-planmate/planmate-monitor/server/internal/webui"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err := run(); err != nil {
		slog.Error("monitor stopped", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 풀은 연결을 미리 맺지 않는다. DB 가 죽어 있어도 서버는 뜨고, 화면이 그 사실을 보여준다.
	planmateDB, err := postgres.Open(ctx, cfg.PlanmateDBURL)
	if err != nil {
		return err
	}
	defer planmateDB.Close()
	communityDB, err := postgres.Open(ctx, cfg.CommunityDBURL)
	if err != nil {
		return err
	}
	defer communityDB.Close()

	es := elastic.New(cfg.ESURL, cfg.ESAPIKey)

	checks := []health.Check{
		{Name: "planmate-db", Ping: planmateDB.Ping},
		{Name: "community-db", Ping: communityDB.Ping},
		{Name: "elasticsearch", Ping: es.Ping},
	}
	if cfg.NotificationDBURL != "" {
		notificationDB, err := postgres.Open(ctx, cfg.NotificationDBURL)
		if err != nil {
			return err
		}
		defer notificationDB.Close()
		checks = append(checks, health.Check{Name: "notification-db", Ping: notificationDB.Ping})
	}
	// monitor 전용 DB 는 아직 연결 상태만 본다(2단계 기능이 붙기 전에 운영 준비가 맞는지 화면에서 확인용).
	if cfg.MonitorDBURL != "" {
		monitorDB, err := postgres.Open(ctx, cfg.MonitorDBURL)
		if err != nil {
			return err
		}
		defer monitorDB.Close()
		checks = append(checks, health.Check{Name: "monitor-db", Ping: monitorDB.Ping})
	}

	dataCache := cache.New(cfg.CacheTTL)
	statsSvc := stats.NewService(
		stats.NewPlanmatePostgres(planmateDB, cfg.Location),
		stats.NewCommunityPostgres(communityDB, cfg.Location),
		dataCache, cfg.Location)
	apmSvc := apm.NewService(apm.NewESStore(es, cfg.APMEnvironment, cfg.Location), dataCache)
	healthChecker := health.NewChecker(cache.New(10*time.Second), checks...)

	var verifier httpapi.TokenVerifier
	if cfg.Access.Verify {
		verifier = cfaccess.New(cfg.Access.TeamDomain, cfg.Access.Audience)
	} else {
		slog.Warn("ACCESS_VERIFY=false — Cloudflare Access 토큰을 검증하지 않는다. 운영에서는 켜야 한다")
	}

	handler := httpapi.New(httpapi.Deps{
		Stats:    statsSvc,
		APM:      apmSvc,
		Health:   healthChecker,
		Verifier: verifier,
		Static:   webui.Handler(),
		Location: cfg.Location,
		Meta: httpapi.Meta{
			Environment: cfg.APMEnvironment,
			KibanaURL:   cfg.KibanaURL,
			TimeZone:    cfg.Location.String(),
		},
	})

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       90 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("monitor listening", "addr", cfg.Addr, "apmEnvironment", cfg.APMEnvironment, "accessVerify", cfg.Access.Verify)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		// 롤링 업데이트·종료 시 진행 중인 요청을 마치고 내려간다.
		slog.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return err
		}
	}
	return nil
}
