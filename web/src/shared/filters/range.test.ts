import { describe, expect, it } from 'vitest'
import { toTimeRange } from './range'

describe('toTimeRange', () => {
  const now = new Date('2026-10-15T11:39:42.123Z')

  it('끝 시각을 분 단위로 내린다', () => {
    expect(toTimeRange('7d', now).stats.to).toBe('2026-10-15T11:39:00.000Z')
  })

  it('출처마다 맞는 간격을 고른다', () => {
    expect(toTimeRange('1h', now).apm.interval).toBe('5m')
    expect(toTimeRange('1h', now).stats.interval).toBe('hour')
    expect(toTimeRange('90d', now).stats.interval).toBe('week')
  })

  it('APM 은 7일보다 긴 기간을 7일로 줄인다', () => {
    const r = toTimeRange('30d', now)
    const span = (s: { from: string; to: string }) => new Date(s.to).getTime() - new Date(s.from).getTime()
    expect(span(r.stats)).toBe(30 * 24 * 3_600_000)
    expect(span(r.apm)).toBe(7 * 24 * 3_600_000)
    expect(r.apm.clamped).toBe(true)
    expect(toTimeRange('24h', now).apm.clamped).toBe(false)
  })
})
