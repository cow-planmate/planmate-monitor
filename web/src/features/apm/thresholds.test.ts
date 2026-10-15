import { describe, expect, it } from 'vitest'
import { errorRateTone, latencyTone } from './thresholds'

describe('errorRateTone', () => {
  it('요청이 적으면 비율이 높아도 경고하지 않는다', () => {
    expect(errorRateTone(0.5, 2)).toBe('neutral')
  })
  it('기준에 따라 색이 바뀐다', () => {
    expect(errorRateTone(0.004, 1000)).toBe('ok')
    expect(errorRateTone(0.02, 1000)).toBe('warn')
    expect(errorRateTone(0.08, 1000)).toBe('danger')
  })
})

describe('latencyTone', () => {
  it('p95 기준', () => {
    expect(latencyTone(null)).toBe('neutral')
    expect(latencyTone(120)).toBe('ok')
    expect(latencyTone(700)).toBe('warn')
    expect(latencyTone(2000)).toBe('danger')
  })
})
