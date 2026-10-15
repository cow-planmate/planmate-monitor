import type { Span, StatsInterval } from '../../../shared/filters/range'
import { MetricsChart, type MetricLine } from './MetricsChart'

const LINES: MetricLine[] = [
  { metric: 'signups', name: '가입', type: 'bar' },
  { metric: 'plans', name: '일정 생성', type: 'line' },
]

// 개요 화면의 성장 추이.
export function GrowthChart({ span }: { span: Span<StatsInterval> }) {
  return (
    <MetricsChart
      title="가입 · 일정 생성 추이"
      subtitle={`최근 ${span.label} · 가입은 현재 남아 있는 회원 기준(탈퇴 시 기록이 지워진다)`}
      lines={LINES}
      span={span}
    />
  )
}
