// Package apm 은 Elastic APM 데이터(Elasticsearch)에서 성능·에러 지표를 뽑는다.
//
// 대상은 APM 에이전트가 붙은 서비스(backend-v2, gateway, community, notification)다.
// planmate-ai 는 에이전트가 없어 여기 나오지 않는다.
//
// 인덱스(APM Server 8.15 기준, 로컬에서 실제 저장 위치를 확인함):
//   - traces-apm*            트랜잭션 원본
//   - logs-apm.error*        에러
//   - metrics-apm.internal*  Java 에이전트의 JVM 메트릭(jvm.memory.heap.*, jvm.gc.*, jvm.thread.count)
package apm

import (
	"errors"
	"time"

	"github.com/cow-planmate/planmate-monitor/server/internal/timeseries"
)

type ServiceSummary struct {
	Service   string   `json:"service"`
	Requests  int64    `json:"requests"`
	Failures  int64    `json:"failures"`
	ErrorRate float64  `json:"errorRate"` // 0~1
	P95Ms     *float64 `json:"p95Ms"`     // 요청이 없으면 null
	RPM       float64  `json:"rpm"`
}

type Summary struct {
	From     time.Time        `json:"from"`
	To       time.Time        `json:"to"`
	Overall  ServiceSummary   `json:"overall"`
	Services []ServiceSummary `json:"services"`
}

// SeriesMetrics 는 한 서비스(또는 전체)의 버킷별 지표다. 배열은 Series.Buckets 와 같은 순서·길이다.
// 요청이 없는 버킷의 에러율·응답시간은 null 이다(0 이 아니다).
type SeriesMetrics struct {
	Service   string     `json:"service"`
	Requests  []int64    `json:"requests"`
	Failures  []int64    `json:"failures"`
	ErrorRate []*float64 `json:"errorRate"`
	P50Ms     []*float64 `json:"p50Ms"`
	P95Ms     []*float64 `json:"p95Ms"`
	P99Ms     []*float64 `json:"p99Ms"`
}

type Series struct {
	Interval timeseries.Interval `json:"interval"`
	Buckets  []time.Time         `json:"buckets"`
	Overall  SeriesMetrics       `json:"overall"`
	Services []SeriesMetrics     `json:"services"`
}

// ErrorGroup 은 같은 원인(APM 의 grouping_key)으로 묶인 에러다.
type ErrorGroup struct {
	GroupingKey string    `json:"groupingKey"`
	Service     string    `json:"service"`
	Message     string    `json:"message"`
	Type        string    `json:"type"`
	Culprit     string    `json:"culprit"`
	Transaction string    `json:"transaction"`
	Count       int64     `json:"count"`
	FirstSeen   time.Time `json:"firstSeen"`
	LastSeen    time.Time `json:"lastSeen"`
}

type Endpoint struct {
	Service   string   `json:"service"`
	Name      string   `json:"name"`
	Requests  int64    `json:"requests"`
	Failures  int64    `json:"failures"`
	ErrorRate float64  `json:"errorRate"`
	P95Ms     *float64 `json:"p95Ms"`
}

// JVMSeries 는 한 서비스의 버킷별 JVM 상태다. GCTimeMs 는 그 버킷 동안 GC 에 쓴 시간(누적값의 증가분)이다.
type JVMSeries struct {
	Service    string              `json:"service"`
	Interval   timeseries.Interval `json:"interval"`
	Buckets    []time.Time         `json:"buckets"`
	HeapUsedMB []*float64          `json:"heapUsedMB"`
	HeapMaxMB  []*float64          `json:"heapMaxMB"`
	Threads    []*float64          `json:"threads"`
	GCTimeMs   []*float64          `json:"gcTimeMs"`
}

const (
	MinWindow = time.Minute
	MaxWindow = 24 * time.Hour

	// 시계열·목록 조회 기간 상한. APM 원본 트랜잭션은 양이 많고 보존 기간도 짧다.
	MaxRange = 7 * 24 * time.Hour
	// 버킷 상한. ES|QL 결과는 버킷 × 서비스 수만큼 행이 생기고 한 번에 최대 10000행이다.
	MaxBuckets = 500

	MaxErrorGroups  = 50
	MaxEndpoints    = 50
	minEndpointReqs = 5 // 요청이 몇 건뿐인 엔드포인트의 p95 는 의미가 없다
)

var (
	ErrInvalidWindow = errors.New("invalid window")
	ErrInvalidRange  = errors.New("invalid range")
)
