import type { TimeRange } from '../../../shared/filters/range'

// APM 데이터는 최대 7일까지 본다. 더 긴 기간을 골랐을 때 줄여서 보여준다는 사실을 알린다.
export function ApmRangeNote({ range }: { range: TimeRange }) {
  if (!range.apm.clamped) return null
  return <p className="text-xs text-muted">APM 데이터는 최대 7일까지 조회한다 — 최근 7일을 보여준다.</p>
}
