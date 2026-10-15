// Package httpapi 는 HTTP 요청·응답만 다룬다. 파라미터를 검증해 서비스에 넘기고 결과를 JSON 으로 쓴다.
// SQL 이나 ES 쿼리는 모른다.
package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/cow-planmate/planmate-monitor/server/internal/apm"
	"github.com/cow-planmate/planmate-monitor/server/internal/health"
	"github.com/cow-planmate/planmate-monitor/server/internal/stats"
	"github.com/cow-planmate/planmate-monitor/server/internal/timeseries"
)

type StatsService interface {
	Overview(ctx context.Context) (stats.Overview, error)
	Timeseries(ctx context.Context, m stats.Metric, r timeseries.Range) ([]timeseries.Point, error)
	PlanSummary(ctx context.Context) (stats.PlanSummary, error)
	RecentFeedback(ctx context.Context, limit int) ([]stats.Feedback, error)
}

type APMService interface {
	Summary(ctx context.Context, window time.Duration) (apm.Summary, error)
	Series(ctx context.Context, r timeseries.Range, service string) (apm.Series, error)
	Endpoints(ctx context.Context, from, to time.Time, service string, limit int) ([]apm.Endpoint, error)
	ErrorGroups(ctx context.Context, from, to time.Time, service string) ([]apm.ErrorGroup, error)
	JVM(ctx context.Context, r timeseries.Range, service string) (apm.JVMSeries, error)
}

type HealthChecker interface {
	Report(ctx context.Context) (health.Report, error)
}

// Meta 는 화면이 알아야 하는 서버 설정이다(비밀값 없음).
type Meta struct {
	Environment string `json:"environment"`
	KibanaURL   string `json:"kibanaUrl,omitempty"`
	TimeZone    string `json:"timeZone"`
}

type Deps struct {
	Stats    StatsService
	APM      APMService
	Health   HealthChecker
	Verifier TokenVerifier // nil 이면 Access 검증을 하지 않는다(로컬 개발)
	Static   http.Handler
	Meta     Meta
	Location *time.Location
}

const apiTimeout = 15 * time.Second

func New(d Deps) http.Handler {
	mux := http.NewServeMux()

	probe := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("ok"))
	}
	mux.HandleFunc("GET /livez", probe)
	mux.HandleFunc("GET /readyz", probe)

	h := &handlers{d: d}
	api := func(pattern string, fn http.HandlerFunc) {
		mux.Handle(pattern, http.TimeoutHandler(fn, apiTimeout, `{"error":"요청 시간 초과"}`))
	}
	api("GET /api/meta", h.meta)
	api("GET /api/health", h.health)
	api("GET /api/stats/overview", h.statsOverview)
	api("GET /api/stats/timeseries", h.statsTimeseries)
	api("GET /api/stats/plans/summary", h.statsPlanSummary)
	api("GET /api/stats/feedback", h.statsFeedback)
	api("GET /api/apm/summary", h.apmSummary)
	api("GET /api/apm/series", h.apmSeries)
	api("GET /api/apm/endpoints", h.apmEndpoints)
	api("GET /api/apm/errors", h.apmErrors)
	api("GET /api/apm/jvm", h.apmJVM)

	// 정의되지 않은 /api 경로가 SPA fallback(index.html, 200)으로 가면 프론트가 JSON 파싱 에러로
	// 원인을 가린다. 반드시 JSON 404 를 돌려준다.
	mux.HandleFunc("/api/", func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "존재하지 않는 API")
	})
	mux.Handle("/", d.Static)

	return chain(mux, recoverer, requestLog, accessAuth(d.Verifier))
}
