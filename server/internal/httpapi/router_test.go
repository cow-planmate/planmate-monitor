package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cow-planmate/planmate-monitor/server/internal/apm"
	"github.com/cow-planmate/planmate-monitor/server/internal/health"
	"github.com/cow-planmate/planmate-monitor/server/internal/platform/cfaccess"
	"github.com/cow-planmate/planmate-monitor/server/internal/stats"
	"github.com/cow-planmate/planmate-monitor/server/internal/timeseries"
)

type fakeStats struct{ gotRange timeseries.Range }

func (f *fakeStats) Overview(context.Context) (stats.Overview, error) {
	return stats.Overview{Users: &stats.Count{Total: 3}}, nil
}
func (f *fakeStats) Timeseries(_ context.Context, m stats.Metric, r timeseries.Range) ([]timeseries.Point, error) {
	f.gotRange = r
	if m != stats.MetricSignups {
		return nil, stats.ErrUnknownMetric
	}
	return []timeseries.Point{}, nil
}
func (f *fakeStats) PlanSummary(context.Context) (stats.PlanSummary, error) {
	return stats.PlanSummary{}, nil
}
func (f *fakeStats) RecentFeedback(context.Context, int) ([]stats.Feedback, error) { return nil, nil }

type fakeAPM struct{}

func (fakeAPM) Summary(_ context.Context, w time.Duration) (apm.Summary, error) {
	if w > apm.MaxWindow {
		return apm.Summary{}, apm.ErrInvalidWindow
	}
	return apm.Summary{}, nil
}

func (fakeAPM) Series(_ context.Context, r timeseries.Range, _ string) (apm.Series, error) {
	if r.Interval == timeseries.Week {
		return apm.Series{}, apm.ErrInvalidRange
	}
	return apm.Series{Interval: r.Interval}, nil
}
func (fakeAPM) Endpoints(context.Context, time.Time, time.Time, string, int) ([]apm.Endpoint, error) {
	return nil, nil
}
func (fakeAPM) ErrorGroups(context.Context, time.Time, time.Time, string) ([]apm.ErrorGroup, error) {
	return nil, nil
}
func (fakeAPM) JVM(_ context.Context, _ timeseries.Range, service string) (apm.JVMSeries, error) {
	if service == "" {
		return apm.JVMSeries{}, apm.ErrInvalidRange
	}
	return apm.JVMSeries{Service: service}, nil
}

type fakeHealth struct{}

func (fakeHealth) Report(context.Context) (health.Report, error) { return health.Report{}, nil }

type fakeVerifier struct{}

func (fakeVerifier) Verify(_ context.Context, token string) (*cfaccess.Claims, error) {
	if token == "good" {
		return &cfaccess.Claims{Email: "a@b.c"}, nil
	}
	return nil, errors.New("bad token")
}

func newTestRouter(v TokenVerifier) (http.Handler, *fakeStats) {
	fs := &fakeStats{}
	static := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("spa")) })
	kst := time.FixedZone("KST", 9*3600)
	return New(Deps{Stats: fs, APM: fakeAPM{}, Health: fakeHealth{}, Verifier: v, Static: static, Location: kst}), fs
}

func do(h http.Handler, path string, header map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for k, v := range header {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestUnknownAPIReturnsJSON404NotSPA(t *testing.T) {
	h, _ := newTestRouter(nil)
	rec := do(h, "/api/nope", nil)
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("code=%d ct=%q body=%q", rec.Code, rec.Header().Get("Content-Type"), rec.Body.String())
	}
	if rec := do(h, "/users", nil); rec.Body.String() != "spa" {
		t.Fatalf("API 가 아닌 경로는 SPA 로 가야 한다: %q", rec.Body.String())
	}
}

func TestAccessAuth(t *testing.T) {
	h, _ := newTestRouter(fakeVerifier{})

	if rec := do(h, "/api/stats/overview", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("토큰 없이 API 접근: code=%d", rec.Code)
	}
	if rec := do(h, "/", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("토큰 없이 화면 접근: code=%d", rec.Code)
	}
	if rec := do(h, "/api/stats/overview", map[string]string{cfaccess.HeaderName: "good"}); rec.Code != http.StatusOK {
		t.Errorf("유효한 토큰: code=%d", rec.Code)
	}
	for _, p := range []string{"/livez", "/readyz"} {
		if rec := do(h, p, nil); rec.Code != http.StatusOK {
			t.Errorf("%s 는 토큰 없이 통과해야 한다(kubelet): code=%d", p, rec.Code)
		}
	}
}

func TestTimeseriesValidation(t *testing.T) {
	h, fs := newTestRouter(nil)

	bad := []string{
		"/api/stats/timeseries?metric=signups&interval=minute",
		"/api/stats/timeseries?metric=signups&from=yesterday",
		"/api/stats/timeseries?metric=signups&from=2025-01-01T00:00:00Z&to=2026-01-01T00:00:00Z&interval=hour", // 버킷 초과
		"/api/stats/timeseries?metric=nope",
	}
	for _, p := range bad {
		if rec := do(h, p, nil); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: code=%d, 400 이어야 한다", p, rec.Code)
		}
	}

	rec := do(h, "/api/stats/timeseries?metric=signups", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("기본값 요청: code=%d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["interval"] != "day" {
		t.Errorf("interval 기본값은 day: %v", body["interval"])
	}
	if days := fs.gotRange.To.Sub(fs.gotRange.From).Hours() / 24; days < 30 || days > 31 {
		t.Errorf("기본 기간은 최근 30일이어야 한다: %.1f일", days)
	}
}

func TestAPMWindowValidation(t *testing.T) {
	h, _ := newTestRouter(nil)
	if rec := do(h, "/api/apm/summary?minutes=abc", nil); rec.Code != http.StatusBadRequest {
		t.Errorf("code=%d", rec.Code)
	}
	if rec := do(h, "/api/apm/summary?minutes=99999", nil); rec.Code != http.StatusBadRequest {
		t.Errorf("code=%d", rec.Code)
	}
}

func TestFeedbackReturnsEmptyArrayNotNull(t *testing.T) {
	h, _ := newTestRouter(nil)
	rec := do(h, "/api/stats/feedback", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"items":[]`) {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec := do(h, "/api/stats/feedback?limit=x", nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("limit 형식 오류는 400: code=%d", rec.Code)
	}
}

func TestAPMEndpoints(t *testing.T) {
	h, _ := newTestRouter(nil)
	ok := []string{"/api/apm/series", "/api/apm/series?interval=5m&from=" + time.Now().Add(-time.Hour).UTC().Format(time.RFC3339),
		"/api/apm/endpoints", "/api/apm/errors", "/api/apm/jvm?service=planmate-gateway"}
	for _, p := range ok {
		if rec := do(h, p, nil); rec.Code != http.StatusOK {
			t.Errorf("%s: code=%d body=%s", p, rec.Code, rec.Body.String())
		}
	}
	bad := []string{"/api/apm/series?interval=week", "/api/apm/jvm", "/api/apm/series?interval=5m&from=2026-01-01T00:00:00Z",
		"/api/apm/endpoints?limit=x"}
	for _, p := range bad {
		if rec := do(h, p, nil); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: code=%d, 400 이어야 한다", p, rec.Code)
		}
	}
	if rec := do(h, "/api/apm/errors", nil); !strings.Contains(rec.Body.String(), `"items":[]`) {
		t.Errorf("빈 목록은 [] 여야 한다: %s", rec.Body.String())
	}
}
