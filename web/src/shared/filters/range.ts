import { useSearchParams } from 'react-router-dom'

// 기간 필터. URL(?range=30d)에 저장해서 새로고침·링크 공유에도 유지된다.
//
// 데이터 출처마다 지원하는 간격과 기간이 다르다:
//   - 사용자 통계(DB date_trunc): hour, day, week, month
//   - APM(ES|QL): 5m, 15m, hour, day, 최대 7일 — 그보다 긴 기간을 고르면 최근 7일로 줄여서 보여준다.
// 서버의 버킷 수 상한(통계 1000, APM 500)을 넘지 않도록 간격을 골라 두었다.

export type RangeKey = '1h' | '6h' | '24h' | '7d' | '30d' | '90d'
export type StatsInterval = 'hour' | 'day' | 'week' | 'month'
export type ApmInterval = '5m' | '15m' | 'hour' | 'day'

interface RangeDef {
  key: RangeKey
  label: string
  hours: number
  statsInterval: StatsInterval
  apmInterval: ApmInterval
}

export const RANGES: RangeDef[] = [
  { key: '1h', label: '1시간', hours: 1, statsInterval: 'hour', apmInterval: '5m' },
  { key: '6h', label: '6시간', hours: 6, statsInterval: 'hour', apmInterval: '15m' },
  { key: '24h', label: '24시간', hours: 24, statsInterval: 'hour', apmInterval: 'hour' },
  { key: '7d', label: '7일', hours: 24 * 7, statsInterval: 'day', apmInterval: 'hour' },
  { key: '30d', label: '30일', hours: 24 * 30, statsInterval: 'day', apmInterval: 'day' },
  { key: '90d', label: '90일', hours: 24 * 90, statsInterval: 'week', apmInterval: 'day' },
]

const DEFAULT: RangeKey = '30d'
const APM_MAX_HOURS = 24 * 7

export interface Span<I extends string> {
  from: string
  to: string
  interval: I
  label: string
}

export interface TimeRange {
  key: RangeKey
  stats: Span<StatsInterval>
  apm: Span<ApmInterval> & { clamped: boolean } // clamped: 고른 기간이 APM 상한(7일)을 넘어 줄였는지
}

// from/to 를 분 단위로 내려서, 같은 분 안의 요청은 같은 쿼리 키(=캐시)를 쓰게 한다.
export function toTimeRange(key: RangeKey, now = new Date()): TimeRange {
  const r = RANGES.find((x) => x.key === key) ?? RANGES.find((x) => x.key === DEFAULT)!
  const to = new Date(Math.floor(now.getTime() / 60_000) * 60_000)
  const fromOf = (hours: number) => new Date(to.getTime() - hours * 3_600_000).toISOString()
  const apmHours = Math.min(r.hours, APM_MAX_HOURS)
  return {
    key: r.key,
    stats: { from: fromOf(r.hours), to: to.toISOString(), interval: r.statsInterval, label: r.label },
    apm: {
      from: fromOf(apmHours),
      to: to.toISOString(),
      interval: r.apmInterval,
      label: apmHours < r.hours ? '7일' : r.label,
      clamped: apmHours < r.hours,
    },
  }
}

export function useRangeFilter() {
  const [params, setParams] = useSearchParams()
  const raw = params.get('range')
  const key: RangeKey = RANGES.some((r) => r.key === raw) ? (raw as RangeKey) : DEFAULT

  const setKey = (next: RangeKey) =>
    setParams(
      (prev) => {
        const p = new URLSearchParams(prev)
        p.set('range', next)
        return p
      },
      { replace: true },
    )

  return { key, range: toTimeRange(key), setKey }
}
