import type { Tone } from '../../shared/ui/KpiCard'

// 색 기준. 실제 운영 수치를 보면서 조정한다(2단계 알림 규칙과 같은 값을 쓰게 될 것).
export const ERROR_RATE_WARN = 0.01
export const ERROR_RATE_DANGER = 0.05
export const P95_WARN_MS = 500
export const P95_DANGER_MS = 1500

export function errorRateTone(rate: number | null | undefined, requests: number): Tone {
  // 요청이 거의 없을 때(예: 새벽 2건 중 1건 실패) 비율만 보고 경고하면 오탐이 된다.
  if (rate == null || requests < 20) return 'neutral'
  if (rate >= ERROR_RATE_DANGER) return 'danger'
  if (rate >= ERROR_RATE_WARN) return 'warn'
  return 'ok'
}

export function latencyTone(p95: number | null | undefined): Tone {
  if (p95 == null) return 'neutral'
  if (p95 >= P95_DANGER_MS) return 'danger'
  if (p95 >= P95_WARN_MS) return 'warn'
  return 'ok'
}
