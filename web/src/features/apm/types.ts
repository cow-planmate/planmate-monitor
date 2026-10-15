// 서버 internal/apm 의 JSON 응답과 1:1 로 맞춘다.

export interface ServiceSummary {
  service: string
  requests: number
  failures: number
  errorRate: number // 0~1
  p95Ms: number | null
  rpm: number
}

export interface ApmSummary {
  from: string
  to: string
  overall: ServiceSummary
  services: ServiceSummary[] | null
}

// /api/apm/series — 배열은 buckets 와 같은 길이다. 요청 없는 버킷의 에러율·응답시간은 null.
export interface SeriesMetrics {
  service: string
  requests: number[]
  failures: number[]
  errorRate: (number | null)[]
  p50Ms: (number | null)[]
  p95Ms: (number | null)[]
  p99Ms: (number | null)[]
}

export interface ApmSeries {
  interval: string
  buckets: string[]
  overall: SeriesMetrics
  services: SeriesMetrics[] | null
}

export interface Endpoint {
  service: string
  name: string
  requests: number
  failures: number
  errorRate: number
  p95Ms: number | null
}

export interface ErrorGroup {
  groupingKey: string
  service: string
  message: string
  type: string
  culprit: string
  transaction: string
  count: number
  firstSeen: string
  lastSeen: string
}

export interface JvmSeries {
  service: string
  interval: string
  buckets: string[]
  heapUsedMB: (number | null)[]
  heapMaxMB: (number | null)[]
  threads: (number | null)[]
  gcTimeMs: (number | null)[]
}
