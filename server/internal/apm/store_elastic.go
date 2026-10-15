package apm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cow-planmate/planmate-monitor/server/internal/platform/elastic"
	"github.com/cow-planmate/planmate-monitor/server/internal/timeseries"
)

// 이 파일의 ES|QL 은 APM 인덱스 필드에 대한 계약이다. 필드 이름은 로컬 APM Server 8.15 에 Java 에이전트
// 형식으로 넣은 데이터에서 확인했다. 운영 인덱스에서 한 번 더 확인할 것(특히 metrics-apm.internal*).
//
// 쿼리 문자열에 들어가는 값은 모두 ? 파라미터다. fmt 로 끼워 넣는 것은 이 파일의 상수와
// 검증된 정수(버킷 식, LIMIT)뿐이다.

// requestFilter 는 "사용자 요청 트랜잭션" 조건이다.
// 헬스체크·actuator 호출은 트래픽이 적은 우리 서비스에서 처리량의 상당 부분을 차지하고
// p95 를 실제보다 낮게 만든다. 운영 데이터의 transaction.name 분포를 보고 확정할 것.
const requestFilter = `@timestamp >= TO_DATETIME(?) AND @timestamp < TO_DATETIME(?)
    AND processor.event == "transaction"
    AND transaction.type == "request"
    AND service.environment == ?
    AND NOT (transaction.name LIKE "*actuator*" OR transaction.name LIKE "*health*")`

const esqlSummary = `
FROM traces-apm*
| WHERE ` + requestFilter + `%s
| EVAL failed = CASE(event.outcome == "failure", 1, 0)
| STATS requests = COUNT(*), failures = SUM(failed), p95 = PERCENTILE(transaction.duration.us, 95)%s
%s`

const esqlSeries = `
FROM traces-apm*
| WHERE ` + requestFilter + `%s
| EVAL failed = CASE(event.outcome == "failure", 1, 0), bucket = %s
| STATS requests = COUNT(*), failures = SUM(failed),
        p50 = PERCENTILE(transaction.duration.us, 50),
        p95 = PERCENTILE(transaction.duration.us, 95),
        p99 = PERCENTILE(transaction.duration.us, 99) BY bucket%s
| LIMIT 10000`

const esqlEndpoints = `
FROM traces-apm*
| WHERE ` + requestFilter + `%s
| EVAL failed = CASE(event.outcome == "failure", 1, 0)
| STATS requests = COUNT(*), failures = SUM(failed), p95 = PERCENTILE(transaction.duration.us, 95)
    BY service.name, transaction.name
| WHERE requests >= %d
| SORT p95 DESC
| LIMIT %d`

// "first" 는 ES|QL 예약어라 first_seen 으로 쓴다.
const esqlErrors = `
FROM logs-apm.error*
| WHERE @timestamp >= TO_DATETIME(?) AND @timestamp < TO_DATETIME(?) AND service.environment == ?%s
| STATS occurrences = COUNT(*),
        message = VALUES(error.grouping_name),
        type = VALUES(error.exception.type),
        culprit = VALUES(error.culprit),
        tx = VALUES(transaction.name),
        first_seen = MIN(@timestamp),
        last_seen = MAX(@timestamp)
    BY error.grouping_key, service.name
| SORT occurrences DESC
| LIMIT %d`

const esqlJVMHeap = `
FROM metrics-apm.internal*
| WHERE @timestamp >= TO_DATETIME(?) AND @timestamp < TO_DATETIME(?)
    AND service.environment == ? AND service.name == ?
    AND jvm.memory.heap.used IS NOT NULL
| EVAL bucket = %s
| STATS heap_used = AVG(jvm.memory.heap.used), heap_max = MAX(jvm.memory.heap.max), threads = AVG(jvm.thread.count) BY bucket
| LIMIT 10000`

// jvm.gc.time 은 JVM 시작 이후 누적값(ms)이다. 버킷마다 최댓값을 받아 서비스 계층이 증가분을 계산한다.
const esqlJVMGC = `
FROM metrics-apm.internal*
| WHERE @timestamp >= TO_DATETIME(?) AND @timestamp < TO_DATETIME(?)
    AND service.environment == ? AND service.name == ?
    AND jvm.gc.time IS NOT NULL
| EVAL bucket = %s
| STATS gc_time = MAX(jvm.gc.time) BY bucket, labels.name
| LIMIT 10000`

