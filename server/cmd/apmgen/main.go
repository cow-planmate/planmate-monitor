// apmgen 은 로컬 APM Server 에 가짜 트랜잭션·에러·JVM 메트릭을 보내는 개발용 도구다.
//
// 실제 APM Server 의 intake API 로 보내므로 Elasticsearch 에는 운영(Java 에이전트 1.52)과 같은 필드
// 구조로 저장된다. 화면 확인용으로 다음 상황을 일부러 섞는다:
//
//   - 시간대별 트래픽(새벽에 적고 저녁에 많다)
//
//   - 헬스체크 트랜잭션 — 대시보드가 걸러내야 한다
//
//   - 장애 구간: 2시간 40분 전부터 20분간 backend-v2 지연 4배, 에러율 15%
//
//   - 여러 종류의 에러 — 에러 그룹 목록 확인용
//
//   - JVM 힙(톱니 모양)·GC 누적 카운터·스레드 수, notification 은 6시간 전에 한 번 재시작(카운터 초기화)
//
//     go run ./cmd/apmgen                 # 최근 24시간치를 채우고 끝
//     go run ./cmd/apmgen -live           # 채운 뒤 10초마다 계속 보낸다
package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"log"
	"math"
	mrand "math/rand/v2"
	"net/http"
	"time"
)

type failure struct {
	typ, message, culprit string
}

type endpoint struct {
	name     string
	baseMs   float64
	failRate float64
	failures []failure
}

type service struct {
	name      string
	endpoints []endpoint
	heapMaxMB float64
	threads   float64

	// GC 누적 카운터. Java 에이전트는 JVM 시작 이후 누적값을 보낸다.
	gcYoungMs, gcOldMs, gcYoungCount float64
	restartAt                        time.Time
}

var (
	dbTimeout = failure{"org.springframework.dao.QueryTimeoutException", "JDBC query timed out after 3000ms", "com.planmate.domain.plan.repository.PlanRepository"}
	poolEmpty = failure{"org.postgresql.util.PSQLException", "HikariPool-1 - Connection is not available, request timed out after 30000ms", "com.zaxxer.hikari.pool.HikariPool"}
	npe       = failure{"java.lang.NullPointerException", "Cannot invoke \"String.length()\" because \"title\" is null", "com.planmate.community.domain.post.service.PostService"}
	extAPI    = failure{"org.springframework.web.client.ResourceAccessException", "I/O error on GET request for \"https://apis.data.go.kr\": Read timed out", "com.planmate.domain.place.client.TourApiClient"}
	notFound  = failure{"com.planmate.common.exception.PlanNotFoundException", "Plan not found", "com.planmate.domain.plan.service.PlanService"}
	upstream  = failure{"io.netty.channel.ConnectTimeoutException", "connection timed out: planmate-app/10.43.12.7:8080", "org.springframework.cloud.gateway.filter.NettyRoutingFilter"}
	fcm       = failure{"com.google.firebase.messaging.FirebaseMessagingException", "Requested entity was not found.", "com.planmate.notification.push.FcmSender"}
)

func newServices(now time.Time) []*service {
	return []*service{
		{name: "planmate-backend-v2", heapMaxMB: 256, threads: 48, endpoints: []endpoint{
			{"GET /api/plans", 45, 0.004, []failure{dbTimeout}},
			{"POST /api/plans", 120, 0.015, []failure{dbTimeout, poolEmpty}},
			{"GET /api/plans/{planId}", 60, 0.006, []failure{notFound}},
			{"GET /api/places/search", 220, 0.025, []failure{extAPI}},
			{"GET /api/destinations/tree", 35, 0.001, []failure{dbTimeout}},
		}},
		{name: "planmate-gateway", heapMaxMB: 256, threads: 32, endpoints: []endpoint{
			{"GET /api/**", 15, 0.002, []failure{upstream}},
			{"POST /api/**", 25, 0.004, []failure{upstream}},
		}},
		{name: "planmate-community", heapMaxMB: 512, threads: 40, endpoints: []endpoint{
			{"GET /api/community/posts", 70, 0.006, []failure{dbTimeout}},
			{"POST /api/community/posts", 150, 0.02, []failure{npe}},
			{"GET /api/community/feeds", 90, 0.008, []failure{dbTimeout}},
			{"POST /api/community/feeds/{feedId}/fork", 260, 0.01, []failure{notFound}},
		}},
		{name: "planmate-notification", heapMaxMB: 256, threads: 24, restartAt: now.Add(-6 * time.Hour), endpoints: []endpoint{
			{"GET /api/notifications", 30, 0.004, []failure{dbTimeout}},
			{"POST /api/notifications/read", 25, 0.01, []failure{fcm}},
		}},
	}
}

