package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/cow-planmate/planmate-monitor/server/internal/apm"
	"github.com/cow-planmate/planmate-monitor/server/internal/stats"
	"github.com/cow-planmate/planmate-monitor/server/internal/timeseries"
)

type handlers struct {
	d Deps
}

func (h *handlers) meta(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.d.Meta)
}

func (h *handlers) health(w http.ResponseWriter, r *http.Request) {
	rep, err := h.d.Health.Report(r.Context())
	if err != nil {
		internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, rep)
}

func (h *handlers) statsOverview(w http.ResponseWriter, r *http.Request) {
	ov, err := h.d.Stats.Overview(r.Context())
	if err != nil {
		internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, ov)
}

// GET /api/stats/timeseries?metric=signups&from=RFC3339&to=RFC3339&interval=day
// from/to 를 생략하면 최근 30일, interval 을 생략하면 day 다.
func (h *handlers) statsTimeseries(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	rng, ok := h.parseRange(w, r, 30*24*time.Hour, timeseries.Day, timeseries.MaxBuckets)
	if !ok {
		return
	}

	pts, err := h.d.Stats.Timeseries(r.Context(), stats.Metric(q.Get("metric")), rng)
	switch {
	case errors.Is(err, stats.ErrUnknownMetric), errors.Is(err, stats.ErrInvalidInterval):
		writeError(w, http.StatusBadRequest, err.Error())
	case err != nil:
		internalError(w, r, err)
	default:
		writeJSON(w, http.StatusOK, map[string]any{
			"metric":   q.Get("metric"),
			"interval": rng.Interval,
			"points":   pts,
		})
	}
}

func (h *handlers) statsPlanSummary(w http.ResponseWriter, r *http.Request) {
	sum, err := h.d.Stats.PlanSummary(r.Context())
	if err != nil {
		internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, sum)
}

// GET /api/stats/feedback?limit=20
func (h *handlers) statsFeedback(w http.ResponseWriter, r *http.Request) {
	limit, ok := intParam(w, r, "limit", 20)
	if !ok {
		return
	}
	fb, err := h.d.Stats.RecentFeedback(r.Context(), limit)
	if err != nil {
		internalError(w, r, err)
		return
	}
	if fb == nil {
		fb = []stats.Feedback{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": fb})
}

// GET /api/apm/summary?minutes=15
func (h *handlers) apmSummary(w http.ResponseWriter, r *http.Request) {
	minutes, ok := intParam(w, r, "minutes", 15)
	if !ok {
		return
	}
	sum, err := h.d.APM.Summary(r.Context(), time.Duration(minutes)*time.Minute)
	switch {
	case errors.Is(err, apm.ErrInvalidWindow):
		writeError(w, http.StatusBadRequest, "minutes 는 1 ~ 1440 사이여야 한다")
	case err != nil:
		internalError(w, r, err)
	default:
		writeJSON(w, http.StatusOK, sum)
	}
}

// GET /api/apm/series?from&to&interval&service — 기본은 최근 24시간, 1시간 간격
func (h *handlers) apmSeries(w http.ResponseWriter, r *http.Request) {
	rng, ok := h.parseRange(w, r, 24*time.Hour, timeseries.Hour, apm.MaxBuckets)
	if !ok {
		return
	}
	out, err := h.d.APM.Series(r.Context(), rng, r.URL.Query().Get("service"))
	writeAPM(w, r, out, err)
}

// GET /api/apm/endpoints?from&to&service&limit=10
func (h *handlers) apmEndpoints(w http.ResponseWriter, r *http.Request) {
	rng, ok := h.parseRange(w, r, 24*time.Hour, timeseries.Hour, apm.MaxBuckets)
	if !ok {
		return
	}
	limit, ok := intParam(w, r, "limit", 10)
	if !ok {
		return
	}
	out, err := h.d.APM.Endpoints(r.Context(), rng.From, rng.To, r.URL.Query().Get("service"), limit)
	if out == nil {
		out = []apm.Endpoint{}
	}
	writeAPM(w, r, map[string]any{"items": out}, err)
}

// GET /api/apm/errors?from&to&service
func (h *handlers) apmErrors(w http.ResponseWriter, r *http.Request) {
	rng, ok := h.parseRange(w, r, 24*time.Hour, timeseries.Hour, apm.MaxBuckets)
	if !ok {
		return
	}
	out, err := h.d.APM.ErrorGroups(r.Context(), rng.From, rng.To, r.URL.Query().Get("service"))
	if out == nil {
		out = []apm.ErrorGroup{}
	}
	writeAPM(w, r, map[string]any{"items": out}, err)
}

// GET /api/apm/jvm?service=...&from&to&interval
func (h *handlers) apmJVM(w http.ResponseWriter, r *http.Request) {
	rng, ok := h.parseRange(w, r, 24*time.Hour, timeseries.Hour, apm.MaxBuckets)
	if !ok {
		return
	}
	out, err := h.d.APM.JVM(r.Context(), rng, r.URL.Query().Get("service"))
	writeAPM(w, r, out, err)
}

// writeAPM 은 APM 서비스 결과를 쓴다. 구간·간격 검증 실패는 400, 나머지 실패는 데이터 소스 오류다.
func writeAPM(w http.ResponseWriter, r *http.Request, v any, err error) {
	switch {
	case errors.Is(err, apm.ErrInvalidRange):
		writeError(w, http.StatusBadRequest, err.Error())
	case err != nil:
		internalError(w, r, err)
	default:
		writeJSON(w, http.StatusOK, v)
	}
}

func parseTime(s string, def time.Time) (time.Time, error) {
	if s == "" {
		return def, nil
	}
	return time.Parse(time.RFC3339, s)
}

// parseRange 는 from/to/interval 쿼리 파라미터로 구간을 만든다. 형식이 틀리면 400 을 쓰고 false 를 돌려준다.
// from 을 생략하면 to 에서 defSpan 만큼 앞, to 를 생략하면 지금이다.
func (h *handlers) parseRange(w http.ResponseWriter, r *http.Request, defSpan time.Duration,
	defInterval timeseries.Interval, maxBuckets int) (timeseries.Range, bool) {
	q := r.URL.Query()
	to, err := parseTime(q.Get("to"), time.Now())
	if err != nil {
		writeError(w, http.StatusBadRequest, "to 는 RFC3339 형식이어야 한다")
		return timeseries.Range{}, false
	}
	from, err := parseTime(q.Get("from"), to.Add(-defSpan))
	if err != nil {
		writeError(w, http.StatusBadRequest, "from 은 RFC3339 형식이어야 한다")
		return timeseries.Range{}, false
	}
	iv := defInterval
	if s := q.Get("interval"); s != "" {
		if iv, err = timeseries.ParseInterval(s); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return timeseries.Range{}, false
		}
	}
	rng, err := timeseries.NewRangeMax(from, to, iv, h.d.Location, maxBuckets)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return timeseries.Range{}, false
	}
	return rng, true
}

// intParam 은 정수 쿼리 파라미터를 읽는다. 형식이 틀리면 400 을 쓰고 false 를 돌려준다.
func intParam(w http.ResponseWriter, r *http.Request, name string, def int) (int, bool) {
	v := r.URL.Query().Get(name)
	if v == "" {
		return def, true
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		writeError(w, http.StatusBadRequest, name+" 는 정수여야 한다")
		return 0, false
	}
	return n, true
}
