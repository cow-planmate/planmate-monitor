import { describe, expect, it } from 'vitest'
import { formatAgo, formatBucket, formatMs, formatPercent } from './format'

describe('format', () => {
  it('UTC 버킷을 KST 날짜로 보여준다', () => {
    // UTC 10/14 15:00 = KST 10/15 00:00
    expect(formatBucket('2026-10-14T15:00:00Z', 'day')).toBe('10. 15.')
  })

  it('짧은 간격은 시:분으로 보여준다', () => {
    expect(formatBucket('2026-10-15T03:05:00Z', '5m')).toBe('12:05')
  })

  it('상대 시각', () => {
    const now = new Date('2026-10-15T12:00:00Z').getTime()
    expect(formatAgo('2026-10-15T11:57:00Z', now)).toBe('3분 전')
    expect(formatAgo('2026-10-15T09:00:00Z', now)).toBe('3시간 전')
  })

  it('응답시간은 1초 이상이면 초 단위다', () => {
    expect(formatMs(182.4)).toBe('182ms')
    expect(formatMs(1534)).toBe('1.53s')
    expect(formatMs(null)).toBe('—')
  })

  it('비율을 퍼센트로 바꾼다', () => {
    expect(formatPercent(0.0083)).toBe('0.8%')
    expect(formatPercent(undefined)).toBe('—')
  })
})