var kst = time.FixedZone("KST", 9*3600)

// trafficFactor 는 KST 시각별 트래픽 배율이다. 새벽에 적고 저녁에 많다.
func trafficFactor(t time.Time) float64 {
	h := float64(t.In(kst).Hour()) + float64(t.In(kst).Minute())/60
	return 0.15 + 1.1*math.Pow(math.Sin(math.Pi*math.Max(0, h-6)/17), 2)
}

func main() {
	url := flag.String("url", "http://localhost:8200", "APM Server 주소")
	token := flag.String("token", "local-token", "APM secret token")
	env := flag.String("env", "local", "service.environment")
	backfill := flag.Duration("backfill", 24*time.Hour, "과거 몇 시간치를 채울지")
	perMin := flag.Float64("rpm", 20, "서비스당 분당 기본 요청 수(시간대 배율이 곱해진다)")
	live := flag.Bool("live", false, "채운 뒤 계속 보낸다")
	flag.Parse()

	now := time.Now()
	g := &generator{
		url: *url, token: *token, env: *env, rpm: *perMin,
		services:      newServices(now),
		incidentStart: now.Add(-160 * time.Minute),
		incidentEnd:   now.Add(-140 * time.Minute),
	}

	start := now.Add(-*backfill)
	for t := start; t.Before(now); t = t.Add(time.Minute) {
		g.send(t, time.Minute)
	}
	log.Printf("최근 %s 채움 (장애 구간 %s ~ %s)", *backfill,
		g.incidentStart.In(kst).Format("15:04"), g.incidentEnd.In(kst).Format("15:04"))

	if !*live {
		return
	}
	tick := time.NewTicker(10 * time.Second)
	defer tick.Stop()
	for t := range tick.C {
		g.send(t.Add(-10*time.Second), 10*time.Second)
	}
}

type generator struct {
	url, token, env string
	rpm             float64
	services        []*service
	incidentStart   time.Time
	incidentEnd     time.Time
}

func (g *generator) inIncident(svc string, t time.Time) bool {
	return svc == "planmate-backend-v2" && !t.Before(g.incidentStart) && t.Before(g.incidentEnd)
}

