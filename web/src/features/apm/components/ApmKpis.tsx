import { KpiCard } from '../../../shared/ui/KpiCard'
import { formatMs, formatPercent, formatRate } from '../../../shared/lib/format'
import { NOW_WINDOW_MINUTES, useApmSummary } from '../queries'
import { errorRateTone, latencyTone } from '../thresholds'

// 성능 KPI 는 항상 최근 15분 기준이다(상단 기간 선택은 추이 차트에만 적용된다).
export function ApmKpis() {
  const { data, isPending, isError } = useApmSummary()
  const o = data?.overall
  const services = data?.services ?? []
  const healthy = services.filter((s) => errorRateTone(s.errorRate, s.requests) !== 'danger').length
  const sub = `최근 ${NOW_WINDOW_MINUTES}분`

  return (
    <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
      <KpiCard
        label="응답시간 p95"
        value={formatMs(o?.p95Ms)}
        sub={sub}
        tone={latencyTone(o?.p95Ms)}
        loading={isPending}
        unavailable={isError}
        to="/performance"
      />
      <KpiCard
        label="에러율"
        value={formatPercent(o?.errorRate)}
        sub={o ? `${sub} · 실패 ${o.failures}건` : sub}
        tone={errorRateTone(o?.errorRate, o?.requests ?? 0)}
        loading={isPending}
        unavailable={isError}
        to="/errors"
      />
      <KpiCard
        label="처리량"
        value={formatRate(o?.rpm)}
        sub={o ? `${sub} · ${o.requests.toLocaleString('ko-KR')}건` : sub}
        loading={isPending}
        unavailable={isError}
        to="/performance"
      />
      <KpiCard
        label="정상 서비스"
        value={`${healthy} / ${services.length}`}
        sub="에러율 5% 미만"
        tone={services.length === 0 ? 'neutral' : healthy === services.length ? 'ok' : 'danger'}
        loading={isPending}
        unavailable={isError}
        to="/performance"
      />
    </div>
  )
}