type ESStore struct {
	es          *elastic.Client
	environment string
	loc         *time.Location
}

func NewESStore(es *elastic.Client, environment string, loc *time.Location) *ESStore {
	return &ESStore{es: es, environment: environment, loc: loc}
}

// args 는 시간 구간·환경 파라미터를 만들고, service 가 있으면 서비스 조건과 파라미터를 덧붙인다.
func (s *ESStore) args(from, to time.Time, service string) (string, []any) {
	params := []any{ts(from), ts(to), s.environment}
	if service == "" {
		return "", params
	}
	return "\n    AND service.name == ?", append(params, service)
}

func (s *ESStore) Summarize(ctx context.Context, from, to time.Time, byService bool) ([]ServiceSummary, error) {
	by, sort := "", ""
	if byService {
		by, sort = " BY service.name", "| SORT service.name"
	}
	where, params := s.args(from, to, "")
	res, err := s.query(ctx, fmt.Sprintf(esqlSummary, where, by, sort), params)
	if err != nil || res == nil {
		return nil, err
	}
	var out []ServiceSummary
	for _, row := range res.Rows() {
		out = append(out, ServiceSummary{
			Service:  str(row["service.name"]),
			Requests: toInt(row["requests"]),
			Failures: toInt(row["failures"]),
			P95Ms:    usToMs(row["p95"]),
		})
	}
	return out, nil
}

func (s *ESStore) Series(ctx context.Context, r timeseries.Range, service string, byService bool) ([]SeriesRow, error) {
	by := ""
	if byService {
		by = ", service.name"
	}
	where, params := s.args(r.From, r.To, service)
	res, err := s.query(ctx, fmt.Sprintf(esqlSeries, where, bucketExpr(r.Interval, s.loc), by), params)
	if err != nil || res == nil {
		return nil, err
	}
	var out []SeriesRow
	for _, row := range res.Rows() {
		b, err := parseTime(row["bucket"])
		if err != nil {
			return nil, err
		}
		out = append(out, SeriesRow{
			Bucket:   b,
			Service:  str(row["service.name"]),
			Requests: toInt(row["requests"]),
			Failures: toInt(row["failures"]),
			P50Ms:    usToMs(row["p50"]),
			P95Ms:    usToMs(row["p95"]),
			P99Ms:    usToMs(row["p99"]),
		})
	}
	return out, nil
}

func (s *ESStore) Endpoints(ctx context.Context, from, to time.Time, service string, limit int) ([]Endpoint, error) {
	where, params := s.args(from, to, service)
	res, err := s.query(ctx, fmt.Sprintf(esqlEndpoints, where, minEndpointReqs, limit), params)
	if err != nil || res == nil {
		return nil, err
	}
	var out []Endpoint
	for _, row := range res.Rows() {
		e := Endpoint{
			Service:  str(row["service.name"]),
			Name:     str(row["transaction.name"]),
			Requests: toInt(row["requests"]),
			Failures: toInt(row["failures"]),
			P95Ms:    usToMs(row["p95"]),
		}
		if e.Requests > 0 {
			e.ErrorRate = float64(e.Failures) / float64(e.Requests)
		}
		out = append(out, e)
	}
	return out, nil
}