// send 는 [start, start+span) 동안의 이벤트를 서비스마다 한 번의 intake 요청으로 보낸다.
func (g *generator) send(start time.Time, span time.Duration) {
	for _, s := range g.services {
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		_ = enc.Encode(map[string]any{"metadata": map[string]any{"service": map[string]any{
			"name":        s.name,
			"environment": g.env,
			"agent":       map[string]string{"name": "java", "version": "1.52.0"},
			"language":    map[string]string{"name": "Java"},
			"runtime":     map[string]string{"name": "Java", "version": "17.0.12"},
		}}})

		n := poisson(g.rpm * trafficFactor(start) * span.Minutes())
		for range n {
			ts := start.Add(time.Duration(mrand.Int64N(int64(span))))
			ep := s.endpoints[mrand.IntN(len(s.endpoints))]
			ms, failRate := ep.baseMs*(0.5+mrand.ExpFloat64()), ep.failRate
			if g.inIncident(s.name, ts) {
				ms, failRate = ms*4, 0.15
			}
			var f *failure
			if mrand.Float64() < failRate {
				fl := ep.failures[mrand.IntN(len(ep.failures))]
				if g.inIncident(s.name, ts) {
					fl = poolEmpty
				}
				f = &fl
			}
			writeTx(enc, ts, ep.name, ms, f)
		}
		// 헬스체크: 대시보드가 걸러내야 하는 트래픽. 15초마다 한 번, 아주 빠르다.
		for t := start.Truncate(15 * time.Second); t.Before(start.Add(span)); t = t.Add(15 * time.Second) {
			if !t.Before(start) {
				writeTx(enc, t, "GET /actuator/health", 2, nil)
			}
		}
		// JVM 메트릭: Java 에이전트처럼 30초마다.
		for t := start.Truncate(30 * time.Second); t.Before(start.Add(span)); t = t.Add(30 * time.Second) {
			if !t.Before(start) {
				g.writeJVM(enc, s, t)
			}
		}

		req, _ := http.NewRequest(http.MethodPost, g.url+"/intake/v2/events", &buf)
		req.Header.Set("Content-Type", "application/x-ndjson")
		req.Header.Set("Authorization", "Bearer "+g.token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Fatalf("APM Server 요청 실패: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode >= 300 {
			log.Fatalf("APM Server 응답 %d (%s)", resp.StatusCode, s.name)
		}
	}
}

func (g *generator) writeJVM(enc *json.Encoder, s *service, t time.Time) {
	if !s.restartAt.IsZero() && !t.Before(s.restartAt) && t.Before(s.restartAt.Add(30*time.Second)) {
		s.gcYoungMs, s.gcOldMs, s.gcYoungCount = 0, 0, 0 // 재시작: 누적 카운터가 0부터 다시 시작한다
	}
	load := trafficFactor(t)
	if g.inIncident(s.name, t) {
		load *= 2.5
	}
	// 힙은 GC 주기마다 떨어지는 톱니 모양.
	phase := math.Mod(float64(t.Unix())/600, 1)
	heapUsed := s.heapMaxMB * (0.3 + 0.45*phase*math.Min(1, 0.5+load/2))
	s.gcYoungCount += 1 + 2*load
	s.gcYoungMs += (3 + 6*load) * (0.8 + 0.4*mrand.Float64())
	if mrand.Float64() < 0.02*load {
		s.gcOldMs += 40 + 60*mrand.Float64()
	}
	mb := 1024.0 * 1024
	us := t.UnixMicro()

	_ = enc.Encode(map[string]any{"metricset": map[string]any{
		"timestamp": us,
		"samples": map[string]any{
			"jvm.memory.heap.used":      map[string]float64{"value": heapUsed * mb},
			"jvm.memory.heap.committed": map[string]float64{"value": s.heapMaxMB * 0.8 * mb},
			"jvm.memory.heap.max":       map[string]float64{"value": s.heapMaxMB * mb},
			"jvm.thread.count":          map[string]float64{"value": math.Round(s.threads + 6*load)},
		},
	}})
	for _, gc := range []struct {
		name        string
		timeMs, cnt float64
	}{
		{"G1 Young Generation", s.gcYoungMs, s.gcYoungCount},
		{"G1 Old Generation", s.gcOldMs, math.Floor(s.gcOldMs / 70)},
	} {
		_ = enc.Encode(map[string]any{"metricset": map[string]any{
			"timestamp": us,
			"tags":      map[string]string{"name": gc.name},
			"samples": map[string]any{
				"jvm.gc.time":  map[string]float64{"value": math.Round(gc.timeMs)},
				"jvm.gc.count": map[string]float64{"value": gc.cnt},
			},
		}})
	}
}

func writeTx(enc *json.Encoder, ts time.Time, name string, ms float64, f *failure) {
	traceID, txID := hexID(16), hexID(8)
	outcome, result, status := "success", "HTTP 2xx", 200
	if f != nil {
		outcome, result, status = "failure", "HTTP 5xx", 500
	}
	_ = enc.Encode(map[string]any{"transaction": map[string]any{
		"id":         txID,
		"trace_id":   traceID,
		"name":       name,
		"type":       "request",
		"duration":   ms,
		"timestamp":  ts.UnixMicro(),
		"outcome":    outcome,
		"result":     result,
		"sampled":    true,
		"span_count": map[string]int{"started": 0},
		"context": map[string]any{
			"response": map[string]any{"status_code": status},
		},
	}})
	if f != nil {
		_ = enc.Encode(map[string]any{"error": map[string]any{
			"id":             hexID(16),
			"trace_id":       traceID,
			"transaction_id": txID,
			"parent_id":      txID,
			"timestamp":      ts.UnixMicro(),
			"culprit":        f.culprit,
			"transaction":    map[string]any{"name": name, "type": "request", "sampled": true},
			"exception": map[string]any{
				"type":    f.typ,
				"message": f.message,
				"stacktrace": []map[string]any{
					{"classname": f.culprit, "function": "handle", "filename": "Handler.java", "lineno": 42},
				},
			},
		}})
	}
}

// poisson 은 평균 λ 인 포아송 난수다(트래픽이 매 분 똑같지 않게).
func poisson(lambda float64) int {
	if lambda <= 0 {
		return 0
	}
	if lambda > 50 {
		return int(math.Max(0, math.Round(lambda+math.Sqrt(lambda)*mrand.NormFloat64())))
	}
	l, k, p := math.Exp(-lambda), 0, 1.0
	for {
		p *= mrand.Float64()
		if p <= l {
			return k
		}
		k++
	}
}

func hexID(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
