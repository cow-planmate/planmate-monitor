// 서버 internal/health 의 JSON 응답과 1:1 로 맞춘다.

export interface SourceStatus {
  name: string
  ok: boolean
  latencyMs: number
  error?: string
}

export interface HealthReport {
  checkedAt: string
  sources: SourceStatus[]
}