func (s *ESStore) ErrorGroups(ctx context.Context, from, to time.Time, service string, limit int) ([]ErrorGroup, error) {
	where, params := s.args(from, to, service)
	res, err := s.query(ctx, fmt.Sprintf(esqlErrors, where, limit), params)
	if err != nil || res == nil {
		return nil, err
	}
	var out []ErrorGroup
	for _, row := range res.Rows() {
		g := ErrorGroup{
			GroupingKey: str(row["error.grouping_key"]),
			Service:     str(row["service.name"]),
			Message:     first(row["message"]),
			Type:        first(row["type"]),
			Culprit:     first(row["culprit"]),
			Transaction: first(row["tx"]),
			Count:       toInt(row["occurrences"]),
		}
		if g.FirstSeen, err = parseTime(row["first_seen"]); err != nil {
			return nil, err
		}
		if g.LastSeen, err = parseTime(row["last_seen"]); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, nil
}

func (s *ESStore) JVM(ctx context.Context, r timeseries.Range, service string) ([]JVMRow, []GCRow, error) {
	params := []any{ts(r.From), ts(r.To), s.environment, service}
	bucket := bucketExpr(r.Interval, s.loc)

	heap, err := s.query(ctx, fmt.Sprintf(esqlJVMHeap, bucket), params)
	if err != nil {
		return nil, nil, err
	}
	var jvm []JVMRow
	if heap != nil {
		for _, row := range heap.Rows() {
			b, err := parseTime(row["bucket"])
			if err != nil {
				return nil, nil, err
			}
			jvm = append(jvm, JVMRow{
				Bucket:     b,
				HeapUsedMB: bytesToMB(row["heap_used"]),
				HeapMaxMB:  bytesToMB(row["heap_max"]),
				Threads:    num(row["threads"]),
			})
		}
	}

	gcRes, err := s.query(ctx, fmt.Sprintf(esqlJVMGC, bucket), params)
	if err != nil {
		return nil, nil, err
	}
	var gc []GCRow
	if gcRes != nil {
		for _, row := range gcRes.Rows() {
			b, err := parseTime(row["bucket"])
			if err != nil {
				return nil, nil, err
			}
			v := num(row["gc_time"])
			if v == nil {
				continue
			}
			gc = append(gc, GCRow{Bucket: b, Collector: str(row["labels.name"]), TimeMs: *v})
		}
	}
	return jvm, gc, nil
}

// query 는 ES|QL 을 실행한다. 인덱스나 필드가 아직 없으면(데이터가 한 번도 안 들어옴) 에러가 아니라 nil 결과다.
func (s *ESStore) query(ctx context.Context, q string, params []any) (*elastic.ESQLResult, error) {
	res, err := s.es.ESQL(ctx, q, params...)
	if isMissingData(err) {
		return nil, nil
	}
	return res, err
}

// bucketExpr 는 @timestamp 를 버킷 시작 시각으로 자르는 ES|QL 식이다.
// ES|QL 의 DATE_TRUNC 는 UTC 기준이라, 하루 단위는 표시 시간대만큼 밀었다가 자르고 되돌린다.
// (KST 는 서머타임이 없어 오프셋이 고정이다.)
func bucketExpr(iv timeseries.Interval, loc *time.Location) string {
	switch iv {
	case timeseries.Minute5:
		return "DATE_TRUNC(5 minutes, @timestamp)"
	case timeseries.Minute15:
		return "DATE_TRUNC(15 minutes, @timestamp)"
	case timeseries.Hour:
		return "DATE_TRUNC(1 hour, @timestamp)"
	default:
		_, off := time.Now().In(loc).Zone()
		return fmt.Sprintf("DATE_TRUNC(1 day, @timestamp + %d seconds) - %d seconds", off, off)
	}
}

func isMissingData(err error) bool {
	var esErr *elastic.Error
	return errors.As(err, &esErr) && esErr.Status == 400 &&
		(strings.Contains(esErr.Body, "Unknown index") || strings.Contains(esErr.Body, "Unknown column"))
}

func ts(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func parseTime(v any) (time.Time, error) {
	s, _ := v.(string)
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("ES|QL 시각 %q: %w", s, err)
	}
	return t, nil
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

// first 는 VALUES() 결과의 첫 값이다. 값이 하나면 문자열, 여러 개면 배열로 온다.
func first(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case []any:
		if len(x) > 0 {
			return str(x[0])
		}
	}
	return ""
}

// ES|QL 응답의 숫자는 JSON 디코딩 후 float64 로 온다.
func toInt(v any) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int64:
		return n
	}
	return 0
}

func num(v any) *float64 {
	if f, ok := v.(float64); ok {
		return &f
	}
	return nil
}

func usToMs(v any) *float64 {
	if f := num(v); f != nil {
		ms := *f / 1000
		return &ms
	}
	return nil
}

func bytesToMB(v any) *float64 {
	if f := num(v); f != nil {
		mb := *f / (1024 * 1024)
		return &mb
	}
	return nil
}
